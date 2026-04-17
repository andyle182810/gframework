package keycloak

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	grantTypeUMATicket   = "urn:ietf:params:oauth:grant-type:uma-ticket"
	responseModeDecision = "decision"

	headerContentType = "Content-Type"
	headerAuth        = "Authorization"
	contentTypeForm   = "application/x-www-form-urlencoded"
)

var (
	ErrUMARequestFailed   = errors.New("keycloak uma: request failed")
	ErrUMAResponseInvalid = errors.New("keycloak uma: unexpected response")
)

type UMAClient struct {
	tokenEndpoint string
	audience      string
	httpClient    *http.Client
}

func NewUMAClient(
	tokenEndpoint string,
	audience string,
	opts ...UMAOption,
) *UMAClient {
	c := &UMAClient{
		tokenEndpoint: tokenEndpoint,
		audience:      audience,
		httpClient:    http.DefaultClient,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *UMAClient) Check(ctx context.Context, userToken, resource, scope string) (bool, error) {
	if userToken == "" || resource == "" || scope == "" {
		return false, fmt.Errorf("%w: Check requires userToken, resource and scope", ErrInvalidInput)
	}

	permission := resource + "#" + scope

	form := url.Values{}
	form.Set("grant_type", grantTypeUMATicket)
	form.Set("audience", c.audience)
	form.Set("permission", permission)
	form.Set("response_mode", responseModeDecision)

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.tokenEndpoint, strings.NewReader(form.Encode()),
	)
	if err != nil {
		return false, fmt.Errorf("%w: build request: %w", ErrUMARequestFailed, err)
	}

	req.Header.Set(headerContentType, contentTypeForm)
	req.Header.Set(headerAuth, "Bearer "+userToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrUMARequestFailed, err)
	}

	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil

	case http.StatusForbidden:
		return false, nil

	default:
		return false, unexpectedStatus(resp)
	}
}

func unexpectedStatus(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("%w: status=%d body_read_error=%w",
			ErrUMAResponseInvalid, resp.StatusCode, err)
	}

	return fmt.Errorf("%w: status=%d body=%s",
		ErrUMAResponseInvalid, resp.StatusCode, strings.TrimSpace(string(body)))
}
