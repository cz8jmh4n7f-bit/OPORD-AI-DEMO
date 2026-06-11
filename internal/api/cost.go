package api

import "net/http"

type costLineDTO struct {
	Name        string   `json:"name"`
	Kind        string   `json:"kind"`
	Provider    string   `json:"provider"`
	Environment string   `json:"environment"`
	Status      string   `json:"status"`
	MonthlyUSD  float64  `json:"monthlyUsd"`
	Owner       string   `json:"owner,omitempty"`
	Project     string   `json:"project,omitempty"`
	CostCenter  string   `json:"costCenter,omitempty"`
	TTLHours    int      `json:"ttlHours,omitempty"`
	RiskFlags   []string `json:"riskFlags,omitempty"`
}

type costReportDTO struct {
	Lines    []costLineDTO `json:"lines"`
	TotalUSD float64       `json:"totalUsd"`
}

func (s *Server) getCost(w http.ResponseWriter, r *http.Request) {
	rep, err := s.svc.CostReport(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	lines := make([]costLineDTO, 0, len(rep.Lines))
	for _, l := range rep.Lines {
		lines = append(lines, costLineDTO{
			Name:        l.Name,
			Kind:        l.Kind,
			Provider:    l.Provider,
			Environment: l.Environment,
			Status:      l.Status,
			MonthlyUSD:  l.MonthlyUSD,
			Owner:       l.Owner,
			Project:     l.Project,
			CostCenter:  l.CostCenter,
			TTLHours:    l.TTLHours,
			RiskFlags:   l.RiskFlags,
		})
	}
	writeJSON(w, http.StatusOK, costReportDTO{Lines: lines, TotalUSD: rep.TotalUSD})
}
