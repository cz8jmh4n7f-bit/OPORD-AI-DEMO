package api

import (
	"net/http"
	"strconv"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/orchestrator"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

type finopsSpendDTO struct {
	Name       string  `json:"name"`
	MonthlyUSD float64 `json:"monthlyUsd"`
}

type finopsPhaseDTO struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Actions     []string `json:"actions"`
}

type finopsAllocationCoverageDTO struct {
	Resources        int     `json:"resources"`
	OwnerTagged      int     `json:"ownerTagged"`
	ProjectTagged    int     `json:"projectTagged"`
	CostCenterTagged int     `json:"costCenterTagged"`
	TTLProtected     int     `json:"ttlProtected"`
	CoveragePct      float64 `json:"coveragePct"`
}

type finopsBudgetDTO struct {
	Scope        string  `json:"scope"`
	Name         string  `json:"name"`
	LimitUSD     float64 `json:"limitUsd"`
	ActualUSD    float64 `json:"actualUsd"`
	RemainingUSD float64 `json:"remainingUsd"`
	UsagePct     float64 `json:"usagePct"`
	Status       string  `json:"status"`
}

type finopsGuardrailDTO struct {
	Severity string `json:"severity"`
	Resource string `json:"resource"`
	Kind     string `json:"kind"`
	Message  string `json:"message"`
	Action   string `json:"action"`
}

type finopsSavingsDTO struct {
	Resource   string  `json:"resource"`
	Kind       string  `json:"kind"`
	Provider   string  `json:"provider"`
	MonthlyUSD float64 `json:"monthlyUsd"`
	SavingsUSD float64 `json:"savingsUsd"`
	Confidence string  `json:"confidence"`
	Action     string  `json:"action"`
}

type finopsUnitMetricDTO struct {
	Name       string  `json:"name"`
	Resources  int     `json:"resources"`
	MonthlyUSD float64 `json:"monthlyUsd"`
	AvgUSD     float64 `json:"avgUsd"`
}

type focusGuideDTO struct {
	Cloud          string `json:"cloud"`
	ProviderType   string `json:"providerType"`
	FocusVersion   string `json:"focusVersion"`
	Status         string `json:"status"`
	Export         string `json:"export"`
	Analytics      string `json:"analytics"`
	OPORDReadiness string `json:"opordReadiness"`
	URL            string `json:"url"`
}

type costBucketDTO struct {
	Key  string  `json:"key"`
	Name string  `json:"name,omitempty"`
	USD  float64 `json:"usd"`
}

type costPointDTO struct {
	Date string  `json:"date"`
	USD  float64 `json:"usd"`
}

type costAnomalyDTO struct {
	Date        string  `json:"date"`
	USD         float64 `json:"usd"`
	BaselineUSD float64 `json:"baselineUsd"`
	Factor      float64 `json:"factor"`
}

type costAccountDTO struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type costActualsDTO struct {
	Currency     string           `json:"currency"`
	WindowDays   int              `json:"windowDays"`
	TotalUSD     float64          `json:"totalUsd"`
	MTDUSD       float64          `json:"mtdUsd"`
	ForecastUSD  float64          `json:"forecastUsd"`
	DailyRunRate float64          `json:"dailyRunRate"`
	ByCloud      []costBucketDTO  `json:"byCloud,omitempty"`
	ByAccount    []costBucketDTO  `json:"byAccount"`
	ByService    []costBucketDTO  `json:"byService"`
	Daily        []costPointDTO   `json:"daily"`
	Anomalies    []costAnomalyDTO `json:"anomalies"`
	Accounts     []costAccountDTO `json:"accounts"`
}

