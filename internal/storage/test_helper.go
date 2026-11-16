package storage

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

// TB is a minimal interface implemented by *testing.T.
type TB interface {
	Helper()
	Skipf(format string, args ...any)
	Fatalf(format string, args ...any)
}

// SetupIsolatedTestDB creates an isolated schema in PostgreSQL for test execution,
// builds a PostgresRepository scoped to that schema via search_path, optionally runs
// migrations, and returns a cleanup function that drops the schema.
func SetupIsolatedTestDB(tb TB, runMigrations bool) (*PostgresRepository, func()) {
	tb.Helper()

	baseDSN := os.Getenv("DATABASE_URL")
	if baseDSN == "" {
		baseDSN = "postgres://postgres:postgres@localhost:5432/havoc?sslmode=disable"
	}

	baseDB, err := sql.Open("postgres", baseDSN)
	if err != nil {
		tb.Skipf("Skipping test because database connection failed: %v", err)
		return nil, func() {}
	}

	if err := baseDB.Ping(); err != nil {
		_ = baseDB.Close()
		tb.Skipf("Skipping test because database is not available: %v", err)
		return nil, func() {}
	}

	// Generate a unique, valid PostgreSQL schema name (lowercase letters/digits/underscores, <= 63 chars)
	cleanUUID := strings.ToLower(strings.ReplaceAll(uuid.New().String(), "-", ""))[:12]
	schemaName := fmt.Sprintf("test_%s_%d", cleanUUID, time.Now().UnixNano()%1000000)

	ctx := context.Background()
	if _, err := baseDB.ExecContext(ctx, fmt.Sprintf("CREATE SCHEMA %s", schemaName)); err != nil {
		_ = baseDB.Close()
		tb.Fatalf("failed to create isolated test schema %s: %v", schemaName, err)
		return nil, func() {}
	}

	// Inject search_path into the connection string
	isolatedDSN := buildDSNWithSearchPath(baseDSN, schemaName)

	repo, err := NewPostgresRepository(isolatedDSN)
	if err != nil {
		_, _ = baseDB.ExecContext(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schemaName))
		_ = baseDB.Close()
		tb.Fatalf("failed to initialize repository with schema %s: %v", schemaName, err)
		return nil, func() {}
	}

	if runMigrations {
		if err := repo.RunMigrations(ctx); err != nil {
			_ = repo.Close()
			_, _ = baseDB.ExecContext(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schemaName))
			_ = baseDB.Close()
			tb.Fatalf("failed to run migrations in schema %s: %v", schemaName, err)
			return nil, func() {}
		}
	}

	cleanup := func() {
		_ = repo.Close()
		dropCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = baseDB.ExecContext(dropCtx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schemaName))
		_ = baseDB.Close()
	}

	return repo, cleanup
}

func buildDSNWithSearchPath(dsn, schemaName string) string {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err == nil {
			q := u.Query()
			q.Set("search_path", schemaName)
			u.RawQuery = q.Encode()
			return u.String()
		}
	}

	if strings.Contains(dsn, "search_path=") {
		return dsn
	}
	return strings.TrimSpace(dsn) + fmt.Sprintf(" search_path=%s", schemaName)
}
