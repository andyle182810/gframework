package authtoken

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

var (
	ErrTokenRequestFailed = errors.New("authtoken: token request failed")
	ErrNoAccessToken      = errors.New("authtoken: no access token in response")
)

const (
	DefaultTimeout       = 10 * time.Second
	tokenExpiryBuffer    = 30 * time.Second
	headerContentType    = "Content-Type"
	contentTypeForm      = "application/x-www-form-urlencoded"
	grantTypeCredentials = "client_credentials"
)

//nolint:tagliatelle
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type Client struct {
	tokenURL     string
	clientID     string
	clientSecret string
	httpClient   *http.Client

	mu          sync.RWMutex
	accessToken string
	expiresAt   time.Time
}

func New(tokenURL, clientID, clientSecret string, opts ...Option) *Client {
	c := &Client{
		tokenURL:     tokenURL,
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient: &http.Client{ //nolint:exhaustruct
			Timeout: DefaultTimeout,
		},
		mu:          sync.RWMutex{},
		accessToken: "",
		expiresAt:   time.Time{},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *Client) GetToken(ctx context.Context) (string, error) {
	c.mu.RLock()
	if c.accessToken != "" && time.Now().Before(c.expiresAt) {
		token := c.accessToken
		c.mu.RUnlock()

		return token, nil
	}
	c.mu.RUnlock()

	return c.refreshToken(ctx)
}

func (c *Client) refreshToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock (another goroutine might have refreshed)
	if c.accessToken != "" && time.Now().Before(c.expiresAt) {
		return c.accessToken, nil
	}

	token, expiresIn, err := c.fetchToken(ctx)
	if err != nil {
		return "", err
	}

	c.accessToken = token
	// Refresh before actual expiry to avoid edge cases
	c.expiresAt = time.Now().Add(time.Duration(expiresIn)*time.Second - tokenExpiryBuffer)

	return c.accessToken, nil
}

func (c *Client) fetchToken(ctx context.Context) (string, int, error) {
	data := url.Values{}
	data.Set("grant_type", grantTypeCredentials)
	data.Set("client_id", c.clientID)
	data.Set("client_secret", c.clientSecret)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.tokenURL,
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return "", 0, fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set(headerContentType, contentTypeForm)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("failed to fetch token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("%w: status %d", ErrTokenRequestFailed, resp.StatusCode)
	}

	var tokenResp tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", 0, fmt.Errorf("failed to decode token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return "", 0, ErrNoAccessToken
	}

	return tokenResp.AccessToken, tokenResp.ExpiresIn, nil
}

func (c *Client) InvalidateToken() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.accessToken = ""
	c.expiresAt = time.Time{}
}