func actualsToDTO(a *providers.CostActuals) *costActualsDTO {
	if a == nil {
		return nil
	}
	bucket := func(in []providers.CostBucket) []costBucketDTO {
		out := make([]costBucketDTO, 0, len(in))
		for _, b := range in {
			out = append(out, costBucketDTO{Key: b.Key, Name: b.Name, USD: b.USD})
		}
		return out
	}
	daily := make([]costPointDTO, 0, len(a.Daily))
	for _, p := range a.Daily {
		daily = append(daily, costPointDTO{Date: p.Date, USD: p.USD})
	}
	anomalies := make([]costAnomalyDTO, 0, len(a.Anomalies))
	for _, an := range a.Anomalies {
		anomalies = append(anomalies, costAnomalyDTO{Date: an.Date, USD: an.USD, BaselineUSD: an.BaselineUSD, Factor: an.Factor})
	}
	accounts := make([]costAccountDTO, 0, len(a.Accounts))
	for _, ac := range a.Accounts {
		accounts = append(accounts, costAccountDTO{ID: ac.ID, Name: ac.Name})
	}
	return &costActualsDTO{
		Currency:     a.Currency,
		WindowDays:   a.WindowDays,
		TotalUSD:     a.TotalUSD,
		MTDUSD:       a.MTDUSD,
		ForecastUSD:  a.ForecastUSD,
		DailyRunRate: a.DailyRunRate,
		ByCloud:      bucket(a.ByCloud),
		ByAccount:    bucket(a.ByAccount),
		ByService:    bucket(a.ByService),
		Daily:        daily,
		Anomalies:    anomalies,
		Accounts:     accounts,
	}
}

type finopsCloudDTO struct {
	Name      string  `json:"name"`
	Type      string  `json:"type"`
	USD       float64 `json:"usd"`
	Available bool    `json:"available"`
	Error     string  `json:"error,omitempty"`
}

func cloudsToDTO(in []orchestrator.FinOpsCloud) []finopsCloudDTO {
	out := make([]finopsCloudDTO, 0, len(in))
	for _, c := range in {
		out = append(out, finopsCloudDTO{Name: c.Name, Type: c.Type, USD: c.USD, Available: c.Available, Error: c.Error})
	}
	return out
}

type finopsReportDTO struct {
	TotalUSD             float64                     `json:"totalUsd"`
	ProjectedMonthlyUSD  float64                     `json:"projectedMonthlyUsd"`
	DailyRunRateUSD      float64                     `json:"dailyRunRateUsd"`
	ActiveResources      int                         `json:"activeResources"`
	ProviderSpend        []finopsSpendDTO            `json:"providerSpend"`
	EnvironmentSpend     []finopsSpendDTO            `json:"environmentSpend"`
	KindSpend            []finopsSpendDTO            `json:"kindSpend"`
	AllocationCoverage   finopsAllocationCoverageDTO `json:"allocationCoverage"`
	Budgets              []finopsBudgetDTO           `json:"budgets"`
	Guardrails           []finopsGuardrailDTO        `json:"guardrails"`
	SavingsOpportunities []finopsSavingsDTO          `json:"savingsOpportunities"`
	UnitMetrics          []finopsUnitMetricDTO       `json:"unitMetrics"`
	FocusGuides          []focusGuideDTO             `json:"focusGuides"`
	Phases               []finopsPhaseDTO            `json:"phases"`
	Recommendations      []string                    `json:"recommendations"`
	Actuals              *costActualsDTO             `json:"actuals,omitempty"`
	ActualsSource        string                      `json:"actualsSource,omitempty"`
	ActualsError         string                      `json:"actualsError,omitempty"`
	Clouds               []finopsCloudDTO            `json:"clouds,omitempty"`
	Provider             string                      `json:"provider,omitempty"`
	Account              string                      `json:"account,omitempty"`
	WindowDays           int                         `json:"windowDays,omitempty"`
}

