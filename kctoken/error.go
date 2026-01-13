package kctoken

//nolint:tagliatelle // Keycloak API returns snake_case
type KeycloakError struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}
