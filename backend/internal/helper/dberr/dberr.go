package dberr

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// UniqueViolation is the PostgreSQL SQLSTATE raised when an insert or update
// breaks a unique index or primary key.
const UniqueViolation = "23505"

// IsUniqueViolation reports whether err wraps a PostgreSQL unique constraint
// violation, letting services translate a database conflict into HTTP 409.
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == UniqueViolation
}
