package providers

import (
	"sort"
	"time"
)

// CostRow is one (day, account, service, usd) cell from a cloud cost API, before
// aggregation. Every CostReporter maps its cloud's billing data into these rows and
// calls AggregateActuals, so the by-account / by-service / trend / forecast / anomaly
// logic lives in ONE place and each provider only writes its cloud-specific query.
type CostRow struct {
	Date        string // YYYY-MM-DD
	Account     string // linked account / project / subscription id
	AccountName string // human label (optional)
	Service     string // cloud service name
	USD         float64
}

// AggregateActuals folds raw cost rows into a CostActuals: total, month-to-date, a
// run-rate end-of-month forecast (cloud-native forecasts need weeks of history these
// young accounts lack), a recent daily run-rate, by-account and by-service
// breakdowns, the daily trend, anomalies (a day >= 2x the trailing-7-day baseline,
// above a $1 floor), and the distinct accounts seen (for the filter). `now` is
// injected for testability.
func AggregateActuals(rows []CostRow, days int, now time.Time) *CostActuals {
	if days <= 0 || days > 365 {
		days = 30
	}

	dailyMap := map[string]float64{}
	acctMap := map[string]float64{}
	svcMap := map[string]float64{}
	names := map[string]string{}
	var total float64
	for _, r := range rows {
		dailyMap[r.Date] += r.USD
		if r.Account != "" {
			acctMap[r.Account] += r.USD
			if r.AccountName != "" {
				names[r.Account] = r.AccountName
			}
		}
		if r.Service != "" {
			svcMap[r.Service] += r.USD
		}
		total += r.USD
	}

	daily := make([]CostPoint, 0, len(dailyMap))
	for d, v := range dailyMap {
		daily = append(daily, CostPoint{Date: d, USD: round2c(v)})
	}
	sort.Slice(daily, func(i, j int) bool { return daily[i].Date < daily[j].Date })

	// Month-to-date + run-rate end-of-month forecast.
	monthPrefix := now.Format("2006-01")
	var mtd float64
	for _, pt := range daily {
		if len(pt.Date) >= 7 && pt.Date[:7] == monthPrefix {
			mtd += pt.USD
		}
	}
	daysInMonth := float64(time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day())
	dayOfMonth := float64(now.Day())
	forecast := mtd
	if dayOfMonth > 0 {
		forecast = mtd / dayOfMonth * daysInMonth
	}

	return &CostActuals{
		Currency:     "USD",
		WindowDays:   days,
		TotalUSD:     round2c(total),
		MTDUSD:       round2c(mtd),
		ForecastUSD:  round2c(forecast),
		DailyRunRate: round2c(trailingAvgC(daily, 7)),
		ByAccount:    bucketsC(acctMap, names),
		ByService:    bucketsC(svcMap, nil),
		Daily:        daily,
		Anomalies:    detectAnomaliesC(daily),
		Accounts:     accountRefsC(acctMap, names),
	}
}

func bucketsC(m map[string]float64, names map[string]string) []CostBucket {
	out := make([]CostBucket, 0, len(m))
	for k, v := range m {
		b := CostBucket{Key: k, USD: round2c(v)}
		if names != nil {
			b.Name = names[k]
		}
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].USD == out[j].USD {
			return out[i].Key < out[j].Key
		}
		return out[i].USD > out[j].USD
	})
	return out
}

func accountRefsC(m map[string]float64, names map[string]string) []CostAccountRef {
	out := make([]CostAccountRef, 0, len(m))
	for id := range m {
		out = append(out, CostAccountRef{ID: id, Name: names[id]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// trailingAvgC averages the last n daily points, dropping the final (partial) day
// when there is enough history so today's incomplete spend doesn't deflate the rate.
func trailingAvgC(daily []CostPoint, n int) float64 {
	pts := daily
	if len(pts) > 2 {
		pts = pts[:len(pts)-1]
	}
	if len(pts) == 0 {
		return 0
	}
	if len(pts) > n {
		pts = pts[len(pts)-n:]
	}
	var sum float64
	for _, p := range pts {
		sum += p.USD
	}
	return sum / float64(len(pts))
}

func detectAnomaliesC(daily []CostPoint) []CostAnomaly {
	out := []CostAnomaly{}
	for i := 7; i < len(daily); i++ {
		var sum float64
		for j := i - 7; j < i; j++ {
			sum += daily[j].USD
		}
		baseline := sum / 7
		if baseline <= 0 {
			continue
		}
		factor := daily[i].USD / baseline
		if daily[i].USD >= 1 && factor >= 2 {
			out = append(out, CostAnomaly{
				Date:        daily[i].Date,
				USD:         daily[i].USD,
				BaselineUSD: round2c(baseline),
				Factor:      round2c(factor),
			})
		}
	}
	return out
}

func round2c(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}