func (s *Server) getFinOps(w http.ResponseWriter, r *http.Request) {
	provider := r.URL.Query().Get("provider")
	account := r.URL.Query().Get("account")
	days := 0
	if d := r.URL.Query().Get("days"); d != "" {
		days, _ = strconv.Atoi(d)
	}
	rep, err := s.svc.FinOpsReport(r.Context(), orchestrator.FinOpsOptions{Provider: provider, Account: account, Days: days})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	providers := make([]finopsSpendDTO, 0, len(rep.ProviderSpend))
	for _, item := range rep.ProviderSpend {
		providers = append(providers, finopsSpendDTO{Name: item.Name, MonthlyUSD: item.MonthlyUSD})
	}
	envs := make([]finopsSpendDTO, 0, len(rep.EnvironmentSpend))
	for _, item := range rep.EnvironmentSpend {
		envs = append(envs, finopsSpendDTO{Name: item.Name, MonthlyUSD: item.MonthlyUSD})
	}
	kinds := make([]finopsSpendDTO, 0, len(rep.KindSpend))
	for _, item := range rep.KindSpend {
		kinds = append(kinds, finopsSpendDTO{Name: item.Name, MonthlyUSD: item.MonthlyUSD})
	}
	budgets := make([]finopsBudgetDTO, 0, len(rep.Budgets))
	for _, item := range rep.Budgets {
		budgets = append(budgets, finopsBudgetDTO{
			Scope:        item.Scope,
			Name:         item.Name,
			LimitUSD:     item.LimitUSD,
			ActualUSD:    item.ActualUSD,
			RemainingUSD: item.RemainingUSD,
			UsagePct:     item.UsagePct,
			Status:       item.Status,
		})
	}
	guardrails := make([]finopsGuardrailDTO, 0, len(rep.Guardrails))
	for _, item := range rep.Guardrails {
		guardrails = append(guardrails, finopsGuardrailDTO{
			Severity: item.Severity,
			Resource: item.Resource,
			Kind:     item.Kind,
			Message:  item.Message,
			Action:   item.Action,
		})
	}
	savings := make([]finopsSavingsDTO, 0, len(rep.SavingsOpportunities))
	for _, item := range rep.SavingsOpportunities {
		savings = append(savings, finopsSavingsDTO{
			Resource:   item.Resource,
			Kind:       item.Kind,
			Provider:   item.Provider,
			MonthlyUSD: item.MonthlyUSD,
			SavingsUSD: item.SavingsUSD,
			Confidence: item.Confidence,
			Action:     item.Action,
		})
	}
	units := make([]finopsUnitMetricDTO, 0, len(rep.UnitMetrics))
	for _, item := range rep.UnitMetrics {
		units = append(units, finopsUnitMetricDTO{
			Name:       item.Name,
			Resources:  item.Resources,
			MonthlyUSD: item.MonthlyUSD,
			AvgUSD:     item.AvgUSD,
		})
	}
	guides := make([]focusGuideDTO, 0, len(rep.FocusGuides))
	for _, guide := range rep.FocusGuides {
		guides = append(guides, focusGuideDTO{
			Cloud:          guide.Cloud,
			ProviderType:   guide.ProviderType,
			FocusVersion:   guide.FocusVersion,
			Status:         guide.Status,
			Export:         guide.Export,
			Analytics:      guide.Analytics,
			OPORDReadiness: guide.OPORDReadiness,
			URL:            guide.URL,
		})
	}
	phases := make([]finopsPhaseDTO, 0, len(rep.Phases))
	for _, phase := range rep.Phases {
		phases = append(phases, finopsPhaseDTO{
			Name:        phase.Name,
			Description: phase.Description,
			Actions:     phase.Actions,
		})
	}

	writeJSON(w, http.StatusOK, finopsReportDTO{
		TotalUSD:            rep.TotalUSD,
		ProjectedMonthlyUSD: rep.ProjectedMonthlyUSD,
		DailyRunRateUSD:     rep.DailyRunRateUSD,
		ActiveResources:     rep.ActiveResources,
		ProviderSpend:       providers,
		EnvironmentSpend:    envs,
		KindSpend:           kinds,
		AllocationCoverage: finopsAllocationCoverageDTO{
			Resources:        rep.AllocationCoverage.Resources,
			OwnerTagged:      rep.AllocationCoverage.OwnerTagged,
			ProjectTagged:    rep.AllocationCoverage.ProjectTagged,
			CostCenterTagged: rep.AllocationCoverage.CostCenterTagged,
			TTLProtected:     rep.AllocationCoverage.TTLProtected,
			CoveragePct:      rep.AllocationCoverage.CoveragePct,
		},
		Budgets:              budgets,
		Guardrails:           guardrails,
		SavingsOpportunities: savings,
		UnitMetrics:          units,
		FocusGuides:          guides,
		Phases:               phases,
		Recommendations:      rep.Recommendations,
		Actuals:              actualsToDTO(rep.Actuals),
		ActualsSource:        rep.ActualsSource,
		ActualsError:         rep.ActualsError,
		Clouds:               cloudsToDTO(rep.Clouds),
		Provider:             rep.Provider,
		Account:              rep.Account,
		WindowDays:           rep.WindowDays,
	})
}
