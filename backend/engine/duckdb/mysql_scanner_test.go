//go:build cgo

package duckdb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

// MySQLDSN is a guess about a third-party scanner until the scanner itself says
// so. Its option names are validated while the string is being read, so a
// connect attempt to a port nobody listens on still separates "the scanner
// understood this string" from "the scanner refused it" — no MySQL server needed.
//
// Requires the connector bundle that ships with the desktop app:
//
//	DUCKDB_EXTENSION_DIRECTORY=/Applications/muara-bi.app/Contents/Resources/duckdb/extensions \
//	  go test ./duckdb -run TestMySQLDSNs -v
func openBundledMySQLSession(t *testing.T) Session {
	t.Helper()
	bundle := os.Getenv("DUCKDB_EXTENSION_DIRECTORY")
	if bundle == "" {
		t.Skip("set DUCKDB_EXTENSION_DIRECTORY to the connector bundle to run this")
	}
	engine, err := NewEngine(GuardrailConfig{
		MaxMemory:          "256MB",
		ExtensionDirectory: bundle,
	})
	if errors.Is(err, ErrCGODisabled) {
		t.Skip(ErrCGODisabled.Error())
	}
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	t.Cleanup(func() { _ = engine.Close() })

	session, err := engine.NewSession(context.Background())
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

// attachClass is what one ATTACH attempt proved, in order of how much it says.
func attachClass(err error) string {
	switch {
	case err == nil:
		return "connected"
	case strings.Contains(err.Error(), "Can't connect to MySQL server"),
		strings.Contains(err.Error(), "Connection refused"):
		// The scanner read every key=value pair and handed them to the driver.
		return "parsed"
	default:
		return "refused: " + err.Error()
	}
}

func TestMySQLDSNsAreAcceptedByTheScanner(t *testing.T) {
	session := openBundledMySQLSession(t)
	ctx := context.Background()

	sources := []MySQLSource{
		{Host: "127.0.0.1", Port: 1, User: "root", Passwd: "s3cret", DB: "sales"},
		{Host: "127.0.0.1", Port: 1, User: "root", Passwd: "su per secret"},
		{Host: "127.0.0.1", Port: 1, User: "root", Passwd: "a=b"},
		{Host: "127.0.0.1", Port: 1, User: "root", Passwd: "p@ss/w:ord", DB: "sales"},
		{Host: "127.0.0.1", Port: 1, User: "root", Passwd: `C:\path`},
		{Host: "127.0.0.1", Port: 1, User: "root", Passwd: `C:\My Files`},
		{Host: "127.0.0.1", Port: 1, User: "root", Passwd: `say"hi"`},
		{Host: "127.0.0.1", Port: 1, User: "root", Passwd: `say "hi"`},
		{Host: "127.0.0.1", Port: 1, User: "root", Passwd: `"leading`},
		{Host: "127.0.0.1", Port: 1, User: "root", Passwd: "a\tb"},
		{Host: "127.0.0.1", User: "root"},
	}

	for i, src := range sources {
		dsn, err := MySQLDSN(src)
		if err != nil {
			t.Fatalf("MySQLDSN(%+v): %v", src, err)
		}
		err = session.AttachMySQL(ctx, fmt.Sprintf("m%d", i), dsn)
		if class := attachClass(err); strings.HasPrefix(class, "refused") {
			t.Errorf("scanner refused dsn %q: %s", dsn, class)
		}
	}
}

// The two forms that used to reach this code silently: the Go driver's DSN, and
// a MySQL profile that asked for a private CA. Both must fail at the string, not
// halfway through a query.
func TestMySQLScannerRefusesTheFormsWeDoNotEmit(t *testing.T) {
	session := openBundledMySQLSession(t)
	ctx := context.Background()

	for i, dsn := range []string{
		"root:s3cret@tcp(127.0.0.1:1)/sales",
		"mysql://root:s3cret@127.0.0.1:1/sales",
		"host=127.0.0.1 port=1 user=root passwd=s3cret ssl-ca=/etc/ssl/internal-ca.pem",
		"host=127.0.0.1 port=1 user=root rootcert=/etc/ssl/internal-ca.pem",
	} {
		err := session.AttachMySQL(ctx, fmt.Sprintf("r%d", i), dsn)
		if err == nil || !strings.Contains(err.Error(), "Invalid dsn") &&
			!strings.Contains(err.Error(), "Unrecognized configuration parameter") {
			t.Errorf("%q: want a parse-level refusal, got %v", dsn, err)
		}
	}
}
