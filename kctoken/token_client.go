package kctoken

import (
	"errors"
	"time"

	"resty.dev/v3"
)

const (
	defaultTimeout = 10 * time.Second
)

var (
	ErrRequestFailed = errors.New("HTTP request failed")
	ErrNoAccessToken = errors.New("no access token in response")
)

type TokenClient struct {
	url          string
	clientID     string
	clientSecret string
	restyClient  *resty.Client
}

type TokenClienttOption func(*TokenClient)

func NewTokenClient(url, clientID, clientSecret string, opts ...TokenClienttOption) *TokenClient {
	client := &TokenClient{
		url:          url,
		clientID:     clientID,
		clientSecret: clientSecret,
		restyClient:  createDefaultRestyClient(),
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

func WithRestyClient(restyClient *resty.Client) TokenClienttOption {
	return func(c *TokenClient) {
		if restyClient != nil {
			c.restyClient = restyClient
		}
	}
}

func WithTimeout(timeout time.Duration) TokenClienttOption {
	return func(c *TokenClient) {
		c.restyClient.SetTimeout(timeout)
	}
}

//nolint:tagliatelle // Keycloak API returns snake_case
type TokenResponse struct {
	AccessToken      string `json:"access_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
	TokenType        string `json:"token_type"`
	NotBeforePolicy  int    `json:"not-before-policy"`
	Scope            string `json:"scope"`
}

func createDefaultRestyClient() *resty.Client {
	return resty.New().
		SetTimeout(defaultTimeout).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetHeader("Accept", "application/json")
}
