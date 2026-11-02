package duckdb

import (
	"fmt"
	"strconv"
	"strings"
)

// MySQLSource is one source the mysql_scanner can attach to. The scanner reads a
// space-separated key=value string, so which characters survive is a property of
// the scanner, not of a product: it lives here so both products share it.
type MySQLSource struct {
	Host   string
	Port   int
	User   string
	Passwd string
	DB     string
	Socket string
}

// ErrUnusableMySQLSource reports a source the scanner cannot be told about.
var ErrUnusableMySQLSource = fmt.Errorf("unusable mysql source")

// MySQLDSN renders a source the way mysql_scanner parses one.
//
// The measured grammar (DuckDB 1.1.3, probed 2026-11-02 against a dead address,
// so a parse error and a connection error are distinguishable):
//
//   - only host, port, user, passwd, db and socket are accepted; a key the
//     scanner does not know fails with "Unrecognized configuration parameter",
//     and so does a value containing "=" because it splits into a second, empty
//     key;
//   - a value containing whitespace is only representable inside double quotes —
//     unquoted it leaves a stray token and "expected key=value pairs separated
//     by spaces";
//   - a value that merely contains a double quote survives unquoted, but one
//     that *starts* with a quote opens quoted mode and fails "unterminated
//     quote";
//   - inside quotes the scanner escapes "\" and '"' and rejects every other
//     escape, so a lone backslash must be doubled — while outside quotes a
//     backslash is taken verbatim and must not be touched;
//   - an empty value is a parse error unquoted (`db=`) and representable as "";
//   - port 0 becomes 3306, and `user:password@tcp(host:port)/db`, the form Go
//     drivers use, is rejected outright.
func MySQLDSN(src MySQLSource) (string, error) {
	host := strings.TrimSpace(src.Host)
	if host == "" {
		return "", fmt.Errorf("%w: host is required", ErrUnusableMySQLSource)
	}
	if src.Port < 0 || src.Port > 65535 {
		return "", fmt.Errorf("%w: port %d is outside 0-65535", ErrUnusableMySQLSource, src.Port)
	}

	fields := []struct {
		key   string
		value string
		skip  bool
	}{
		{key: "host", value: host},
		// Port 0 is what a profile with no port set looks like, and the scanner
		// already defaults it to 3306 — sending "port=0" would be a lie.
		{key: "port", value: strconv.Itoa(src.Port), skip: src.Port == 0},
		{key: "user", value: src.User, skip: src.User == ""},
		{key: "passwd", value: src.Passwd, skip: src.Passwd == ""},
		{key: "db", value: src.DB, skip: src.DB == ""},
		{key: "socket", value: src.Socket, skip: src.Socket == ""},
	}

	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		if f.skip {
			continue
		}
		parts = append(parts, f.key+"="+mysqlDSNValue(f.value))
	}
	return strings.Join(parts, " "), nil
}

// mysqlDSNValue quotes only when the value needs it, because the unquoted form
// is what every working deployment produces today — and unquoted is also the
// only form in which a backslash stays verbatim.
func mysqlDSNValue(v string) string {
	if v == "" || strings.ContainsAny(v, " \t\n\r=") || v[0] == '"' {
		escaped := strings.ReplaceAll(v, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		return `"` + escaped + `"`
	}
	return v
}
