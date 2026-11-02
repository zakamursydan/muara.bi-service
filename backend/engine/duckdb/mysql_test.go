package duckdb

import (
	"errors"
	"strings"
	"testing"
)

func TestMySQLDSNEmitsTheKeysTheScannerReads(t *testing.T) {
	dsn, err := MySQLDSN(MySQLSource{
		Host: "dwh.internal", Port: 3306, User: "reader", Passwd: "s3cret", DB: "sales",
	})
	if err != nil {
		t.Fatalf("MySQLDSN: %v", err)
	}
	want := "host=dwh.internal port=3306 user=reader passwd=s3cret db=sales"
	if dsn != want {
		t.Errorf("dsn = %q, want %q", dsn, want)
	}
}

// An empty value is not the same as an absent one here: `passwd=` breaks the
// scanner's tokenising, while omitting the key connects without a password.
func TestMySQLDSNOmitsAbsentFields(t *testing.T) {
	dsn, err := MySQLDSN(MySQLSource{Host: "db.internal", Port: 3306, User: "root"})
	if err != nil {
		t.Fatalf("MySQLDSN: %v", err)
	}
	if dsn != "host=db.internal port=3306 user=root" {
		t.Errorf("dsn = %q, want the three keys that have values", dsn)
	}

	// Port 0 is what an unset port looks like, and the scanner already defaults
	// it to 3306 — emitting "port=0" would advertise a port that is not used.
	noPort, err := MySQLDSN(MySQLSource{Host: "db.internal", User: "root", Passwd: "pw"})
	if err != nil {
		t.Fatalf("MySQLDSN: %v", err)
	}
	if strings.Contains(noPort, "port=") {
		t.Errorf("dsn = %q, want no port key when the port is unset", noPort)
	}
}

// Each case is a character class the scanner treats differently, and every
// expectation was measured against mysql_scanner (see TestMySQLDSNsAreAcceptedBy
// -TheScanner): a value only needs quoting when whitespace or "=" would split
// it, or when it opens with a quote, and a backslash must only be doubled inside
// quotes because unquoted it is already verbatim.
func TestMySQLDSNQuotesWhatCannotStandAlone(t *testing.T) {
	cases := []struct {
		name   string
		passwd string
		want   string
	}{
		{"space", "su per secret", `"su per secret"`},
		{"tab", "a\tb", "\"a\tb\""},
		{"equals", "a=b", `"a=b"`},
		{"at-and-slash", "p@ss/w:ord", `p@ss/w:ord`},
		{"colon-path", `C:\path`, `C:\path`},
		{"backslash-then-space", `C:\My Files`, `"C:\\My Files"`},
		{"inner-quote", `say"hi"`, `say"hi"`},
		{"quote-and-space", `say "hi"`, `"say \"hi\""`},
		{"leading-quote", `"leading`, `"\"leading"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dsn, err := MySQLDSN(MySQLSource{Host: "db.internal", User: "root", Passwd: c.passwd})
			if err != nil {
				t.Fatalf("MySQLDSN: %v", err)
			}
			// Asserting the whole dsn keeps a stray quote in one value from being
			// hidden by a substring match on another field.
			if want := `host=db.internal user=root passwd=` + c.want; dsn != want {
				t.Errorf("dsn = %q, want %q", dsn, want)
			}
		})
	}
}

func TestMySQLDSNRefusesSourcesTheScannerCannotServe(t *testing.T) {
	for _, src := range []MySQLSource{
		{Host: "", Port: 3306, User: "root"},
		{Host: "   ", Port: 3306, User: "root"},
		{Host: "db.internal", Port: -1},
		{Host: "db.internal", Port: 70000},
	} {
		dsn, err := MySQLDSN(src)
		if !errors.Is(err, ErrUnusableMySQLSource) {
			t.Errorf("%+v accepted as %q", src, dsn)
		}
		if dsn != "" {
			t.Errorf("%+v returned a usable dsn %q", src, dsn)
		}
	}
}

func TestMySQLDSNTrimsTheHostOnly(t *testing.T) {
	// A pasted host is a paste artefact; a pasted password is the password.
	dsn, err := MySQLDSN(MySQLSource{Host: "  db.internal  ", User: " root ", Passwd: " pw "})
	if err != nil {
		t.Fatalf("MySQLDSN: %v", err)
	}
	if !strings.HasPrefix(dsn, "host=db.internal ") {
		t.Errorf("dsn = %q, want the trimmed host first", dsn)
	}
	if !strings.Contains(dsn, `user=" root "`) || !strings.Contains(dsn, `passwd=" pw "`) {
		t.Errorf("dsn = %q, want the padded credentials quoted, not trimmed", dsn)
	}
}
