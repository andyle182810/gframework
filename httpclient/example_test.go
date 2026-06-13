package httpclient_test

import (
	"context"
	"fmt"

	"github.com/andyle182810/gframework/httpclient"
)

// ExampleNew shows a typical client setup. (Not run: requires a live API.)
//
//nolint:testableexamples // requires live infrastructure; compile-checked only
func ExampleNew() {
	type User struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	client := httpclient.New(
		"https://api.example.com",
		httpclient.WithMaxResponseSize(1<<20),
	)

	var user User
	if err := client.Get(context.Background(), "/users/123", &user); err != nil {
		fmt.Println("request failed:", err)

		return
	}

	fmt.Println(user.Name)
}

// ExampleWithInternalAuthHeader shows a service-to-service client that
// authenticates against endpoints protected by middleware.InternalAuth.
// (Not run: requires Keycloak and a live service.)
//
//nolint:testableexamples // requires live infrastructure; compile-checked only
func ExampleWithInternalAuthHeader() {
	client := httpclient.New(
		"https://internal-billing.svc.local",
		httpclient.WithAuth(httpclient.AuthConfig{ //nolint:exhaustruct
			BaseURL:      "https://auth.example.com",
			Realm:        "my-realm",
			ClientID:     "report-service",
			ClientSecret: "service-secret",
		}),
		// Sends the token in X-Internal-Authorization as well — only safe for
		// clients that exclusively call internal services.
		httpclient.WithInternalAuthHeader(),
	)

	_ = client.Post(context.Background(), "/internal/sync", nil, nil)
}
