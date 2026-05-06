package pgdump

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"os/exec"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func mustClose[T interface{ Close() error }](t *testing.T, label string, closer T) {
	t.Helper()
	if err := closer.Close(); err != nil {
		t.Errorf("close %s: %v", label, err)
	}
}

func mustTerminateContainer(t *testing.T, ctx context.Context, container testcontainers.Container) {
	t.Helper()
	if err := container.Terminate(ctx); err != nil {
		t.Errorf("terminate container: %v", err)
	}
}

func setupPostgres(t *testing.T) (string, func()) {
	t.Helper()
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "postgres:15",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "testdb1",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}

	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("Failed to get port: %v", err)
	}

	connStr := "postgres://test:test@localhost:" + port.Port() + "/testdb1?sslmode=disable"

	// Create second database with retry.
	var db *sql.DB
	for range 5 {
		db, err = sql.Open("postgres", connStr)
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		_, err = db.Exec("CREATE DATABASE testdb2")
		if err == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if err != nil {
		t.Fatalf("Failed to create second database after retries: %v", err)
	}
	defer mustClose(t, "primary db", db)

	// Add test data to both databases.
	connStr2 := "postgres://test:test@localhost:" + port.Port() + "/testdb2?sslmode=disable"
	db2, err := sql.Open("postgres", connStr2)
	if err != nil {
		t.Fatalf("Failed to connect to second db: %v", err)
	}
	defer mustClose(t, "secondary db", db2)

	for _, conn := range []*sql.DB{db, db2} {
		_, err = conn.Exec(`
			CREATE TABLE users (
				id SERIAL PRIMARY KEY,
				name TEXT NOT NULL,
				email TEXT UNIQUE NOT NULL
			);
			INSERT INTO users (name, email) VALUES 
				('Alice', 'alice@example.com'),
				('Bob', 'bob@example.com');
		`)
		if err != nil {
			t.Fatalf("Failed to create test data: %v", err)
		}
	}

	return port.Port(), func() {
		mustTerminateContainer(t, ctx, container)
	}
}

func requireBinary(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("%s not installed", name)
	}
}

func TestDumpDB(t *testing.T) {
	requireBinary(t, "pg_dump")

	port, cleanup := setupPostgres(t)
	defer cleanup()

	opts := Opts{
		Host:     "localhost",
		Port:     port,
		User:     "test",
		Password: "test",
	}

	ctx := context.Background()
	reader, err := DumpDB(ctx, "testdb1", opts)
	if err != nil {
		t.Fatalf("DumpDB failed: %v", err)
	}
	defer mustClose(t, "DumpDB reader", reader)

	dump, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("Failed to read dump: %v", err)
	}

	if len(dump) == 0 {
		t.Fatal("Dump is empty")
	}

	content := string(dump)
	if !strings.Contains(content, "CREATE TABLE") {
		t.Error("Dump missing CREATE TABLE statement")
	}
	if !strings.Contains(content, "alice@example.com") {
		t.Error("Dump missing inserted test data")
	}
}

func TestDumpDBWithExtraArgs(t *testing.T) {
	requireBinary(t, "pg_dump")

	port, cleanup := setupPostgres(t)
	defer cleanup()

	opts := Opts{
		Host:      "localhost",
		Port:      port,
		User:      "test",
		Password:  "test",
		ExtraArgs: []string{"--schema-only"},
	}

	ctx := context.Background()
	reader, err := DumpDB(ctx, "testdb1", opts)
	if err != nil {
		t.Fatalf("DumpDB failed: %v", err)
	}
	defer mustClose(t, "schema-only DumpDB reader", reader)

	dump, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("Failed to read dump: %v", err)
	}

	content := string(dump)
	if !strings.Contains(content, "CREATE TABLE") {
		t.Error("Schema-only dump missing CREATE TABLE statement")
	}
	if strings.Contains(content, "alice@example.com") {
		t.Error("Schema-only dump should not contain row data")
	}
}

