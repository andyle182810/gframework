package keycloak

import (
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
)

type AdminOption func(*AdminClient)

func WithTokenSafetyBuffer(buffer time.Duration) AdminOption {
	return func(c *AdminClient) {
		if buffer > 0 {
			c.tokenSafetyBuffer = buffer
		}
	}
}

func WithRestyClient(restyClient *resty.Client) AdminOption {
	return func(c *AdminClient) {
		if restyClient != nil {
			c.gocloak.SetRestyClient(restyClient)
		}
	}
}

type UMAOption func(*UMAClient)

func WithUMAHTTPClient(httpClient *http.Client) UMAOption {
	return func(c *UMAClient) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}
