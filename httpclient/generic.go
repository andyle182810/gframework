//nolint:ireturn
package httpclient

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

func GetJSON[T any](ctx context.Context, c *Client, path string, opts ...RequestOption) (T, error) {
	var result T

	err := c.Get(ctx, path, &result, opts...)

	return result, err
}

func PostJSON[T any](ctx context.Context, c *Client, path string, body any, opts ...RequestOption) (T, error) {
	var result T

	err := c.Post(ctx, path, body, &result, opts...)

	return result, err
}

func PutJSON[T any](ctx context.Context, c *Client, path string, body any, opts ...RequestOption) (T, error) {
	var result T

	err := c.Put(ctx, path, body, &result, opts...)

	return result, err
}

func PatchJSON[T any](ctx context.Context, c *Client, path string, body any, opts ...RequestOption) (T, error) {
	var result T

	err := c.Patch(ctx, path, body, &result, opts...)

	return result, err
}

func DeleteJSON[T any](ctx context.Context, c *Client, path string, opts ...RequestOption) (T, error) {
	var result T

	err := c.Delete(ctx, path, &result, opts...)

	return result, err
}

func DoJSON[T any](ctx context.Context, c *Client, method, path string, body any, opts ...RequestOption) (T, error) {
	var result T

	err := c.Do(ctx, method, path, body, &result, opts...)

	return result, err
}

func (c *Client) Head(ctx context.Context, path string, opts ...RequestOption) (*Response, error) {
	return c.doHead(ctx, http.MethodHead, path, opts...)
}

func (c *Client) Options(ctx context.Context, path string, opts ...RequestOption) (*Response, error) {
	return c.doHead(ctx, http.MethodOptions, path, opts...)
}

func (c *Client) doHead(ctx context.Context, method, path string, opts ...RequestOption) (*Response, error) {
	startedAt := time.Now()
	metricsPath := pathLabel(path)
	upstream := upstreamLabel(c.baseURL)
	cfg := c.buildRequestConfig(ctx, opts...)

	if cfg.metricsPath != "" {
		metricsPath = cfg.metricsPath
	}

	if cfg.timeout > 0 {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, cfg.timeout)
		defer cancel()
	}

	if c.tokenProvider != nil {
		token, err := c.tokenProvider.GetToken(ctx)
		if err != nil {
			err = fmt.Errorf("%w: %w", ErrAuthFailed, err)
			c.recordRequest(upstream, method, metricsPath, 0, time.Since(startedAt), err)

			return nil, err
		}

		cfg.headers[HeaderAuthorization] = "Bearer " + token
		cfg.headers[HeaderXInternalAuthorization] = "Bearer " + token
	}

	req, err := c.buildRequest(ctx, method, path, nil, cfg)
	if err != nil {
		c.recordRequest(upstream, method, metricsPath, 0, time.Since(startedAt), err)

		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		err = fmt.Errorf("%w: %w", ErrRequestFailed, err)
		c.recordRequest(upstream, method, metricsPath, 0, time.Since(startedAt), err)

		return nil, err
	}
	defer resp.Body.Close()

	respRequestID := responseRequestID(resp, cfg.requestID)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err = c.handleErrorResponse(resp, respRequestID)
		c.recordRequest(upstream, method, metricsPath, resp.StatusCode, time.Since(startedAt), err)

		return nil, err
	}

	headers := make(map[string]string)

	for k := range resp.Header {
		headers[k] = resp.Header.Get(k)
	}

	c.recordRequest(upstream, method, metricsPath, resp.StatusCode, time.Since(startedAt), nil)

	return &Response{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		RequestID:  respRequestID,
	}, nil
}

func responseRequestID(resp *http.Response, fallback string) string {
	requestID := resp.Header.Get(HeaderXRequestID)
	if requestID == "" {
		return fallback
	}

	return requestID
}
