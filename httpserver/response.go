package httpserver

type HandlerResponse[T any] struct {
	Data       T           `json:"data"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

type Pagination struct {
	Page       int `json:"page" example:"1"`
	PageSize   int `json:"pageSize" example:"10"`
	TotalCount int `json:"totalCount" example:"42"`
	TotalPages int `json:"totalPages" example:"5"`
}

type APIResponse[T any] struct {
	RequestID  string      `json:"requestId,omitempty" example:"3bf74527-8097-4217-8485-ffe05d16f82e"`
	Data       T           `json:"data"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

type ErrorResponse struct {
	Message string `json:"message"`
}

func (e ErrorResponse) Error() string {
	return e.Message
}
