package httpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	tokenRequestTimeout = 10 * time.Second
)

//nolint:tagliatelle
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type tokenProvider struct {
	config     AuthConfig
	httpClient *http.Client

	mu          sync.RWMutex
	accessToken string
	expiresAt   time.Time
}

func newTokenProvider(config AuthConfig) *tokenProvider {
	return &tokenProvider{
		config: config,
		httpClient: &http.Client{ //nolint:exhaustruct
			Timeout: tokenRequestTimeout,
		},
		mu:          sync.RWMutex{},
		accessToken: "",
		expiresAt:   time.Time{},
	}
}

func (p *tokenProvider) GetToken(ctx context.Context) (string, error) {
	p.mu.RLock()
	if p.accessToken != "" && time.Now().Before(p.expiresAt) {
		token := p.accessToken
		p.mu.RUnlock()

		return token, nil
	}
	p.mu.RUnlock()

	return p.refreshToken(ctx)
}

func (p *tokenProvider) refreshToken(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check after acquiring write lock (another goroutine might have refreshed)
	if p.accessToken != "" && time.Now().Before(p.expiresAt) {
		return p.accessToken, nil
	}

	token, expiresIn, err := p.fetchToken(ctx)
	if err != nil {
		return "", err
	}

	p.accessToken = token
	// Refresh 30 seconds before actual expiry to avoid edge cases
	p.expiresAt = time.Now().Add(time.Duration(expiresIn)*time.Second - 30*time.Second)

	return p.accessToken, nil
}

func (p *tokenProvider) fetchToken(ctx context.Context) (string, int, error) {
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", p.config.ClientID)
	data.Set("client_secret", p.config.ClientSecret)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		p.config.TokenURL,
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return "", 0, fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set(HeaderContentType, ContentTypeFormURLEncoded)

	resp, err := p.httpClient.Do(req)
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

	return tokenResp.AccessToken, tokenResp.ExpiresIn, nil
}

func (p *tokenProvider) InvalidateToken() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.accessToken = ""
	p.expiresAt = time.Time{}
}
