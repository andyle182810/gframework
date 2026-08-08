package i18n

import (
	"strings"
	"unicode"
)

// Code prefixes for the two catalog entries a field failure is rendered from:
// the template for the rule that failed, and the label for the field it failed
// on.
const (
	validationCodePrefix = "VALIDATION_"
	fieldCodePrefix      = "FIELD_"
)

// CodeValidationInvalid is the template used for a validate tag with no
// translation of its own, so an unknown rule still reads as a sentence.
const CodeValidationInvalid = validationCodePrefix + "INVALID"

// Placeholders a validation template may carry. Both are always substituted;
// param is empty for rules that take no argument.
const (
	placeholderField = "{field}"
	placeholderParam = "{param}"
)

// ValidationCode returns the catalog code for a validate tag, so `required`
// resolves to VALIDATION_REQUIRED.
func ValidationCode(tag string) string {
	return validationCodePrefix + strings.ToUpper(tag)
}

// FieldCode returns the catalog code for a request field, so `phoneNumber`
// resolves to FIELD_PHONE_NUMBER. The name is the one the client sent — the
// json, query, or param tag — not the Go field name.
func FieldCode(field string) string {
	var builder strings.Builder

	// Worst case every character is a word boundary and gains a separator.
	builder.Grow(len(fieldCodePrefix) + 2*len(field))
	builder.WriteString(fieldCodePrefix)

	for index, char := range field {
		switch {
		case unicode.IsUpper(char):
			if index > 0 {
				builder.WriteRune('_')
			}

			builder.WriteRune(char)
		case unicode.IsLetter(char) || unicode.IsDigit(char):
			builder.WriteRune(unicode.ToUpper(char))
		default:
			builder.WriteRune('_')
		}
	}

	return builder.String()
}

// ValidationMessage renders one field failure in locale. The template comes
// from the tag and the field name from the catalog, so a service localizes its
// request fields by registering labels rather than by touching a handler.
//
// A field with no registered label falls back to the name the client sent,
// which keeps the message useful while a service's labels are still landing.
func ValidationMessage(catalog Catalog, locale Locale, field, tag, param string) string {
	label, found := catalog.Lookup(FieldCode(field), locale)
	if !found {
		label = field
	}

	text, found := catalog.Lookup(ValidationCode(tag), locale)
	if !found {
		// An unrecognized tag, or a catalog that never merged the builtins.
		text, _ = validationCatalog().Lookup(CodeValidationInvalid, locale)
	}

	text = strings.ReplaceAll(text, placeholderField, label)

	return strings.ReplaceAll(text, placeholderParam, param)
}

// validationCatalog returns a template per validate tag the framework's
// validator can report. Tags absent here render as CodeValidationInvalid.
//
//nolint:funlen // A flat translation table; splitting it only hides entries.
func validationCatalog() Catalog {
	return Catalog{
		CodeValidationInvalid: {
			English:    placeholderField + " is invalid",
			Vietnamese: placeholderField + " không hợp lệ",
		},
		ValidationCode("required"): {
			English:    placeholderField + " is required",
			Vietnamese: "Vui lòng nhập " + placeholderField,
		},
		ValidationCode("required_with"): {
			English:    placeholderField + " is required",
			Vietnamese: "Vui lòng nhập " + placeholderField,
		},
		ValidationCode("email"): {
			English:    placeholderField + " must be a valid email address",
			Vietnamese: placeholderField + " phải là địa chỉ email hợp lệ",
		},
		ValidationCode("url"): {
			English:    placeholderField + " must be a valid URL",
			Vietnamese: placeholderField + " phải là đường dẫn hợp lệ",
		},
		ValidationCode("uri"): {
			English:    placeholderField + " must be a valid URI",
			Vietnamese: placeholderField + " phải là URI hợp lệ",
		},
		ValidationCode("uuid"): {
			English:    placeholderField + " must be a valid UUID",
			Vietnamese: placeholderField + " phải là mã định danh hợp lệ",
		},
		ValidationCode("alphanum"): {
			English:    placeholderField + " must contain only alphanumeric characters",
			Vietnamese: placeholderField + " chỉ được chứa chữ và số",
		},
		ValidationCode("numeric"): {
			English:    placeholderField + " must be numeric",
			Vietnamese: placeholderField + " phải là số",
		},
		ValidationCode("datetime"): {
			English:    placeholderField + " must be a valid date",
			Vietnamese: placeholderField + " phải là ngày hợp lệ",
		},
		ValidationCode("regexp"): {
			English:    placeholderField + " must match the required pattern",
			Vietnamese: placeholderField + " không đúng định dạng",
		},
		ValidationCode("min"): {
			English:    placeholderField + " must be at least " + placeholderParam,
			Vietnamese: placeholderField + " phải tối thiểu " + placeholderParam,
		},
		ValidationCode("max"): {
			English:    placeholderField + " must be at most " + placeholderParam,
			Vietnamese: placeholderField + " phải tối đa " + placeholderParam,
		},
		ValidationCode("len"): {
			English:    placeholderField + " must be exactly " + placeholderParam + " characters",
			Vietnamese: placeholderField + " phải có đúng " + placeholderParam + " ký tự",
		},
		ValidationCode("gt"): {
			English:    placeholderField + " must be greater than " + placeholderParam,
			Vietnamese: placeholderField + " phải lớn hơn " + placeholderParam,
		},
		ValidationCode("gte"): {
			English:    placeholderField + " must be greater than or equal to " + placeholderParam,
			Vietnamese: placeholderField + " phải lớn hơn hoặc bằng " + placeholderParam,
		},
		ValidationCode("lt"): {
			English:    placeholderField + " must be less than " + placeholderParam,
			Vietnamese: placeholderField + " phải nhỏ hơn " + placeholderParam,
		},
		ValidationCode("lte"): {
			English:    placeholderField + " must be less than or equal to " + placeholderParam,
			Vietnamese: placeholderField + " phải nhỏ hơn hoặc bằng " + placeholderParam,
		},
		ValidationCode("oneof"): {
			English:    placeholderField + " must be one of [" + placeholderParam + "]",
			Vietnamese: placeholderField + " phải là một trong [" + placeholderParam + "]",
		},
		ValidationCode("nefield"): {
			English:    placeholderField + " must be different from " + placeholderParam,
			Vietnamese: placeholderField + " phải khác " + placeholderParam,
		},
	}
}
