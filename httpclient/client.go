package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

type TokenProvider interface {
	GetToken(ctx context.Context) (string, error)
	InvalidateToken()
}

var _ Doer = (*http.Client)(nil)

type Client struct {
	baseURL         string
	httpClient      Doer
	requestIDKey    any
	defaultHeaders  map[string]string
	authConfig      *AuthConfig
	tokenProvider   TokenProvider
	maxResponseSize int64 // 0 means no limit
}

func New(baseURL string, opts ...Option) *Client {
	c := &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{ //nolint:exhaustruct
			Timeout: DefaultTimeout,
		},
		requestIDKey: nil,
		defaultHeaders: map[string]string{
			HeaderContentType: ContentTypeJSON,
		},
		authConfig:      nil,
		tokenProvider:   nil,
		maxResponseSize: 0,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *Client) Get(
	ctx context.Context,
	path string,
	response any,
	opts ...RequestOption,
) error {
	return c.do(ctx, http.MethodGet, path, nil, response, opts...)
}

func (c *Client) Post(
	ctx context.Context,
	path string,
	body any,
	response any,
	opts ...RequestOption,
) error {
	return c.do(ctx, http.MethodPost, path, body, response, opts...)
}

func (c *Client) Put(
	ctx context.Context,
	path string,
	body any,
	response any,
	opts ...RequestOption,
) error {
	return c.do(ctx, http.MethodPut, path, body, response, opts...)
}

func (c *Client) Patch(
	ctx context.Context,
	path string,
	body any,
	response any,
	opts ...RequestOption,
) error {
	return c.do(ctx, http.MethodPatch, path, body, response, opts...)
}

func (c *Client) Delete(
	ctx context.Context,
	path string,
	response any,
	opts ...RequestOption,
) error {
	return c.do(ctx, http.MethodDelete, path, nil, response, opts...)
}

func (c *Client) Do(
	ctx context.Context,
	method string,
	path string,
	body any,
	response any,
	opts ...RequestOption,
) error {
	return c.do(ctx, method, path, body, response, opts...)
}

func (c *Client) do(
	ctx context.Context,
	method string,
	path string,
	body any,
	response any,
	opts ...RequestOption,
) error {
	cfg := c.buildRequestConfig(ctx, opts...)

	reqCtx := ctx

	if cfg.timeout > 0 {
		var cancel context.CancelFunc
		reqCtx, cancel = context.WithTimeout(ctx, cfg.timeout)
		defer cancel()
	}

	if c.tokenProvider != nil {
		token, err := c.tokenProvider.GetToken(reqCtx)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrAuthFailed, err)
		}

		cfg.headers[HeaderAuthorization] = "Bearer " + token
	}

	req, err := c.buildRequest(reqCtx, method, path, body, cfg)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrRequestFailed, err)
	}
	defer resp.Body.Close()

	return c.handleResponse(resp, response, cfg.requestID)
}

func (c *Client) buildRequestConfig(ctx context.Context, opts ...RequestOption) *requestConfig {
	cfg := &requestConfig{
		headers:   make(map[string]string),
		query:     nil,
		timeout:   0,
		requestID: "",
	}

	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.requestID == "" {
		cfg.requestID = c.extractRequestID(ctx)
	}

	return cfg
}

func (c *Client) extractRequestID(ctx context.Context) string {
	if c.requestIDKey != nil {
		if id, ok := ctx.Value(c.requestIDKey).(string); ok && id != "" {
			return id
		}
	}

	return uuid.New().String()
}

func (c *Client) buildRequest(
	ctx context.Context,
	method string,
	path string,
	body any,
	cfg *requestConfig,
) (*http.Request, error) {
	url := c.buildURL(path, cfg.query)

	var bodyReader io.Reader

	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrEncodeBody, err)
		}

		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCreateRequest, err)
	}

	for k, v := range c.defaultHeaders {
		req.Header.Set(k, v)
	}

	for k, v := range cfg.headers {
		req.Header.Set(k, v)
	}

	if cfg.requestID != "" {
		req.Header.Set(HeaderXRequestID, cfg.requestID)
	}

	return req, nil
}

func (c *Client) handleResponse(resp *http.Response, response any, requestID string) error {
	respRequestID := resp.Header.Get(HeaderXRequestID)
	if respRequestID == "" {
		respRequestID = requestID
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.handleErrorResponse(resp, respRequestID)
	}

	if response == nil {
		return nil
	}

	body := io.Reader(resp.Body)
	if c.maxResponseSize > 0 {
		body = io.LimitReader(resp.Body, c.maxResponseSize+1)
	}

	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrDecodeResponse, err)
	}

	if c.maxResponseSize > 0 && int64(len(bodyBytes)) > c.maxResponseSize {
		return ErrResponseTooLarge
	}

	if err := json.Unmarshal(bodyBytes, response); err != nil {
		return fmt.Errorf("%w: %w", ErrDecodeResponse, err)
	}

	return nil
}

func (c *Client) handleErrorResponse(resp *http.Response, requestID string) error {
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return NewServiceError(resp.StatusCode, "", "", requestID)
	}

	var errResp ErrorResponse
	if err := json.Unmarshal(bodyBytes, &errResp); err == nil && errResp.Message != "" {
		return NewServiceError(resp.StatusCode, errResp.Message, errResp.Internal, requestID)
	}

	return NewServiceError(resp.StatusCode, string(bodyBytes), "", requestID)
}

func (c *Client) BaseURL() string {
	return c.baseURL
}

func (c *Client) buildURL(path string, query map[string]string) string {
	if path != "" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	fullURL := c.baseURL + path

	if len(query) == 0 {
		return fullURL
	}

	params := url.Values{}
	for k, v := range query {
		params.Add(k, v)
	}

	return fullURL + "?" + params.Encode()
}
