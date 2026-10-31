// Command duckdb-bundle materialises the DuckDB connector extensions into a
// fixed directory so the production image can ship them and a container with no
// network egress can still ATTACH external databases.
//
// DuckDB is asked to do the downloading itself: it picks the correct
// <version>/<platform> subdirectory for the libduckdb it was built against,
// which a hardcoded curl URL would have to guess.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/marcboeker/go-duckdb"
)

// extensions are the connectors the query engine attaches with. Names are the
// ones the runtime LOAD statements use.
var extensions = []string{"postgres", "mysql", "sqlite"}

func main() {
	out := flag.String("out", "", "directory to install extensions into (required)")
	install := flag.Bool("install", true, "download extensions; when false only verify they load")
	flag.Parse()

	if *out == "" {
		fmt.Fprintln(os.Stderr, "duckdb-bundle: -out is required")
		os.Exit(2)
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "duckdb-bundle: %v\n", err)
		os.Exit(1)
	}

	if err := run(*out, *install); err != nil {
		fmt.Fprintf(os.Stderr, "duckdb-bundle: %v\n", err)
		os.Exit(1)
	}
}

func run(dir string, install bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	db, err := sql.Open("duckdb", "")
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err := db.ExecContext(ctx, "SET extension_directory="+quote(dir)); err != nil {
		return fmt.Errorf("set extension_directory: %w", err)
	}

	var version string
	if err := db.QueryRowContext(ctx, "SELECT version()").Scan(&version); err != nil {
		return fmt.Errorf("read duckdb version: %w", err)
	}
	fmt.Printf("duckdb %s, extensions directory %s\n", version, dir)

	for _, name := range extensions {
		if install {
			if _, err := db.ExecContext(ctx, "INSTALL "+name); err != nil {
				return fmt.Errorf("install %s: %w", name, err)
			}
		}
		if _, err := db.ExecContext(ctx, "LOAD "+name); err != nil {
			return fmt.Errorf("load %s: %w", name, err)
		}
		status := "verified"
		if install {
			status = "installed"
		}
		fmt.Printf("%-8s %s\n", name, status)
	}
	return nil
}

func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
