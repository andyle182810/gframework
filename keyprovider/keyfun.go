package keyprovider

import (
	"context"
	"net/http"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/rs/zerolog"
	"golang.org/x/time/rate"
)

const (
	JWKSRateLimitBurst  = 5
	JWKSRefreshTimeout  = 10 * time.Second
	JWKSRefreshInterval = 60 * time.Minute
)

func NewKeyFunc(ctx context.Context, log zerolog.Logger, urls []string) (keyfunc.Keyfunc, error) { //nolint:ireturn
	rateLimiter := rate.NewLimiter(rate.Every(time.Second), JWKSRateLimitBurst)

	var httpClient *http.Client

	refreshErrorHandlerFunc := func(url string) func(ctx context.Context, err error) {
		return func(_ context.Context, err error) {
			log.Error().
				Err(err).
				Str("jwks_url", url).
				Msg("The JWKS key refresh has failed in the background goroutine")
		}
	}

	override := keyfunc.Override{
		Client:                  httpClient,
		HTTPTimeout:             JWKSRefreshTimeout,
		RefreshInterval:         JWKSRefreshInterval,
		RefreshUnknownKID:       rateLimiter,
		RefreshErrorHandlerFunc: refreshErrorHandlerFunc,
		RateLimitWaitMax:        0,
		ValidationSkipAll:       false,
	}

	keyFunc, err := keyfunc.NewDefaultOverrideCtx(
		ctx,
		urls,
		override,
	)

	if err == nil {
		log.Info().
			Strs("jwks_urls", urls).
			Msg("The Keyfunc has been initialized successfully and the background refresh has been started")
	}

	return keyFunc, err
}
