package internal

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func timeOrNil(v pgtype.Timestamp) *time.Time {
	if !v.Valid {
		return nil
	}
	return &v.Time
}
