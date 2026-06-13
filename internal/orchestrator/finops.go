package orchestrator

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

// --- Cost-actuals cache ---
// Cloud cost APIs rate-limit aggressively - Azure Cost Management returns HTTP 429
// ("Too many requests") after only a few calls, and AWS Cost Explorer bills per
// request. FinOps actuals only change ~daily, so we cache each provider's result
// for a few minutes and serve a STALE cached value on error (429/timeout), so a
// throttled cloud keeps its populated tile instead of going unavailable, and the
// page stops hammering the upstream on every load / tile switch.
const costCacheTTL = 12 * time.Minute

type costCacheEntry struct {
	actuals *providers.CostActuals
	at      time.Time
}

var (
	costCacheMu sync.Mutex
	costCache   = map[string]costCacheEntry{}
)

// cachedReportCost wraps a provider's ReportCost with a TTL cache + stale-on-error
// fallback, keyed per provider+account+window.
func cachedReportCost(ctx context.Context, cr providers.CostReporter, key string, q providers.CostQuery, now time.Time) (*providers.CostActuals, error) {
	costCacheMu.Lock()
	ent, ok := costCache[key]
	costCacheMu.Unlock()
	if ok && now.Sub(ent.at) < costCacheTTL {
		return ent.actuals, nil // fresh hit - no upstream call
	}
	a, err := cr.ReportCost(ctx, q)
	if err != nil {
		if ok && ent.actuals != nil {
			return ent.actuals, nil // stale-on-error: keep serving the last good result
		}
		return nil, err
	}
	if a != nil {
		costCacheMu.Lock()
		costCache[key] = costCacheEntry{actuals: a, at: now}
		costCacheMu.Unlock()
	}
	return a, nil
}

type FinOpsSpendBreakdown struct {
	Name       string
	MonthlyUSD float64
}

type FinOpsPhase struct {
	Name        string
	Description string
	Actions     []string
}

type FinOpsAllocationCoverage struct {
	Resources        int
	OwnerTagged      int
	ProjectTagged    int
	CostCenterTagged int
	TTLProtected     int
	CoveragePct      float64
}

type FinOpsBudget struct {
	Scope        string
	Name         string
	LimitUSD     float64
	ActualUSD    float64
	RemainingUSD float64
	UsagePct     float64
	Status       string
}

type FinOpsGuardrail struct {
	Severity string
	Resource string
	Kind     string
	Message  string
	Action   string
}

type FinOpsSavingsOpportunity struct {
	Resource   string
	Kind       string
	Provider   string
	MonthlyUSD float64
	SavingsUSD float64
	Confidence string
	Action     string
}

type FinOpsUnitMetric struct {
	Name       string
	Resources  int
	MonthlyUSD float64
	AvgUSD     float64
}

type FocusGuide struct {
	Cloud          string
	ProviderType   string
	FocusVersion   string
	Status         string
	Export         string
	Analytics      string
	OPORDReadiness string
	URL            string
}

type FinOpsReport struct {
	TotalUSD             float64
	ProjectedMonthlyUSD  float64
	DailyRunRateUSD      float64
	ActiveResources      int
	ProviderSpend        []FinOpsSpendBreakdown
	EnvironmentSpend     []FinOpsSpendBreakdown
	KindSpend            []FinOpsSpendBreakdown
	AllocationCoverage   FinOpsAllocationCoverage
	Budgets              []FinOpsBudget
	Guardrails           []FinOpsGuardrail
	SavingsOpportunities []FinOpsSavingsOpportunity
	UnitMetrics          []FinOpsUnitMetric
	FocusGuides          []FocusGuide
	Phases               []FinOpsPhase
	Recommendations      []string

	// Actuals is REAL billed spend from the cloud cost API (AWS Cost Explorer),
	// nil when no cost-reporting provider exists or the credentials lack ce:
	// permissions. When present, ProjectedMonthlyUSD/DailyRunRateUSD reflect the
	// real run-rate (not the estimate). The web features it and reconciles it
	// against the OPORD estimate (TotalUSD).
	Actuals       *providers.CostActuals `json:"actuals,omitempty"`
	ActualsSource string                 `json:"actualsSource,omitempty"` // e.g. "aws_cost_explorer"
	ActualsError  string                 `json:"actualsError,omitempty"`  // why actuals are unavailable (drives the "connect Cost Explorer" banner)
	Clouds        []FinOpsCloud          `json:"clouds,omitempty"`        // ALL cost-capable clouds (available + unavailable) - drives the stable cloud tiles
	Provider      string                 `json:"provider,omitempty"`      // the cloud (provider name) tile selected ("" = all clouds)
	Account       string                 `json:"account,omitempty"`       // the linked-account filter applied ("" = all)
	WindowDays    int                    `json:"windowDays,omitempty"`    // trailing window for actuals
}

