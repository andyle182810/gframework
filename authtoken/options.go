package authtoken

import (
	"time"

	"github.com/Nerzal/gocloak/v13"
)

type Option func(*Client)

func WithTokenExpiryBuffer(buffer time.Duration) Option {
	return func(c *Client) {
		if buffer > 0 {
			c.expiryBuffer = buffer
		}
	}
}

func WithGoCloakClient(client *gocloak.GoCloak) Option {
	return func(c *Client) {
		if client != nil {
			c.gocloak = client
		}
	}
}
