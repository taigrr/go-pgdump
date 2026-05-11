package pgdump

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// Opts configures the connection parameters for pg_dump and pg_dumpall.
type Opts struct {
	// Host is the database server hostname or IP address.
	Host string
	// Port is the database server port.
	Port string
	// User is the database user to connect as.
	User string
	// Password is the database user's password, passed via PGPASSWORD env var.
	Password string
	// ExtraArgs holds additional command-line flags passed directly to
	// pg_dump or pg_dumpall (e.g. "--format=custom", "--schema-only").
	ExtraArgs []string
}

var (
	// ErrPGDumpNotInstalled is returned when pg_dump is not found in the PATH.
	ErrPGDumpNotInstalled = errors.New("pg_dump not installed")

	// ErrPGDumpAllNotInstalled is returned when pg_dumpall is not found in the PATH.
	ErrPGDumpAllNotInstalled = errors.New("pg_dumpall not installed")

	// ErrNotInstalled is an alias kept for backward compatibility.
	// Deprecated: use ErrPGDumpNotInstalled or ErrPGDumpAllNotInstalled.
	ErrNotInstalled = ErrPGDumpNotInstalled

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

// DumpDB starts a pg_dump process and returns an io.ReadCloser for the dump
// output. The caller MUST call Close on the returned reader to avoid leaking
// processes. If the underlying pg_dump process writes to stderr and then exits
// with a non-zero status, Close returns an error that includes the captured
// stderr output.
func DumpDB(ctx context.Context, dbName string, opts Opts) (io.ReadCloser, error) {
	if pgDumpPath == "" {
		return nil, ErrPGDumpNotInstalled
	}

	args := buildDumpDBArgs(dbName, opts)
	cmd := exec.CommandContext(ctx, pgDumpPath, args...)
	cmd.Env = appendPassword(opts.Password)

	return startDump(cmd)
}

// DumpAll starts a pg_dumpall process and returns an io.ReadCloser for the
// dump output. The caller MUST call Close on the returned reader to avoid
// leaking processes.
func DumpAll(ctx context.Context, opts Opts) (io.ReadCloser, error) {
	if pgDumpAllPath == "" {
		return nil, ErrPGDumpAllNotInstalled
	}

	args := buildDumpAllArgs(opts)
	cmd := exec.CommandContext(ctx, pgDumpAllPath, args...)
	cmd.Env = appendPassword(opts.Password)

	return startDump(cmd)
}

func buildDumpDBArgs(dbName string, opts Opts) []string {
	args := buildConnectionArgs(opts)
	args = append(args, "-d", dbName)
	args = append(args, opts.ExtraArgs...)
	return args
}

func buildDumpAllArgs(opts Opts) []string {
	args := buildConnectionArgs(opts)
	args = append(args, opts.ExtraArgs...)
	return args
}

func buildConnectionArgs(opts Opts) []string {
	args := make([]string, 0, 6)
	if opts.Host != "" {
		args = append(args, "-h", opts.Host)
	}
	if opts.Port != "" {
		args = append(args, "-p", opts.Port)
	}
	if opts.User != "" {
		args = append(args, "-U", opts.User)
	}
	return args
}

// startDump wires up stdout/stderr pipes and starts the command.
func startDump(cmd *exec.Cmd) (io.ReadCloser, error) {
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting dump process: %w", err)
	}

	return &dumpReader{cmd: cmd, pipe: stdout, stderr: &stderr}, nil
}

// appendPassword returns a copy of the current environment with the
// PGPASSWORD variable set.
func appendPassword(password string) []string {
	return append(environ(), "PGPASSWORD="+password)
}

// environ wraps os.Environ so tests can substitute it.
var environ = os.Environ
