package jwks

import (
	"context"
	"net/http"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
)

const (
	DefaultRateLimitBurst  = 5
	DefaultRefreshTimeout  = 10 * time.Second
	DefaultRefreshInterval = 60 * time.Minute
)

type KeyFunc struct {
	keyfunc keyfunc.Keyfunc
}

type Config struct {
	urls              []string
	httpClient        *http.Client
	rateLimitBurst    int
	refreshTimeout    time.Duration
	refreshInterval   time.Duration
	rateLimitWaitMax  time.Duration
	validationSkipAll bool
}

type Option func(*Config)

func New(ctx context.Context, urls []string, opts ...Option) (*KeyFunc, error) {
	cfg := &Config{
		urls:              urls,
		httpClient:        nil,
		rateLimitBurst:    DefaultRateLimitBurst,
		refreshTimeout:    DefaultRefreshTimeout,
		refreshInterval:   DefaultRefreshInterval,
		rateLimitWaitMax:  0,
		validationSkipAll: false,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	rateLimiter := rate.NewLimiter(rate.Every(time.Second), cfg.rateLimitBurst)

	override := keyfunc.Override{
		Client:            cfg.httpClient,
		HTTPTimeout:       cfg.refreshTimeout,
		RefreshInterval:   cfg.refreshInterval,
		RefreshUnknownKID: rateLimiter,
		RefreshErrorHandlerFunc: func(url string) func(ctx context.Context, err error) {
			return func(_ context.Context, err error) {
				log.Error().
					Err(err).
					Str("jwks_url", url).
					Msg("JWKS key refresh failed")
			}
		},
		RateLimitWaitMax:  cfg.rateLimitWaitMax,
		ValidationSkipAll: cfg.validationSkipAll,
	}

	keyFunc, err := keyfunc.NewDefaultOverrideCtx(ctx, urls, override)
	if err != nil {
		log.Error().
			Err(err).
			Strs("jwks_urls", urls).
			Msg("Failed to initialize JWKS keyfunc")

		return nil, err
	}

	log.Info().
		Strs("jwks_urls", urls).
		Dur("refresh_interval", cfg.refreshInterval).
		Dur("refresh_timeout", cfg.refreshTimeout).
		Msg("JWKS keyfunc initialized successfully")

	return &KeyFunc{keyfunc: keyFunc}, nil
}

func (k *KeyFunc) Keyfunc(token *jwt.Token) (any, error) {
	return k.keyfunc.Keyfunc(token)
}

func (k *KeyFunc) KeyfuncCtx(ctx context.Context) jwt.Keyfunc {
	return k.keyfunc.KeyfuncCtx(ctx)
}

func (k *KeyFunc) Storage() jwkset.Storage { //nolint:ireturn
	return k.keyfunc.Storage()
}

func (k *KeyFunc) VerificationKeySet(ctx context.Context) (jwt.VerificationKeySet, error) {
	return k.keyfunc.VerificationKeySet(ctx)
}
