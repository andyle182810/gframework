package keyprovider

import (
	"context"
	"net/http"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/andyle182810/gframework/notifylog"
	"golang.org/x/time/rate"
)

const (
	// JWKSRateLimitBurst allows 5 new unknown KID refreshes per second.
	JWKSRateLimitBurst  = 5
	JWKSRefreshTimeout  = 10 * time.Second
	JWKSRefreshInterval = 60 * time.Minute
)

func NewKeyFunc(ctx context.Context, log notifylog.NotifyLog, urls []string) (keyfunc.Keyfunc, error) {
	rateLimiter := rate.NewLimiter(rate.Every(time.Second), JWKSRateLimitBurst)

	var httpClient *http.Client

	refreshErrorHandlerFunc := func(url string) func(ctx context.Context, err error) {
		return func(_ context.Context, err error) {
			log.Error().
				Err(err).
				Str("jwks_url", url).
				Msg("JWKS key refresh failed in background goroutine")
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

	kf, err := keyfunc.NewDefaultOverrideCtx(
		ctx,
		urls,
		override,
	)

	if err == nil {
		log.Info().
			Strs("urls", urls).
			Msg("Keyfunc initialized, background refresh started")
	}

	return kf, err
}
