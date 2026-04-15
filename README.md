# go-pgdump

[![Tests](https://github.com/taigrr/go-pgdump/actions/workflows/test.yml/badge.svg)](https://github.com/taigrr/go-pgdump/actions/workflows/test.yml)

A Go package shim for running PostgreSQL database dumps using `pg_dump` and `pg_dumpall`.

## Features

- Run `pg_dump` for single database dumps
- Run `pg_dumpall` for full cluster dumps
- Stream dump output directly to your application
- Pass extra flags (`--schema-only`, `--format=custom`, etc.)
- Captured stderr on failure for clear diagnostics
- Automatic process cleanup

## Installation

```bash
go get github.com/taigrr/go-pgdump
```

## Usage

### Single Database Dump

```go
import "github.com/taigrr/go-pgdump"

opts := pgdump.Opts{
    Host:     "localhost",
    Port:     "5432",
    User:     "postgres",
    Password: "secret",
}

reader, err := pgdump.DumpDB(ctx, "mydb", opts)
if err != nil {
    log.Fatal(err)
}
defer reader.Close()

dump, err := io.ReadAll(reader)
if err != nil {
    log.Fatal(err)
}
```

### Schema-Only Dump

```go
opts := pgdump.Opts{
    Host:      "localhost",
    Port:      "5432",
    User:      "postgres",
    Password:  "secret",
    ExtraArgs: []string{"--schema-only"},
}

reader, err := pgdump.DumpDB(ctx, "mydb", opts)
```

### Full Cluster Dump

```go
reader, err := pgdump.DumpAll(ctx, opts)
if err != nil {
    log.Fatal(err)
}
defer reader.Close()
```

## Error Handling

When `pg_dump` exits with an error, the stderr output is included in the
error returned by `Close()`:

```go
reader, err := pgdump.DumpDB(ctx, "nonexistent", opts)
// ...
_, _ = io.ReadAll(reader)
if err := reader.Close(); err != nil {
    // err contains the pg_dump stderr output
    log.Fatal(err)
}
```

Sentinel errors for missing binaries:

```go
if errors.Is(err, pgdump.ErrPGDumpNotInstalled) { ... }
if errors.Is(err, pgdump.ErrPGDumpAllNotInstalled) { ... }
```

## Development

### Prerequisites

- Go 1.26+
- Docker (for running tests)
- PostgreSQL client tools (`pg_dump`, `pg_dumpall`)

### Running Tests

```bash
go test -v -race ./...
```

Tests use testcontainers to spin up a PostgreSQL instance and run both
single-database and full-cluster dumps.

## License

0BSD
