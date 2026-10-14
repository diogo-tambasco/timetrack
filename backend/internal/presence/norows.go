package presence

import (
	"errors"

	"github.com/jackc/pgx/v5"
)

// isNoRows = a query não encontrou linha (ex.: projeto não visível ao viewer).
func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
