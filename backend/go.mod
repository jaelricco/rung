module calisthenics/api

go 1.24

require (
	github.com/jackc/pgx/v5 v5.7.2
	golang.org/x/crypto v0.32.0
)

// Indirect requirements have to be listed explicitly since go 1.17's module
// graph pruning: a build resolves nothing, so anything the build needs must
// already be here. They were missing, and only CI's `go mod tidy` was papering
// over it — the tidied file was never committed.
require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	golang.org/x/sync v0.10.0 // indirect
	golang.org/x/sys v0.29.0 // indirect
	golang.org/x/text v0.21.0 // indirect
)
