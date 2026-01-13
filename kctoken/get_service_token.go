package kctoken

import (
	"context"
	"fmt"
)

type GetServiceTokenOptions struct {
	TenantID   string
	CustomerID string
}

type GetServiceTokenOption func(*GetServiceTokenOptions)

func WithTenantID(tenantID string) GetServiceTokenOption {
	return func(opts *GetServiceTokenOptions) {
		opts.TenantID = tenantID
	}
}

func WithCustomerID(customerID string) GetServiceTokenOption {
	return func(opts *GetServiceTokenOptions) {
		opts.CustomerID = customerID
	}
}

func (client *TokenClient) GetServiceToken(ctx context.Context, opts ...GetServiceTokenOption) (string, error) {
	options := &GetServiceTokenOptions{
		TenantID:   "",
		CustomerID: "",
	}

	for _, opt := range opts {
		opt(options)
	}

	formData := map[string]string{
		"grant_type":    "client_credentials",
		"client_id":     client.clientID,
		"client_secret": client.clientSecret,
	}

	if options.TenantID != "" {
		formData["tenant_id"] = options.TenantID
	}

	if options.CustomerID != "" {
		formData["customer_id"] = options.CustomerID
	}

	var tokenResp TokenResponse

	var tokenErr KeycloakError

	resp, err := client.restyClient.R().
		SetContext(ctx).
		SetFormData(formData).
		SetResult(&tokenResp).
		SetError(&tokenErr).
		Post(client.url)
	if err != nil {
		return "", err
	}

	if !resp.IsSuccess() {
		return "", fmt.Errorf("%w: error=%s, description=%s",
			ErrRequestFailed, tokenErr.Error, tokenErr.ErrorDescription)
	}

	if tokenResp.AccessToken == "" {
		return "", ErrNoAccessToken
	}

	return tokenResp.AccessToken, nil
}
