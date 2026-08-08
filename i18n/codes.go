package i18n

import "net/http"

// Generic message codes, one per status a handler can return without naming a
// specific rule. They are the fallback the error handler reports when a message
// is literal prose rather than a catalog code, so every error carries a code a
// client can branch on even before a service is fully migrated.
const (
	CodeBadRequest         = "BAD_REQUEST"
	CodeValidation         = "VALIDATION_ERROR"
	CodeUnauthorized       = "UNAUTHORIZED"
	CodeForbidden          = "FORBIDDEN"
	CodeNotFound           = "NOT_FOUND"
	CodeConflict           = "CONFLICT"
	CodeInternal           = "INTERNAL_ERROR"
	CodeServiceUnavailable = "SERVICE_UNAVAILABLE"
)

// BuiltinCatalog returns text for the generic codes. Services merge their own
// domain catalog over it; a service that wants a plain localized "not found"
// can raise CodeNotFound directly instead of inventing a code.
func BuiltinCatalog() Catalog {
	return Catalog{
		CodeBadRequest: {
			English:    "Invalid request",
			Vietnamese: "Yêu cầu không hợp lệ",
		},
		CodeValidation: {
			English:    "Invalid request data",
			Vietnamese: "Dữ liệu không hợp lệ",
		},
		CodeUnauthorized: {
			English:    "Authentication required",
			Vietnamese: "Yêu cầu chưa được xác thực",
		},
		CodeForbidden: {
			English:    "You do not have permission to perform this action",
			Vietnamese: "Bạn không có quyền thực hiện thao tác này",
		},
		CodeNotFound: {
			English:    "Not found",
			Vietnamese: "Không tìm thấy dữ liệu",
		},
		CodeConflict: {
			English:    "The request conflicts with existing data",
			Vietnamese: "Yêu cầu xung đột với dữ liệu hiện có",
		},
		CodeInternal: {
			English:    "Something went wrong, please try again",
			Vietnamese: "Đã xảy ra lỗi, vui lòng thử lại",
		},
		CodeServiceUnavailable: {
			English:    "The service is temporarily unavailable",
			Vietnamese: "Dịch vụ tạm thời không khả dụng",
		},
	}
}

// CodeForStatus is the generic code reported for an HTTP status.
func CodeForStatus(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return CodeUnauthorized
	case http.StatusForbidden:
		return CodeForbidden
	case http.StatusNotFound:
		return CodeNotFound
	case http.StatusConflict:
		return CodeConflict
	case http.StatusUnprocessableEntity:
		return CodeValidation
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return CodeServiceUnavailable
	}

	if status >= http.StatusInternalServerError {
		return CodeInternal
	}

	return CodeBadRequest
}
