//nolint:ireturn
package httpclient

func NewTokenProviderForTest(cfg AuthConfig) TokenProvider {
	return newTokenProvider(cfg)
}

type TokenResponseForTest = tokenResponse
