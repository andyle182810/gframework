package httpclient

type Response struct {
	StatusCode int
	Headers    map[string]string
	RequestID  string
}

type ErrorResponse struct {
	Message  string `json:"message"`
	Internal string `json:"internal"`
}
