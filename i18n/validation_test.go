package i18n_test

import (
	"testing"

	"github.com/andyle182810/gframework/i18n"
	"github.com/stretchr/testify/require"
)

const (
	fieldPhone   = "phoneNumber"
	fieldPhoneVI = "Số điện thoại"
	tagRequired  = "required"
)

// labelledCatalog is what a migrated service registers: the builtin templates
// plus a label for each field its request structs accept.
func labelledCatalog() i18n.Catalog {
	return i18n.BuiltinCatalog().Merge(i18n.Catalog{
		i18n.FieldCode(fieldPhone): {
			i18n.English:    "Phone number",
			i18n.Vietnamese: fieldPhoneVI,
		},
	})
}

func TestValidationCode(t *testing.T) {
	t.Parallel()

	require.Equal(t, "VALIDATION_REQUIRED", i18n.ValidationCode(tagRequired))
	require.Equal(t, "VALIDATION_REQUIRED_WITH", i18n.ValidationCode("required_with"))
}

func TestFieldCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		field string
		want  string
	}{
		{name: "single word", field: "name", want: "FIELD_NAME"},
		{name: "camel case splits", field: fieldPhone, want: "FIELD_PHONE_NUMBER"},
		{name: "three words", field: "weekStartDate", want: "FIELD_WEEK_START_DATE"},
		{name: "digits stay attached", field: "line1Total", want: "FIELD_LINE1_TOTAL"},
		{name: "nested path separates", field: "lines.productId", want: "FIELD_LINES_PRODUCT_ID"},
		{name: "empty", field: "", want: "FIELD_"},
		// Query and path parameters carry no json tag, so the validator reports
		// the Go field name and these are the shapes that actually arrive.
		{name: "go field name", field: "PageSize", want: "FIELD_PAGE_SIZE"},
		{name: "bare acronym", field: "ID", want: "FIELD_ID"},
		{name: "trailing acronym", field: "vendorID", want: "FIELD_VENDOR_ID"},
		{name: "exported trailing acronym", field: "VendorID", want: "FIELD_VENDOR_ID"},
		{name: "plural acronym", field: "CooperationIDs", want: "FIELD_COOPERATION_IDS"},
		{name: "acronym then word", field: "IDList", want: "FIELD_ID_LIST"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, i18n.FieldCode(tt.field))
		})
	}
}

type validationCase struct {
	name    string
	catalog i18n.Catalog
	locale  i18n.Locale
	field   string
	tag     string
	param   string
	want    string
}

func validationMessageCases() []validationCase {
	return []validationCase{
		{
			name:    "labelled field in the requested locale",
			catalog: labelledCatalog(),
			locale:  i18n.Vietnamese,
			field:   fieldPhone,
			tag:     tagRequired,
			param:   "",
			want:    "Vui lòng nhập " + fieldPhoneVI,
		},
		{
			name:    "labelled field in english",
			catalog: labelledCatalog(),
			locale:  i18n.English,
			field:   fieldPhone,
			tag:     tagRequired,
			param:   "",
			want:    "Phone number is required",
		},
		{
			name:    "unlabelled field falls back to the wire name",
			catalog: labelledCatalog(),
			locale:  i18n.Vietnamese,
			field:   "internalCode",
			tag:     tagRequired,
			param:   "",
			want:    "Vui lòng nhập internalCode",
		},
		{
			name:    "the tag parameter is substituted",
			catalog: labelledCatalog(),
			locale:  i18n.Vietnamese,
			field:   fieldPhone,
			tag:     "max",
			param:   "50",
			want:    fieldPhoneVI + " phải tối đa 50",
		},
		{
			name:    "oneof keeps its list",
			catalog: labelledCatalog(),
			locale:  i18n.English,
			field:   "status",
			tag:     "oneof",
			param:   "active inactive",
			want:    "status must be one of [active inactive]",
		},
		{
			name:    "an unknown tag still reads as a sentence",
			catalog: labelledCatalog(),
			locale:  i18n.Vietnamese,
			field:   fieldPhone,
			tag:     "startsnotwith",
			param:   "",
			want:    fieldPhoneVI + " không hợp lệ",
		},
		{
			name:    "a catalog without the builtins still renders",
			catalog: nil,
			locale:  i18n.English,
			field:   fieldPhone,
			tag:     tagRequired,
			param:   "",
			want:    "phoneNumber is invalid",
		},
	}
}

func TestValidationMessage(t *testing.T) {
	t.Parallel()

	for _, tt := range validationMessageCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := i18n.ValidationMessage(tt.catalog, tt.locale, tt.field, tt.tag, tt.param)
			require.Equal(t, tt.want, got)
		})
	}
}
