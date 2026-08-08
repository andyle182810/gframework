// Package i18n resolves user-facing message codes into localized text.
//
// Services raise errors with a stable code rather than a sentence:
//
//	httpserver.ConflictError(err, messages.PhoneNumberInUse)
//
// The HTTP error handler looks the code up in the catalog registered on the
// server and renders it in the locale the caller asked for, so the response
// carries both the code a client can branch on and text a person can read.
package i18n

import (
	"strconv"
	"strings"
)

// Locale is a language tag the catalog carries text for.
type Locale string

const (
	English    Locale = "en"
	Vietnamese Locale = "vi"

	// Default is the locale used when Accept-Language names nothing supported,
	// and the fallback when a catalog entry is missing a translation.
	Default = English
)

// Catalog maps a message code to its text in each locale. A code missing an
// entry for the requested locale falls back to Default.
type Catalog map[string]map[Locale]string

// Merge returns a catalog holding every entry of c overlaid with other. Neither
// receiver nor argument is modified, so a service can extend the builtin
// catalog without mutating it.
func (c Catalog) Merge(other Catalog) Catalog {
	merged := make(Catalog, len(c)+len(other))

	for code, texts := range c {
		merged[code] = texts
	}

	for code, texts := range other {
		merged[code] = texts
	}

	return merged
}

// Lookup returns the text registered for code in locale. The bool reports
// whether code is registered at all, which is what distinguishes a message code
// from a literal sentence.
func (c Catalog) Lookup(code string, locale Locale) (string, bool) {
	texts, ok := c[code]
	if !ok {
		return "", false
	}

	if text, ok := texts[locale]; ok && text != "" {
		return text, true
	}

	if text, ok := texts[Default]; ok && text != "" {
		return text, true
	}

	// Registered but untranslated: the code itself beats an empty message.
	return code, true
}

// Negotiate picks the best supported locale from an Accept-Language header,
// falling back to Default for an empty, malformed, or unsupported header.
func Negotiate(header string) Locale {
	best := Default
	bestQuality := 0.0

	for _, entry := range strings.Split(header, ",") {
		tag, quality := parseLanguageRange(entry)

		locale, ok := supportedLocale(tag)

		// Ties keep the earlier entry, which is the order the client preferred.
		if !ok || quality <= bestQuality {
			continue
		}

		best, bestQuality = locale, quality
	}

	return best
}

// parseLanguageRange splits one Accept-Language entry into its tag and quality.
// A missing or malformed q-value means 1, per RFC 9110.
func parseLanguageRange(entry string) (string, float64) {
	tag, params, found := strings.Cut(entry, ";")

	tag = strings.ToLower(strings.TrimSpace(tag))

	if !found {
		return tag, 1
	}

	for _, param := range strings.Split(params, ";") {
		name, value, ok := strings.Cut(param, "=")
		if !ok || strings.ToLower(strings.TrimSpace(name)) != "q" {
			continue
		}

		quality, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			return tag, 1
		}

		return tag, quality
	}

	return tag, 1
}

// supportedLocale matches a language range against the locales the system
// speaks, so "vi-VN" resolves to Vietnamese. The wildcard defers to Default.
func supportedLocale(tag string) (Locale, bool) {
	primary, _, _ := strings.Cut(tag, "-")

	switch primary {
	case string(English), "*":
		return English, true
	case string(Vietnamese):
		return Vietnamese, true
	default:
		return Default, false
	}
}
