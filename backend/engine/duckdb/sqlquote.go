package duckdb

import "strings"

// These two helpers are not CGO-dependent, and the file-source mapping that
// uses them is validated in unit tests even in a CGO-free build — so they live
// outside session_cgo.go instead of being duplicated there.

// quoteIdent wraps an object name in double quotes so DuckDB reads it as one
// identifier instead of re-parsing characters like "-".
func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// quoteLiteral turns driver text into a DuckDB string literal. Credentials are
// user supplied, so an embedded quote must not be able to close the ATTACH text.
func quoteLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
