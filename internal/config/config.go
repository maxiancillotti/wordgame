package config

// ServiceName identifies this service to app.Init and tags every metric
// registered against its Prometheus registry.
const ServiceName = "wordgame"

// ServiceConfig is wordgame's own configuration on top of maxkit's standard
// infrastructure config (HTTP server, PostgreSQL, tracing). The game has no
// settings of its own beyond that.
type ServiceConfig struct{}