func TestDumpAll(t *testing.T) {
	requireBinary(t, "pg_dumpall")

	port, cleanup := setupPostgres(t)
	defer cleanup()

	opts := Opts{
		Host:     "localhost",
		Port:     port,
		User:     "test",
		Password: "test",
	}

	ctx := context.Background()
	reader, err := DumpAll(ctx, opts)
	if err != nil {
		t.Fatalf("DumpAll failed: %v", err)
	}
	defer mustClose(t, "DumpAll reader", reader)

	dump, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("Failed to read dump: %v", err)
	}

	if len(dump) == 0 {
		t.Fatal("Dump is empty")
	}

	content := string(dump)
	if !strings.Contains(content, "CREATE ROLE") && !strings.Contains(content, "CREATE DATABASE") {
		t.Error("DumpAll output missing expected cluster-level statements")
	}
}

func TestDumpDBNotInstalled(t *testing.T) {
	orig := pgDumpPath
	pgDumpPath = ""
	defer func() { pgDumpPath = orig }()

	_, err := DumpDB(context.Background(), "test", Opts{})
	if err != ErrPGDumpNotInstalled {
		t.Errorf("Expected ErrPGDumpNotInstalled, got: %v", err)
	}
}

func TestDumpAllNotInstalled(t *testing.T) {
	orig := pgDumpAllPath
	pgDumpAllPath = ""
	defer func() { pgDumpAllPath = orig }()

	_, err := DumpAll(context.Background(), Opts{})
	if err != ErrPGDumpAllNotInstalled {
		t.Errorf("Expected ErrPGDumpAllNotInstalled, got: %v", err)
	}
}

func TestErrNotInstalledBackwardCompat(t *testing.T) {
	// ErrNotInstalled should still match ErrPGDumpNotInstalled for
	// callers that used errors.Is with the old sentinel.
	if ErrNotInstalled != ErrPGDumpNotInstalled {
		t.Error("ErrNotInstalled should equal ErrPGDumpNotInstalled")
	}
}

func TestDumpReaderNilClose(t *testing.T) {
	var reader *dumpReader
	err := reader.Close()
	if err == nil {
		t.Error("Expected error when closing nil dumpReader")
	}
}

type stubReadCloser struct {
	closeErr error
}

func (stubReadCloser) Read([]byte) (int, error) {
	return 0, io.EOF
}

func (s stubReadCloser) Close() error {
	return s.closeErr
}

func TestDumpReaderCloseIncludesStderr(t *testing.T) {
	cmd := exec.Command("sh", "-c", "printf 'permission denied' >&2; exit 7")

	reader, err := startDump(cmd)
	if err != nil {
		t.Fatalf("startDump failed: %v", err)
	}

	_, _ = io.ReadAll(reader)

	err = reader.Close()
	if err == nil {
		t.Fatal("expected Close to return error")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("expected stderr in error, got: %v", err)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected wrapped exec.ExitError, got %T", err)
	}
}

func TestDumpReaderCloseReturnsPipeAndWaitErrors(t *testing.T) {
	pipeErr := errors.New("pipe close failed")
	cmd := exec.Command("sh", "-c", "printf 'bad dump' >&2; exit 3")
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe failed: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer stdout.Close()

	err = (&dumpReader{
		cmd:    cmd,
		pipe:   stubReadCloser{closeErr: pipeErr},
		stderr: stderr,
	}).Close()
	if err == nil {
		t.Fatal("expected combined error")
	}
	if !strings.Contains(err.Error(), pipeErr.Error()) {
		t.Fatalf("expected pipe close error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "bad dump") {
		t.Fatalf("expected stderr in combined error, got: %v", err)
	}
}

func TestAppendPasswordAppendsToEnvironment(t *testing.T) {
	origEnviron := environ
	t.Cleanup(func() { environ = origEnviron })

	environ = func() []string {
		return []string{"PGPASSWORD=old", "PATH=/tmp/bin"}
	}

	env := appendPassword("secret")
	if got, want := env[len(env)-1], "PGPASSWORD=secret"; got != want {
		t.Fatalf("appendPassword last entry = %q, want %q", got, want)
	}
	if got, want := env[0], "PGPASSWORD=old"; got != want {
		t.Fatalf("appendPassword should preserve existing env order, got first %q want %q", got, want)
	}
}
