package httpserver

type HandlerResponse[T any] struct {
	Data       T           `json:"data"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

func NewResponse[T any](data T) *HandlerResponse[T] {
	return &HandlerResponse[T]{
		Data:       data,
		Pagination: nil,
	}
}

func NewPaginatedResponse[T any](data T, pagination *Pagination) *HandlerResponse[T] {
	return &HandlerResponse[T]{
		Data:       data,
		Pagination: pagination,
	}
}

type Pagination struct {
	Page       int `example:"1"  json:"page"`
	PageSize   int `example:"10" json:"pageSize"`
	TotalCount int `example:"42" json:"totalCount"`
	TotalPages int `example:"5"  json:"totalPages"`
}

type APIResponse[T any] struct {
	RequestID  string      `example:"3bf74527-8097-4217-8485-ffe05d16f82e" json:"requestId,omitempty"`
	Data       T           `json:"data"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

type ResponseError struct {
	Message  string `json:"message"`
	Internal string `json:"internal"`
}

func (e ResponseError) Error() string {
	return e.Message
}
