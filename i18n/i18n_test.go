package i18n_test

import (
	"net/http"
	"testing"

	"github.com/andyle182810/gframework/i18n"
	"github.com/stretchr/testify/require"
)

const (
	codeTaken   = "PHONE_NUMBER_IN_USE"
	codeEmpty   = "REGISTERED_BUT_EMPTY"
	textTakenEN = "Phone number is already used"
	textTakenVI = "Số điện thoại đã được sử dụng"
)

func testCatalog() i18n.Catalog {
	return i18n.Catalog{
		codeTaken: {
			i18n.English:    textTakenEN,
			i18n.Vietnamese: textTakenVI,
		},
		"ENGLISH_ONLY": {
			i18n.English: "English only",
		},
		codeEmpty: {},
	}
}

func TestNegotiate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header string
		want   i18n.Locale
	}{
		{name: "empty header falls back", header: "", want: i18n.English},
		{name: "exact tag", header: "vi", want: i18n.Vietnamese},
		{name: "region subtag", header: "vi-VN", want: i18n.Vietnamese},
		{name: "case insensitive", header: "VI-vn", want: i18n.Vietnamese},
		{name: "surrounding spaces", header: "  vi  ", want: i18n.Vietnamese},
		{name: "unsupported language", header: "ja-JP", want: i18n.English},
		{name: "wildcard", header: "*", want: i18n.English},
		{name: "browser list prefers highest q", header: "vi-VN,vi;q=0.9,en;q=0.8", want: i18n.Vietnamese},
		{name: "lower q loses", header: "vi;q=0.2,en;q=0.8", want: i18n.English},
		{name: "equal q keeps client order", header: "vi;q=0.8,en;q=0.8", want: i18n.Vietnamese},
		{name: "unsupported first is skipped", header: "ja,vi;q=0.5", want: i18n.Vietnamese},
		{name: "q=0 means not acceptable", header: "vi;q=0", want: i18n.English},
		{name: "malformed q is treated as 1", header: "vi;q=abc", want: i18n.Vietnamese},
		{name: "malformed header", header: ";;;", want: i18n.English},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, i18n.Negotiate(tt.header))
		})
	}
}

func TestCatalogLookup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		code           string
		locale         i18n.Locale
		want           string
		wantRegistered bool
	}{
		{
			name:           "translated",
			code:           codeTaken,
			locale:         i18n.Vietnamese,
			want:           textTakenVI,
			wantRegistered: true,
		},
		{
			name:           "default locale",
			code:           codeTaken,
			locale:         i18n.English,
			want:           textTakenEN,
			wantRegistered: true,
		},
		{
			name:           "missing translation falls back to default",
			code:           "ENGLISH_ONLY",
			locale:         i18n.Vietnamese,
			want:           "English only",
			wantRegistered: true,
		},
		{
			name:           "registered without text yields the code",
			code:           codeEmpty,
			locale:         i18n.Vietnamese,
			want:           codeEmpty,
			wantRegistered: true,
		},
		{
			name:           "prose is not a code",
			code:           "Phone number is already used",
			locale:         i18n.English,
			want:           "",
			wantRegistered: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			text, registered := testCatalog().Lookup(tt.code, tt.locale)
			require.Equal(t, tt.wantRegistered, registered)
			require.Equal(t, tt.want, text)
		})
	}
}

func TestCatalogMerge(t *testing.T) {
	t.Parallel()

	base := i18n.Catalog{
		codeTaken: {i18n.English: textTakenEN},
		"SHARED":  {i18n.English: "base"},
	}
	overlay := i18n.Catalog{
		"SHARED": {i18n.English: "overlay"},
		"EXTRA":  {i18n.English: "extra"},
	}

	merged := base.Merge(overlay)

	shared, _ := merged.Lookup("SHARED", i18n.English)
	require.Equal(t, "overlay", shared)

	extra, _ := merged.Lookup("EXTRA", i18n.English)
	require.Equal(t, "extra", extra)

	kept, _ := merged.Lookup(codeTaken, i18n.English)
	require.Equal(t, textTakenEN, kept)

	// Merging must not mutate either side: the server merges the builtin
	// catalog on every construction.
	original, _ := base.Lookup("SHARED", i18n.English)
	require.Equal(t, "base", original)
	require.Len(t, overlay, 2)
}

func TestBuiltinCatalogIsFullyTranslated(t *testing.T) {
	t.Parallel()

	for code, texts := range i18n.BuiltinCatalog() {
		require.NotEmpty(t, texts[i18n.English], "%s is missing English text", code)
		require.NotEmpty(t, texts[i18n.Vietnamese], "%s is missing Vietnamese text", code)
	}
}

func TestCodeForStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status int
		want   string
	}{
		{name: "bad request", status: http.StatusBadRequest, want: i18n.CodeBadRequest},
		{name: "unauthorized", status: http.StatusUnauthorized, want: i18n.CodeUnauthorized},
		{name: "forbidden", status: http.StatusForbidden, want: i18n.CodeForbidden},
		{name: "not found", status: http.StatusNotFound, want: i18n.CodeNotFound},
		{name: "conflict", status: http.StatusConflict, want: i18n.CodeConflict},
		{name: "unprocessable", status: http.StatusUnprocessableEntity, want: i18n.CodeValidation},
		{name: "bad gateway", status: http.StatusBadGateway, want: i18n.CodeServiceUnavailable},
		{name: "service unavailable", status: http.StatusServiceUnavailable, want: i18n.CodeServiceUnavailable},
		{name: "gateway timeout", status: http.StatusGatewayTimeout, want: i18n.CodeServiceUnavailable},
		{name: "other server error", status: http.StatusInternalServerError, want: i18n.CodeInternal},
		{name: "other client error", status: http.StatusTeapot, want: i18n.CodeBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, i18n.CodeForStatus(tt.status))
		})
	}
}
