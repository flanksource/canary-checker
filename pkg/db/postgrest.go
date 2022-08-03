package db

import (
	"strconv"

	"github.com/flanksource/commons/deps"
	"github.com/flanksource/commons/logger"
)

var PostgRESTVersion = "v9.0.0"
var PostgRESTServerPort = 3000

func PostgRESTEndpoint() string {
	return "http://localhost:" + strconv.Itoa(PostgRESTServerPort)
}

func GoOffline() error {
	return getBinary()("--help")
}

func getBinary() deps.BinaryFunc {
	return deps.BinaryWithEnv("postgREST", PostgRESTVersion, ".bin", map[string]string{
		"PGRST_DB_URI":                   ConnectionString,
		"PGRST_DB_PORT":                  strconv.Itoa(PostgRESTServerPort),
		"PGRST_DB_SCHEMA":                "public",        // Database is set with default schema. Which is public. See /pkg/db/migrations/
		"PGRST_DB_ANON_ROLE":             "postgrest_api", // See: pkg/db/migrations/4_roles.sql
		"PGRST_OPENAPI_SERVER_PROXY_URI": HTTPEndpoint,
		"PGRST_LOG_LEVEL":                "info", // https://postgrest.org/en/stable/configuration.html?highlight=log_level#log-level
	})
}

func StartPostgrest() {
	if err := getBinary()(""); err != nil {
		logger.Errorf("Failed to start postgREST: %v", err)
	}
}
