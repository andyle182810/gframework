package testutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func AssertJSONResponse(t *testing.T, rec *httptest.ResponseRecorder, target any) {
	t.Helper()

	require.Equal(t, "application/json", rec.Header().Get("Content-Type"),
		"Response Content-Type should be application/json")

	err := json.Unmarshal(rec.Body.Bytes(), target)
	require.NoError(t, err, "Response body should be valid JSON")
}

func AssertStatusCode(t *testing.T, rec *httptest.ResponseRecorder, expectedStatus int) {
	t.Helper()
	assert.Equal(t, expectedStatus, rec.Code, "Response status code mismatch")
}

func AssertHeader(t *testing.T, rec *httptest.ResponseRecorder, header, expectedValue string) {
	t.Helper()
	assert.Equal(t, expectedValue, rec.Header().Get(header), "Header %s mismatch", header)
}

func AssertHeaderExists(t *testing.T, rec *httptest.ResponseRecorder, header string) {
	t.Helper()
	assert.NotEmpty(t, rec.Header().Get(header), "Header %s should exist", header)
}

func AssertResponseBody(t *testing.T, rec *httptest.ResponseRecorder, expected string) {
	t.Helper()
	assert.Equal(t, expected, rec.Body.String(), "Response body mismatch")
}

func AssertResponseContains(t *testing.T, rec *httptest.ResponseRecorder, substring string) {
	t.Helper()
	assert.Contains(t, rec.Body.String(), substring, "Response body should contain substring")
}

func ParseJSONResponse(rec *httptest.ResponseRecorder, target interface{}) error {
	return json.Unmarshal(rec.Body.Bytes(), target)
}

func MustParseJSONResponse(t *testing.T, rec *httptest.ResponseRecorder, target interface{}) {
	t.Helper()

	err := json.Unmarshal(rec.Body.Bytes(), target)

	require.NoError(t, err, "Failed to parse JSON response")
}

func AssertErrorResponse(t *testing.T, rec *httptest.ResponseRecorder, expectedStatus int, expectedMessageSubstring string) {
	t.Helper()

	AssertStatusCode(t, rec, expectedStatus)

	if expectedMessageSubstring != "" {
		AssertResponseContains(t, rec, expectedMessageSubstring)
	}
}

func AssertSuccessResponse(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	assert.GreaterOrEqual(t, rec.Code, http.StatusOK, "Response should be successful")
	assert.Less(t, rec.Code, http.StatusMultipleChoices, "Response should be successful")
}
