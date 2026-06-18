package api

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// DTOs use camelCase JSON to match the web client's TypeScript types
// (web/src/lib/types.ts), so wiring the frontend is mechanical.

// tsPtr converts a nullable Postgres timestamp into a *time.Time for JSON.
func tsPtr(ts pgtype.Timestamptz) *time.Time {
	if !ts.Valid {
		return nil
	}
	t := ts.Time
	return &t
}
