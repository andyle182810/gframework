package httpclient

import (
	"maps"
	"net/http"
	"time"

	"github.com/andyle182810/gframework/authtoken"
)

const (
	DefaultTimeout            = 30 * time.Second
	HeaderContentType         = "Content-Type"
	HeaderXRequestID          = "X-Request-ID"
	HeaderAuthorization       = "Authorization"
	ContentTypeJSON           = "application/json"
	ContentTypeFormURLEncoded = "application/x-www-form-urlencoded"
)

type Option func(*Client)

func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		if httpClient, ok := c.httpClient.(*http.Client); ok {
			httpClient.Timeout = timeout
		}
	}
}

func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

func WithRequestIDKey(key any) Option {
	return func(c *Client) {
		c.requestIDKey = key
	}
}

func WithDefaultHeaders(headers map[string]string) Option {
	return func(c *Client) {
		maps.Copy(c.defaultHeaders, headers)
	}
}

type AuthConfig struct {
	ClientID     string
	ClientSecret string
	TokenURL     string
}

func WithAuth(cfg AuthConfig) Option {
	return func(c *Client) {
		c.authConfig = &cfg
		c.tokenProvider = authtoken.New(cfg.TokenURL, cfg.ClientID, cfg.ClientSecret)
	}
}

func WithTokenProvider(provider TokenProvider) Option {
	return func(c *Client) {
		c.tokenProvider = provider
	}
}

func WithMaxResponseSize(size int64) Option {
	return func(c *Client) {
		c.maxResponseSize = size
	}
}

type RequestOption func(*requestConfig)

type requestConfig struct {
	headers   map[string]string
	query     map[string]string
	timeout   time.Duration
	requestID string
}

func WithRequestHeader(key, value string) RequestOption {
	return func(rc *requestConfig) {
		if rc.headers == nil {
			rc.headers = make(map[string]string)
		}

		rc.headers[key] = value
	}
}

func WithRequestTimeout(timeout time.Duration) RequestOption {
	return func(rc *requestConfig) {
		rc.timeout = timeout
	}
}

func WithRequestID(requestID string) RequestOption {
	return func(rc *requestConfig) {
		rc.requestID = requestID
	}
}

func WithQuery(key, value string) RequestOption {
	return func(rc *requestConfig) {
		if rc.query == nil {
			rc.query = make(map[string]string)
		}

		rc.query[key] = value
	}
}

func WithQueryParams(params map[string]string) RequestOption {
	return func(rc *requestConfig) {
		if rc.query == nil {
			rc.query = make(map[string]string)
		}

		maps.Copy(rc.query, params)
	}
}