// FinOpsCloud is one cost-capable cloud's status for the tile row. Available=false
// means its cost query failed this load (e.g. Azure Cost Management throttling or
// dynamic-cred lag) - the tile still renders (greyed) so the row stays stable
// instead of the cloud vanishing.
type FinOpsCloud struct {
	Name      string  `json:"name"`            // provider name (OPORD-Azure)
	Type      string  `json:"type"`            // aws / azure / gcp
	USD       float64 `json:"usd"`             // window total (0 when unavailable)
	Available bool    `json:"available"`
	Error     string  `json:"error,omitempty"` // short reason when unavailable
}

// FinOpsOptions narrows the FinOps view: a cloud (provider name) tile, an account
// within that cloud, and a trailing window. Zero values mean "all clouds, all
// accounts, 30 days".
type FinOpsOptions struct {
	Provider string // provider name (cloud tile); "" = all clouds
	Account  string // linked account within the selected cloud; "" = all
	Days     int
}

// costActuals finds the first AWS provider that implements CostReporter and asks
// it for real billed spend. It returns (nil, "", nil) when no such provider
// exists, and (nil, source, err) when the cost API is reachable but rejects us
// (e.g. ce: not granted) - the caller surfaces that as a connect banner.
type costCloud struct {
	name  string
	typ   string
	cr    providers.CostReporter
	creds map[string]string
	cfg   map[string]any
	full  *providers.CostActuals // unfiltered (account=""), for the cloud tiles + scope
}

func (s *Service) costActuals(ctx context.Context, provider, account string, days int) (*providers.CostActuals, []FinOpsCloud, string, error) {
	provs, err := s.q.ListProviders(ctx)
	if err != nil {
		return nil, nil, "", err
	}
	if days <= 0 {
		days = 30
	}
	now := time.Now()

	// Query every cost-reporting cloud UNFILTERED - this drives the cloud tiles
	// (each cloud's full total) and is the data we scope from.
	var clouds []costCloud
	var byCloud []providers.CostBucket
	var cloudList []FinOpsCloud // every cost-capable cloud (available + unavailable), for the tiles
	var firstErr error
	for _, p := range provs {
		prov, err := s.registry.Get(models.ProviderType(p.Type))
		if err != nil {
			continue
		}
		cr, ok := prov.(providers.CostReporter)
		if !ok {
			continue // no cost API (vSphere/Proxmox)
		}
		creds, _ := s.creds.Resolve(ctx, p)
		cfg := s.providerCfg(ctx, p)
		full, err := cachedReportCost(ctx, cr, fmt.Sprintf("%s||%d", p.Name, days), providers.CostQuery{Days: days, Account: "", Credentials: creds, Config: cfg}, now)
		if err != nil {
			// Cost-capable but unavailable this load - keep a greyed tile so the row
			// stays stable instead of the cloud vanishing (e.g. transient Azure).
			cloudList = append(cloudList, FinOpsCloud{Name: p.Name, Type: string(p.Type), Available: false, Error: shortErr(err)})
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %w", p.Name, err)
			}
			continue
		}
		if full == nil {
			continue
		}
		clouds = append(clouds, costCloud{name: p.Name, typ: string(p.Type), cr: cr, creds: creds, cfg: cfg, full: full})
		byCloud = append(byCloud, providers.CostBucket{Key: string(p.Type), Name: p.Name, USD: full.TotalUSD})
		cloudList = append(cloudList, FinOpsCloud{Name: p.Name, Type: string(p.Type), USD: full.TotalUSD, Available: true})
	}
	sortClouds(cloudList)
	if len(clouds) == 0 {
		if firstErr != nil {
			return nil, cloudList, "cloud_cost_api", firstErr // nothing succeeded - drive the connect banner
		}
		return nil, cloudList, "", nil
	}

	// Main report: all clouds merged, or the selected cloud (with the account
	// filter re-applied within it). The account dropdown keeps the cloud's FULL
	// account list so you can still switch accounts.
	var main *providers.CostActuals
	var sources []string
	if provider == "" {
		for _, c := range clouds {
			main = mergeActuals(main, c.full)
			sources = append(sources, c.typ)
		}
	} else {
		for _, c := range clouds {
			if c.name != provider {
				continue
			}
			sources = []string{c.typ}
			if account != "" {
				if filtered, err := cachedReportCost(ctx, c.cr, fmt.Sprintf("%s|%s|%d", c.name, account, days), providers.CostQuery{Days: days, Account: account, Credentials: c.creds, Config: c.cfg}, now); err == nil && filtered != nil {
					main = filtered
					main.Accounts = c.full.Accounts // keep the cloud's full account list for the dropdown
				}
			}
			if main == nil {
				main = mergeActuals(nil, c.full) // clone the cloud's unfiltered report
			}
			break
		}
		if main == nil {
			main = &providers.CostActuals{Currency: "USD", WindowDays: days} // selected cloud reported nothing
		}
	}

	sortBuckets(byCloud)
	main.ByCloud = byCloud
	main.WindowDays = days
	sortBuckets(main.ByAccount)
	sortBuckets(main.ByService)
	sort.Slice(main.Daily, func(i, j int) bool { return main.Daily[i].Date < main.Daily[j].Date })
	sort.Slice(main.Accounts, func(i, j int) bool { return main.Accounts[i].ID < main.Accounts[j].ID })
	return main, cloudList, strings.Join(sources, ","), nil
}

