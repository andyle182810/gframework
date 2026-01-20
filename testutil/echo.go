package testutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/andyle182810/gframework/middleware"
	"github.com/andyle182810/gframework/validator"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

type Options struct {
	Method        string            // HTTP method (GET, POST, etc.)
	Path          string            // Request path
	Body          []byte            // Request body
	Headers       map[string]string // Custom headers
	QueryParams   map[string]string // Query parameters
	PathParams    map[string]string // Path parameters (e.g., :id)
	ContentType   string            // Content-Type header (defaults to application/json)
	SkipRequestID bool              // Skip auto-generating X-Request-ID header
}

func SetupEchoContext(
	t *testing.T,
	opts *Options,
) (*echo.Context, *httptest.ResponseRecorder, *http.Request) {
	t.Helper()

	iecho := echo.New()
	iecho.Validator = validator.DefaultRestValidator()
	iecho.HTTPErrorHandler = middleware.ErrorHandler(iecho.HTTPErrorHandler)

	requestPath := opts.Path

	if len(opts.QueryParams) > 0 {
		query := url.Values{}
		for key, value := range opts.QueryParams {
			query.Add(key, value)
		}

		requestPath = fmt.Sprintf("%s?%s", opts.Path, query.Encode())
	}

	req := httptest.NewRequest(opts.Method, requestPath, bytes.NewBuffer(opts.Body))

	if !opts.SkipRequestID {
		requestID := uuid.New().String()
		req.Header.Set(middleware.HeaderXRequestID, requestID)
	}

	contentType := opts.ContentType
	if contentType == "" {
		contentType = "application/json"
	}

	req.Header.Set("Content-Type", contentType)

	for key, value := range opts.Headers {
		req.Header.Set(key, value)
	}

	rec := httptest.NewRecorder()
	ctx := iecho.NewContext(req, rec)

	if len(opts.PathParams) > 0 {
		pathValues := make([]echo.PathValue, 0, len(opts.PathParams))

		for name, value := range opts.PathParams {
			pathValues = append(pathValues, echo.PathValue{
				Name:  name,
				Value: value,
			})
		}

		ctx.SetPathValues(pathValues)
	}

	return ctx, rec, req
}

func SetupEchoContextWithAuth(
	t *testing.T,
	opts *Options,
	token string,
) (*echo.Context, *httptest.ResponseRecorder, *http.Request) {
	t.Helper()

	if opts.Headers == nil {
		opts.Headers = make(map[string]string)
	}

	opts.Headers["Authorization"] = token

	return SetupEchoContext(t, opts)
}

func SetupEchoContextWithJSON(
	t *testing.T,
	method string,
	path string,
	body interface{},
) (*echo.Context, *httptest.ResponseRecorder, *http.Request) {
	t.Helper()

	var bodyBytes []byte

	if body != nil {
		var err error

		bodyBytes, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("Failed to marshal JSON body: %v", err)
		}
	}

	return SetupEchoContext(t, &Options{
		Method:        method,
		Path:          path,
		Body:          bodyBytes,
		Headers:       nil,
		QueryParams:   nil,
		PathParams:    nil,
		ContentType:   "",
		SkipRequestID: false,
	})
}
