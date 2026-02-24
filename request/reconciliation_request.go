package request

import (
	"fmt"
	"time"
)

type RunReconciliationRequest struct {
	PeriodStart             time.Time `json:"period_start"`
	PeriodEnd               time.Time `json:"period_end"`
	Processors              []string  `json:"processors,omitempty"`
	IncludeTemporalAnalysis bool      `json:"include_temporal_analysis,omitempty"`
}

func (r *RunReconciliationRequest) Validate() error {
	if r.PeriodStart.IsZero() {
		return fmt.Errorf("period_start is required")
	}
	if r.PeriodEnd.IsZero() {
		return fmt.Errorf("period_end is required")
	}
	if r.PeriodEnd.Before(r.PeriodStart) {
		return fmt.Errorf("period_end must be after period_start")
	}
	return nil
}