// shortErr trims a provider error for a tile tooltip.
func shortErr(e error) string {
	s := e.Error()
	if len(s) > 140 {
		return s[:140] + "…"
	}
	return s
}

// sortClouds orders the tile row: available clouds first (by spend desc), then
// unavailable ones (by name), so the row layout stays stable.
func sortClouds(c []FinOpsCloud) {
	sort.Slice(c, func(i, j int) bool {
		if c[i].Available != c[j].Available {
			return c[i].Available
		}
		if c[i].Available && c[i].USD != c[j].USD {
			return c[i].USD > c[j].USD
		}
		return c[i].Name < c[j].Name
	})
}

// mergeActuals folds provider b's actuals into a. a==nil starts a fresh copy so the
// provider's own returned slices are never mutated by later appends.
func mergeActuals(a, b *providers.CostActuals) *providers.CostActuals {
	if a == nil {
		cp := *b
		cp.ByAccount = append([]providers.CostBucket(nil), b.ByAccount...)
		cp.ByService = append([]providers.CostBucket(nil), b.ByService...)
		cp.Daily = append([]providers.CostPoint(nil), b.Daily...)
		cp.Anomalies = append([]providers.CostAnomaly(nil), b.Anomalies...)
		cp.Accounts = append([]providers.CostAccountRef(nil), b.Accounts...)
		return &cp
	}
	a.TotalUSD = round2(a.TotalUSD + b.TotalUSD)
	a.MTDUSD = round2(a.MTDUSD + b.MTDUSD)
	a.ForecastUSD = round2(a.ForecastUSD + b.ForecastUSD)
	a.DailyRunRate = round2(a.DailyRunRate + b.DailyRunRate)
	a.ByAccount = append(a.ByAccount, b.ByAccount...)
	a.ByService = append(a.ByService, b.ByService...)
	a.Anomalies = append(a.Anomalies, b.Anomalies...)
	a.Accounts = append(a.Accounts, b.Accounts...)
	a.Daily = mergeDaily(a.Daily, b.Daily)
	if a.Currency == "" {
		a.Currency = b.Currency
	}
	return a
}

