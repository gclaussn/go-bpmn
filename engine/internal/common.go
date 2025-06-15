package internal

import (
	"time"

	"github.com/adhocore/gronx"
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/jackc/pgx/v5/pgtype"
)

func evaluateTimer(timer engine.Timer, start time.Time) (time.Time, error) {
	if !timer.Time.IsZero() {
		// must be UTC and truncated to millis (see engine/pg/pg.go:pgEngineWithContext#require)
		return timer.Time.UTC().Truncate(time.Millisecond), nil
	} else if timer.TimeCycle != "" {
		return gronx.NextTickAfter(timer.TimeCycle, start, false)
	} else if !timer.TimeDuration.IsZero() {
		return timer.TimeDuration.Calculate(start), nil
	} else {
		return start, nil
	}
}

func timeOrNil(v pgtype.Timestamp) *time.Time {
	if !v.Valid {
		return nil
	}
	return &v.Time
}
