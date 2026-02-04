package jwks

import (
	"net/http"
	"time"
)

func WithHTTPClient(client *http.Client) Option {
	return func(cfg *Config) {
		cfg.httpClient = client
	}
}

func WithRateLimitBurst(burst int) Option {
	return func(cfg *Config) {
		if burst > 0 {
			cfg.rateLimitBurst = burst
		}
	}
}

func WithRefreshTimeout(timeout time.Duration) Option {
	return func(cfg *Config) {
		if timeout > 0 {
			cfg.refreshTimeout = timeout
		}
	}
}

func WithRefreshInterval(interval time.Duration) Option {
	return func(cfg *Config) {
		if interval > 0 {
			cfg.refreshInterval = interval
		}
	}
}

func WithRateLimitWaitMax(maxWait time.Duration) Option {
	return func(cfg *Config) {
		cfg.rateLimitWaitMax = maxWait
	}
}

func WithValidationSkipAll(skip bool) Option {
	return func(cfg *Config) {
		cfg.validationSkipAll = skip
	}
}