// mergeDaily sums two daily spend series by date.
func mergeDaily(a, b []providers.CostPoint) []providers.CostPoint {
	byDate := map[string]float64{}
	for _, p := range a {
		byDate[p.Date] += p.USD
	}
	for _, p := range b {
		byDate[p.Date] += p.USD
	}
	out := make([]providers.CostPoint, 0, len(byDate))
	for d, v := range byDate {
		out = append(out, providers.CostPoint{Date: d, USD: round2(v)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out
}

func sortBuckets(b []providers.CostBucket) {
	sort.Slice(b, func(i, j int) bool {
		if b[i].USD == b[j].USD {
			return b[i].Key < b[j].Key
		}
		return b[i].USD > b[j].USD
	})
}

func addSpend(m map[string]float64, key string, monthly float64) {
	if key == "" {
		key = "unknown"
	}
	m[key] += monthly
}

func spendBreakdown(m map[string]float64) []FinOpsSpendBreakdown {
	out := make([]FinOpsSpendBreakdown, 0, len(m))
	for name, monthly := range m {
		out = append(out, FinOpsSpendBreakdown{Name: name, MonthlyUSD: monthly})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MonthlyUSD == out[j].MonthlyUSD {
			return out[i].Name < out[j].Name
		}
		return out[i].MonthlyUSD > out[j].MonthlyUSD
	})
	return out
}

func budgetLimit(env string) float64 {
	switch env {
	case "sandbox", "lab":
		return 100
	case "dev":
		return 250
	case "stage", "staging", "test", "qa":
		return 750
	case "prod", "production":
		return 2500
	default:
		return 500
	}
}

func budgetStatus(usage float64) string {
	switch {
	case usage >= 1:
		return "over"
	case usage >= 0.8:
		return "risk"
	default:
		return "ok"
	}
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func allocationCoverage(lines []CostLine) FinOpsAllocationCoverage {
	c := FinOpsAllocationCoverage{Resources: len(lines)}
	for _, line := range lines {
		if line.Owner != "" {
			c.OwnerTagged++
		}
		if line.Project != "" {
			c.ProjectTagged++
		}
		if line.CostCenter != "" {
			c.CostCenterTagged++
		}
		if line.TTLHours > 0 {
			c.TTLProtected++
		}
	}
	if c.Resources > 0 {
		c.CoveragePct = round2((float64(c.OwnerTagged+c.ProjectTagged+c.CostCenterTagged) / float64(c.Resources*3)) * 100)
	}
	return c
}

func budgets(envSpend map[string]float64) []FinOpsBudget {
	out := make([]FinOpsBudget, 0, len(envSpend))
	for env, actual := range envSpend {
		limit := budgetLimit(env)
		usage := 0.0
		if limit > 0 {
			usage = actual / limit
		}
		out = append(out, FinOpsBudget{
			Scope:        "environment",
			Name:         env,
			LimitUSD:     limit,
			ActualUSD:    round2(actual),
			RemainingUSD: round2(limit - actual),
			UsagePct:     round2(usage * 100),
			Status:       budgetStatus(usage),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UsagePct > out[j].UsagePct
	})
	return out
}

func guardrails(lines []CostLine, envSpend map[string]float64) []FinOpsGuardrail {
	out := []FinOpsGuardrail{}
	for env, actual := range envSpend {
		limit := budgetLimit(env)
		if limit > 0 && actual/limit >= 0.8 {
			severity := "warn"
			if actual > limit {
				severity = "blocker"
			}
			out = append(out, FinOpsGuardrail{
				Severity: severity,
				Resource: env,
				Kind:     "budget",
				Message:  "Environment is close to or above its default monthly budget.",
				Action:   "Require approval for new expensive resources or raise the environment budget.",
			})
		}
	}
	for _, line := range lines {
		if line.Owner == "" || line.Project == "" || line.CostCenter == "" {
			out = append(out, FinOpsGuardrail{
				Severity: "warn",
				Resource: line.Name,
				Kind:     line.Kind,
				Message:  "Missing owner/project/cost-center allocation metadata.",
				Action:   "Add allocation tags before production use.",
			})
		}
		if line.TTLHours == 0 && (line.Environment == "dev" || line.Environment == "sandbox" || line.Environment == "lab") {
			out = append(out, FinOpsGuardrail{
				Severity: "info",
				Resource: line.Name,
				Kind:     line.Kind,
				Message:  "Non-production resource has no TTL.",
				Action:   "Set ttl_hours for temporary workloads.",
			})
		}
		for _, risk := range line.RiskFlags {
			out = append(out, FinOpsGuardrail{
				Severity: "warn",
				Resource: line.Name,
				Kind:     line.Kind,
				Message:  "Risk flag detected: " + risk + ".",
				Action:   "Review provider safety profile and public exposure before approving.",
			})
		}
	}
	if len(out) > 12 {
		return out[:12]
	}
	return out
}

func savings(lines []CostLine) []FinOpsSavingsOpportunity {
	out := []FinOpsSavingsOpportunity{}
	for _, line := range lines {
		switch {
		case line.MonthlyUSD >= 100 && (line.Kind == "vm" || line.Kind == "cluster"):
			out = append(out, FinOpsSavingsOpportunity{
				Resource:   line.Name,
				Kind:       line.Kind,
				Provider:   line.Provider,
				MonthlyUSD: round2(line.MonthlyUSD),
				SavingsUSD: round2(line.MonthlyUSD * 0.25),
				Confidence: "medium",
				Action:     "Check utilization and rightsize instance/node shape.",
			})
		case line.Kind == "database" && line.MonthlyUSD >= 50:
			out = append(out, FinOpsSavingsOpportunity{
				Resource:   line.Name,
				Kind:       line.Kind,
				Provider:   line.Provider,
				MonthlyUSD: round2(line.MonthlyUSD),
				SavingsUSD: round2(line.MonthlyUSD * 0.20),
				Confidence: "medium",
				Action:     "Review storage growth, Multi-AZ, public access, and smallest acceptable SKU.",
			})
		case line.TTLHours == 0 && (line.Environment == "dev" || line.Environment == "sandbox" || line.Environment == "lab") && line.MonthlyUSD > 0:
			out = append(out, FinOpsSavingsOpportunity{
				Resource:   line.Name,
				Kind:       line.Kind,
				Provider:   line.Provider,
				MonthlyUSD: round2(line.MonthlyUSD),
				SavingsUSD: round2(line.MonthlyUSD * 0.50),
				Confidence: "high",
				Action:     "Add TTL or scheduled shutdown for non-production workloads.",
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SavingsUSD > out[j].SavingsUSD })
	if len(out) > 8 {
		return out[:8]
	}
	return out
}

func unitMetrics(envSpend map[string]float64, envResources map[string]int) []FinOpsUnitMetric {
	out := make([]FinOpsUnitMetric, 0, len(envSpend))
	for env, monthly := range envSpend {
		resources := envResources[env]
		avg := 0.0
		if resources > 0 {
			avg = monthly / float64(resources)
		}
		out = append(out, FinOpsUnitMetric{Name: env, Resources: resources, MonthlyUSD: round2(monthly), AvgUSD: round2(avg)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MonthlyUSD > out[j].MonthlyUSD })
	return out
}

// FinOpsReport turns OPORD's resource inventory and estimate model into a
// FinOps operating view. It is intentionally source-aware but billing-system
// agnostic: real FOCUS ingestion can replace the static guide/readiness layer
// without changing the web contract.
func (s *Service) FinOpsReport(ctx context.Context, opts FinOpsOptions) (*FinOpsReport, error) {
	cost, err := s.CostReport(ctx)
	if err != nil {
		return nil, err
	}

	providerSpend := map[string]float64{}
	envSpend := map[string]float64{}
	kindSpend := map[string]float64{}
	envResources := map[string]int{}
	for _, line := range cost.Lines {
		addSpend(providerSpend, line.Provider, line.MonthlyUSD)
		addSpend(envSpend, line.Environment, line.MonthlyUSD)
		addSpend(kindSpend, line.Kind, line.MonthlyUSD)
		envResources[line.Environment]++
	}

	report := &FinOpsReport{
		TotalUSD:             round2(cost.TotalUSD),
		ProjectedMonthlyUSD:  round2(cost.TotalUSD),
		DailyRunRateUSD:      round2(cost.TotalUSD / 30.0),
		ActiveResources:      len(cost.Lines),
		ProviderSpend:        spendBreakdown(providerSpend),
		EnvironmentSpend:     spendBreakdown(envSpend),
		KindSpend:            spendBreakdown(kindSpend),
		AllocationCoverage:   allocationCoverage(cost.Lines),
		Budgets:              budgets(envSpend),
		Guardrails:           guardrails(cost.Lines, envSpend),
		SavingsOpportunities: savings(cost.Lines),
		UnitMetrics:          unitMetrics(envSpend, envResources),
		FocusGuides: []FocusGuide{
			{
				Cloud:          "AWS",
				ProviderType:   "aws",
				FocusVersion:   "v1.2",
				Status:         "setup guide",
				Export:         "Billing and Cost Management FOCUS export or CloudFormation automation",
				Analytics:      "Athena table, CID CLI, and FOCUS SQL use cases",
				OPORDReadiness: "Connect exported S3/Athena dataset, then map account, service, region, tags, and OPORD resource metadata.",
				URL:            "https://focus.finops.org/get-started/aws/",
			},
			{
				Cloud:          "Microsoft Azure",
				ProviderType:   "azure",
				FocusVersion:   "v1.2",
				Status:         "setup guide",
				Export:         "Cost Management export using the FOCUS template",
				Analytics:      "Power BI reports or Microsoft Fabric ingestion",
				OPORDReadiness: "Connect exported storage/Fabric dataset, then align subscription, resource group, tags, and OPORD environments.",
				URL:            "https://focus.finops.org/get-started/microsoft/",
			},
			{
				Cloud:          "Google Cloud",
				ProviderType:   "gcp",
				FocusVersion:   "v1.0",
				Status:         "setup guide",
				Export:         "Detailed Billing Export and Price Export to BigQuery",
				Analytics:      "FOCUS BigQuery view or Looker template",
				OPORDReadiness: "Future provider: connect BigQuery billing data and map projects, labels, SKUs, and environments.",
				URL:            "https://focus.finops.org/get-started/google-cloud/",
			},
		},
		Phases: []FinOpsPhase{
			{
				Name:        "Inform",
				Description: "Make cost and usage visible by owner, provider, environment, service, and resource.",
				Actions: []string{
					"Keep OPORD resource metadata aligned to owners, environments, and projects.",
					"Use FOCUS exports as the normalized billing source across clouds.",
					"Compare OPORD estimates with billed FOCUS cost once ingestion is connected.",
				},
			},
			{
				Name:        "Optimize",
				Description: "Turn visibility into concrete resource and rate improvements.",
				Actions: []string{
					"Find idle or oversized resources from OPORD inventory and billed usage.",
					"Prefer managed services and TTLs for temporary workloads.",
					"Track savings opportunities by resource, team, and environment.",
				},
			},
			{
				Name:        "Operate",
				Description: "Make cost ownership part of day-to-day cloud operations.",
				Actions: []string{
					"Set account/subscription budgets and alerts before provisioning.",
					"Expose cost guardrails in catalog forms and approval workflows.",
					"Review anomalies and budget drift as operational work, not monthly archaeology.",
				},
			},
		},
		Recommendations: []string{
			"Make owner, project, and cost_center mandatory in catalog forms before production.",
			"Block or require approval when environment budget usage exceeds 80%.",
			"Add TTL defaults to sandbox/dev resources and scheduled shutdown for idle compute.",
			"Create a FOCUS data connection per cloud and normalize spend into one queryable dataset.",
			"Compare OPORD estimate-vs-actual and promote high-confidence savings recommendations.",
		},
	}

	days := opts.Days
	if days <= 0 || days > 365 {
		days = 30
	}
	report.Provider = opts.Provider
	report.Account = opts.Account
	report.WindowDays = days
	// Best-effort real spend; the estimate-based report still renders if this
	// fails (e.g. ce: not granted) - ActualsError then drives the connect banner.
	actuals, clouds, source, err := s.costActuals(ctx, opts.Provider, opts.Account, days)
	report.Clouds = clouds
	if err != nil {
		report.ActualsSource = source
		report.ActualsError = err.Error()
	} else if actuals != nil {
		report.Actuals = actuals
		report.ActualsSource = source
		report.ProjectedMonthlyUSD = round2(actuals.ForecastUSD)
		report.DailyRunRateUSD = round2(actuals.DailyRunRate)
	}
	return report, nil
}
