// Package sqlident guards SQL identifiers that must be interpolated into query
// text instead of bound as parameters (RLS columns, visual builder columns).
package sqlident

import "regexp"

var (
	plain     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	qualified = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*){0,2}$`)
)

// IsPlain reports whether s is a single safe identifier such as branch_id.
func IsPlain(s string) bool {
	return plain.MatchString(s)
}

// IsQualified reports whether s is a safe dotted identifier such as
// db_transaksi.orders.branch_id.
func IsQualified(s string) bool {
	return qualified.MatchString(s)
}
