// Package authtoken provides Keycloak service-account token management with automatic caching and refresh.
//
// The client uses an RWMutex with double-check locking to efficiently cache tokens across concurrent requests,
// automatically refreshing expired tokens. A configurable expiry buffer prevents race conditions during token refresh.
//
// Basic usage:
//
//	client := authtoken.New("https://auth.example.com", "my-realm", "client-id", "client-secret")
//
//	token, err := client.GetToken(ctx)
//	if err != nil {
//	    return err
//	}
//	// Use token for API requests
//
// The client is safe for concurrent use. Call InvalidateToken() to force a token refresh on the next request.
package authtoken

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Nerzal/gocloak/v13"
)

var ErrNoAccessToken = errors.New("authtoken: no access token in response")

const tokenExpiryBuffer = 30 * time.Second

type Client struct {
	gocloak      *gocloak.GoCloak
	realm        string
	clientID     string
	clientSecret string
	expiryBuffer time.Duration
	mu           sync.RWMutex
	accessToken  string
	expiresAt    time.Time
}

func New(baseURL, realm, clientID, clientSecret string, opts ...Option) *Client {
	client := &Client{ //nolint:exhaustruct
		gocloak:      gocloak.NewClient(baseURL),
		realm:        realm,
		clientID:     clientID,
		clientSecret: clientSecret,
		expiryBuffer: tokenExpiryBuffer,
		mu:           sync.RWMutex{},
	}

	for _, opt := range opts {
		if opt != nil {
			opt(client)
		}
	}

	return client
}

func (c *Client) GetToken(ctx context.Context) (string, error) {
	c.mu.RLock()
	if c.accessToken != "" && time.Now().Before(c.expiresAt) {
		token := c.accessToken
		c.mu.RUnlock()

		return token, nil
	}
	c.mu.RUnlock()

	return c.getToken(ctx)
}

func (c *Client) getToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.accessToken != "" && time.Now().Before(c.expiresAt) {
		return c.accessToken, nil
	}

	jwt, err := c.gocloak.LoginClient(ctx, c.clientID, c.clientSecret, c.realm)
	if err != nil {
		return "", fmt.Errorf("failed to fetch token: %w", err)
	}

	if jwt == nil || jwt.AccessToken == "" {
		return "", ErrNoAccessToken
	}

	expiresIn := time.Duration(jwt.ExpiresIn) * time.Second

	c.accessToken = jwt.AccessToken
	c.expiresAt = time.Now().Add(expiresIn - c.expiryBuffer)

	return c.accessToken, nil
}

func (c *Client) InvalidateToken() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.accessToken = ""
	c.expiresAt = time.Time{}
}
