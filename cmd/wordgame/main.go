// Command wordgame is the word-guessing game's entry point: it wires
// internal/store up to internal/service and serves internal/transport/rest
// over it, following the app.Init/app.Serve orchestration maxkit expects.
package main

import (
	"net/http"

	"go.uber.org/zap"

	"gitlab.com/maxi.ancillotti/maxkit/pkg/app"
	"gitlab.com/maxi.ancillotti/maxkit/pkg/infra/postgres"
	"gitlab.com/maxi.ancillotti/maxkit/pkg/infra/postgres/migrate"
	"gitlab.com/maxi.ancillotti/maxkit/pkg/obs/logging"

	"github.com/maxiancillotti/wordgame/internal/config"
	"github.com/maxiancillotti/wordgame/internal/service"
	"github.com/maxiancillotti/wordgame/internal/store"
	"github.com/maxiancillotti/wordgame/internal/transport/rest"
	"github.com/maxiancillotti/wordgame/seed"
)

func main() {
	ctx, stop, cfg, err := app.Init[config.ServiceConfig](config.ServiceName, app.RequirePostgreSQL())
	if err != nil {
		logging.From(ctx).Fatal("init failed", zap.Error(err))
	}
	logger := logging.From(ctx)
	defer stop()

	if err := migrate.RunMigrations(cfg.Infrastructure.PostgreSQL, "migrations"); err != nil {
		logger.Fatal("failed to run migrations", zap.Error(err))
	}

	pool, err := postgres.NewConnPool(ctx, cfg.Infrastructure.PostgreSQL)
	if err != nil {
		logger.Fatal("failed to connect to postgres", zap.Error(err))
	}
	defer pool.Close()

	// Safe to run on every boot: seed.Words is idempotent (ON CONFLICT DO
	// NOTHING on words.word), so a restart against an already-seeded
	// database is a cheap no-op pass rather than a step an operator must
	// remember to run separately.
	if err := seed.Words(ctx, pool, "words.txt"); err != nil {
		logger.Fatal("failed to seed words", zap.Error(err))
	}

	st, err := store.New(pool)
	if err != nil {
		logger.Fatal("failed to build store", zap.Error(err))
	}

	svc, err := service.NewService(st)
	if err != nil {
		logger.Fatal("failed to build service", zap.Error(err))
	}

	handler := buildHandler(svc)

	// app.Serve blocks until ctx is cancelled (SIGINT/SIGTERM) and the
	// server has shut down gracefully. No WithUserAuth here: this service
	// uses maxkit's default service-to-service stub auth (Authorization:
	// Bearer <stub token>), with the end user's identity trusted from the
	// already-authenticated X-User-Id/X-User-Role headers, per maxkit's own
	// Authenticator.Middleware.
	//
	// Empty AllowedOrigins (the default outside docker-compose) means
	// WithCORS is a no-op - CORS headers are only ever sent for an origin
	// explicitly listed in docker/config.toml, i.e. swagger-ui's
	// http://localhost:8081 for browser-based "Try it out".
	app.Serve(ctx, cfg.Infrastructure.HTTPServer, handler,
		app.WithCORS(cfg.Infrastructure.CORS),
	)
}

func buildHandler(svc service.Servicer) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", rest.NewRouter(svc))
	return mux
}
