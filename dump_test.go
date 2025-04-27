package pgdump

import (
	"context"
	"database/sql"
	"io"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupPostgres(t *testing.T) (string, func()) {
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "postgres:15",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "testdb1",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections"),
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

	// Create second database with retry
	var db *sql.DB
	for i := 0; i < 5; i++ {
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
	defer db.Close()

	// Add test data to both databases
	time.Sleep(time.Second) // Give the second database time to be ready
	connStr2 := "postgres://test:test@localhost:" + port.Port() + "/testdb2?sslmode=disable"
	db2, err := sql.Open("postgres", connStr2)
	if err != nil {
		t.Fatalf("Failed to connect to second db: %v", err)
	}
	defer db2.Close()

	// Create and populate tables in both databases
	for _, db := range []*sql.DB{db, db2} {
		_, err = db.Exec(`
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
		container.Terminate(ctx)
	}
}

func TestDumpDB(t *testing.T) {
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
	defer reader.Close()

	dump, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("Failed to read dump: %v", err)
	}

	if len(dump) == 0 {
		t.Error("Dump is empty")
	}
}

func TestDumpAll(t *testing.T) {
	port, cleanup := setupPostgres(t)
	defer cleanup()

	opts := Opts{
		Host:     "localhost",
		Port:     port,
		User:     "test",
		Password: "test",
	}

	ctx := context.Background()
	reader, err := DumpAll(ctx, "", opts)
	if err != nil {
		t.Fatalf("DumpAll failed: %v", err)
	}
	defer reader.Close()

	dump, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("Failed to read dump: %v", err)
	}

	if len(dump) == 0 {
		t.Error("Dump is empty")
	}
}
