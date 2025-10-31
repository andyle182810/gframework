package kctoken

type KeycloakError struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}
