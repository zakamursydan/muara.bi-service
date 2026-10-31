package generated

import "embed"

// Migrations holds the goose SQL files so a release binary migrates whatever
// database it points at, with no schema files on disk next to the executable.
//
//go:embed migrations/*.sql
var Migrations embed.FS
