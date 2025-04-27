# go-pgdump

A Go package shim for running PostgreSQL database dumps using `pg_dump` and `pg_dumpall`.

## Features

- Run `pg_dump` for single database dumps
- Run `pg_dumpall` for full cluster dumps
- Stream dump output directly to your application
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

// Read the dump output
dump, err := io.ReadAll(reader)
if err != nil {
    log.Fatal(err)
}
```

### Full Cluster Dump

```go
reader, err := pgdump.DumpAll(ctx, "", opts)
if err != nil {
    log.Fatal(err)
}
defer reader.Close()
```

## Development

### Prerequisites

- Go 1.24.2+
- Docker (for running tests)
- PostgreSQL client tools (`pg_dump`, `pg_dumpall`)

### Running Tests

```bash
go test -v
```

Tests use testcontainers to spin up a PostgreSQL instance and execute both single database and full cluster dumps.

## License

0BSD
