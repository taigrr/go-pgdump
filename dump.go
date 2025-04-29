package pgdump

import (
	"context"
	"errors"
	"io"
	"os/exec"
)

type Opts struct {
	Host     string
	Port     string
	User     string
	Password string
}

var (
	// ErrNotInstalled is returned when pg_dump is not found in the PATH.
	ErrNotInstalled = errors.New("pg_dump not installed")

	pgDumpPath    string
	pgDumpAllPath string
)

func init() {
	var err error
	pgDumpPath, err = exec.LookPath("pg_dump")
	if err != nil {
		pgDumpPath = ""
	}
	pgDumpAllPath, err = exec.LookPath("pg_dumpall")
	if err != nil {
		pgDumpAllPath = ""
	}
}

// DumpDB starts a pg_dump process and returns an io.ReadCloser for the dump output.
// Caller MUST call Close() to avoid leaking processes.
func DumpDB(ctx context.Context, dbName string, opts Opts) (io.ReadCloser, error) {
	if pgDumpPath == "" {
		return nil, ErrNotInstalled
	}
	args := []string{
		"-h", opts.Host,
		"-p", opts.Port,
		"-U", opts.User,
		"-d", dbName,
	}

	cmd := exec.CommandContext(ctx, pgDumpPath, args...)

	// Set password safely via environment
	cmd.Env = []string{"PGPASSWORD=" + opts.Password}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return &dumpReader{cmd: cmd, pipe: stdout}, nil
}

// DumpAll starts a pg_dumpall process and returns an io.ReadCloser for the dump output.
// Caller MUST call Close() to avoid leaking processes.
func DumpAll(ctx context.Context, dbName string, opts Opts) (io.ReadCloser, error) {
	if pgDumpPath == "" {
		return nil, ErrNotInstalled
	}
	args := []string{
		"-h", opts.Host,
		"-p", opts.Port,
		"-U", opts.User,
	}

	cmd := exec.CommandContext(ctx, pgDumpAllPath, args...)

	// Set password safely via environment
	cmd.Env = []string{"PGPASSWORD=" + opts.Password}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return &dumpReader{cmd: cmd, pipe: stdout}, nil
}
