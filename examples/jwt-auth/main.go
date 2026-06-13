// JWT authentication with gframework: JWKS key fetching, hardened JWT
// validation (algorithm pinning + issuer/audience checks), realm-role
// authorization, and internal service-to-service auth.
//
// Requires a running Keycloak (see docker-compose.yml):
//
//	docker compose up -d
//	go run .
//
//	# Obtain a token from Keycloak, then:
//	curl -H "Authorization: Bearer $TOKEN" http://localhost:8081/api/me
package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/andyle182810/gframework/httpserver"
	"github.com/andyle182810/gframework/jwks"
	"github.com/andyle182810/gframework/middleware"
	"github.com/andyle182810/gframework/runner"
	"github.com/labstack/echo/v5"
	"github.com/rs/zerolog/log"
)

const (
	// In production, read these from configuration.
	keycloakBase = "http://localhost:8080"
	realm        = "demo"

	issuer   = keycloakBase + "/realms/" + realm
	jwksURL  = issuer + "/protocol/openid-connect/certs"
	audience = "demo-api"
)

func main() {
	ctx := context.Background()

	// jwks.New fetches the realm's signing keys and refreshes them in the
	// background; unknown key IDs trigger a rate-limited immediate refresh,
	// so key rotation works without restarts.
	keyfunc, err := jwks.New(ctx, []string{jwksURL})
	if err != nil {
		log.Error().Err(err).Msg("failed to initialize JWKS (is Keycloak running?)")
		os.Exit(1)
	}

	server := httpserver.New(&httpserver.Config{ //nolint:exhaustruct
		Host:        "0.0.0.0",
		Port:        8081,
		GracePeriod: 10 * time.Second,
	})

	// Harden the JWT middleware: pin the issuer and audience so tokens minted
	// by other realms — or for other services in this realm — are rejected.
	// Algorithm pinning (asymmetric only) is on by default.
	jwtConfig := middleware.DefaultJWTConfig()
	jwtConfig.Keyfunc = keyfunc
	jwtConfig.Issuer = issuer
	jwtConfig.Audiences = []string{audience}

	api := server.Root.Group("/api", middleware.JWTWithConfig(jwtConfig))

	// Any authenticated user.
	api.GET("/me", func(c *echo.Context) error {
		claims, err := middleware.GetExtendedClaimsFromContext(c)
		if err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "no claims")
		}

		return c.JSON(http.StatusOK, map[string]any{
			"subject":  claims.Subject,
			"username": claims.PreferredUsername,
			"email":    claims.Email,
			"roles":    claims.GetRealmRoles(),
		})
	})

	// Realm-role guard: only users with the "admin" realm role get through.
	admin := api.Group("/admin", middleware.RequireAnyRealmRole("admin"))
	admin.GET("/stats", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"secret": "admin-only data"})
	})

	// Internal service-to-service endpoint: callers authenticate with a
	// service-account token in the X-Internal-Authorization header and must be
	// in the allow-list (matched against the token's azp claim). On the calling
	// side, use httpclient with WithTokenProvider + WithInternalAuthHeader.
	internalConfig := middleware.DefaultInternalAuthConfig()
	internalConfig.Keyfunc = keyfunc
	internalConfig.Issuer = issuer
	internalConfig.AllowedClients = []string{"billing-service", "report-service"}

	internal := server.Root.Group("/internal", middleware.InternalAuthWithConfig(internalConfig))
	internal.POST("/sync", func(c *echo.Context) error {
		return c.JSON(http.StatusAccepted, map[string]string{"status": "sync started"})
	})

	runner.New(
		runner.WithCoreService(server),
	).Run()
}
