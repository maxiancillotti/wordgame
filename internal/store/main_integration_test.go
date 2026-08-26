//go:build integration

package store_test

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	testcont "github.com/testcontainers/testcontainers-go/modules/postgres"

	"gitlab.com/maxi.ancillotti/maxkit/pkg/config"
	"gitlab.com/maxi.ancillotti/maxkit/pkg/infra/postgres"
	"gitlab.com/maxi.ancillotti/maxkit/pkg/infra/postgres/migrate"
)

const (
	testDBUsername = "test"
	testDBPassword = "test"
)

var testDBName = "testdb_" + uuid.NewString()

// testPool is shared by every test in this package: starting a Postgres
// container per test would be needlessly slow, so TestMain starts one for
// the whole run and each test truncates its own data (see
// helpers_integration_test.go's truncateAll).
var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()

	pgContainer, err := testcont.Run(ctx, "postgres:16-alpine",
		testcont.WithDatabase(testDBName),
		testcont.WithUsername(testDBUsername),
		testcont.WithPassword(testDBPassword),
		testcont.BasicWaitStrategies(),
	)
	if err != nil {
		log.Fatalf("start postgres container: %v", err)
	}
	defer func() { _ = pgContainer.Terminate(context.Background()) }()

	host, err := pgContainer.Host(ctx)
	if err != nil {
		log.Fatalf("get container host: %v", err)
	}
	mappedPort, err := pgContainer.MappedPort(ctx, "5432/tcp")
	if err != nil {
		log.Fatalf("get mapped port: %v", err)
	}

	cfg := config.PostgreSQLConfig{
		Username: testDBUsername,
		Password: testDBPassword,
		Host:     host,
		Port:     mappedPort.Port(),
		DBName:   testDBName,
		SSLMode:  "disable",
	}

	if err := migrate.RunMigrations(cfg, migrationsDir()); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	pool, err := postgres.NewConnPool(ctx, cfg)
	if err != nil {
		log.Fatalf("connect to test database: %v", err)
	}
	defer pool.Close()

	testPool = pool

	os.Exit(m.Run())
}

// migrationsDir resolves the absolute path to wordgame's repo-root
// migrations directory, independent of the test binary's working
// directory (go test runs with the package directory as cwd, not the repo
// root, so a relative "migrations" path wouldn't resolve).
func migrationsDir() string {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("resolve migrationsDir: runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "migrations")
}
