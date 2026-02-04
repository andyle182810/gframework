package httpclient_test

import (
	"sort"
	"testing"

	"github.com/andyle182810/gframework/httpclient"
	"github.com/stretchr/testify/require"
)

func TestNewRegistry_CreatesEmptyRegistry(t *testing.T) {
	t.Parallel()

	reg := httpclient.NewRegistry()

	require.Equal(t, 0, reg.Count())
}

func TestRegistry_RegistersClient(t *testing.T) {
	t.Parallel()

	reg := httpclient.NewRegistry()
	reg.Register("users-api", "https://users.example.com")

	require.True(t, reg.Has("users-api"))
}

func TestRegistry_RegisterReturnsRegistryForChaining(t *testing.T) {
	t.Parallel()

	reg := httpclient.NewRegistry()

	result := reg.Register("api1", "https://api1.example.com")

	require.Same(t, reg, result)
}

func TestRegistry_SupportsChainedRegistration(t *testing.T) {
	t.Parallel()

	reg := httpclient.NewRegistry().
		Register("api1", "https://api1.example.com").
		Register("api2", "https://api2.example.com").
		Register("api3", "https://api3.example.com")

	require.Equal(t, 3, reg.Count())
}

func TestRegistry_OverwritesExistingClient(t *testing.T) {
	t.Parallel()

	reg := httpclient.NewRegistry()
	reg.Register("api", "https://old.example.com")
	reg.Register("api", "https://new.example.com")

	client := reg.Client("api")

	require.Equal(t, "https://new.example.com", client.BaseURL())
}

func TestRegistry_ClientReturnsRegisteredClient(t *testing.T) {
	t.Parallel()

	reg := httpclient.NewRegistry()
	reg.Register("api", "https://api.example.com")

	client := reg.Client("api")

	require.Equal(t, "https://api.example.com", client.BaseURL())
}

func TestRegistry_ClientPanicsForUnregisteredClient(t *testing.T) {
	t.Parallel()

	reg := httpclient.NewRegistry()

	require.Panics(t, func() {
		reg.Client("unknown")
	})
}

func TestRegistry_MustClientReturnsRegisteredClient(t *testing.T) {
	t.Parallel()

	reg := httpclient.NewRegistry()
	reg.Register("api", "https://api.example.com")

	client := reg.MustClient("api")

	require.NotNil(t, client)
}

func TestRegistry_MustClientPanicsForUnregisteredClient(t *testing.T) {
	t.Parallel()

	reg := httpclient.NewRegistry()

	require.Panics(t, func() {
		reg.MustClient("unknown")
	})
}

func TestRegistry_GetClientReturnsClientAndTrueForRegisteredClient(t *testing.T) {
	t.Parallel()

	reg := httpclient.NewRegistry()
	reg.Register("api", "https://api.example.com")

	client, ok := reg.GetClient("api")

	require.True(t, ok)
	require.NotNil(t, client)
}

func TestRegistry_GetClientReturnsNilAndFalseForUnregisteredClient(t *testing.T) {
	t.Parallel()

	reg := httpclient.NewRegistry()

	client, ok := reg.GetClient("unknown")

	require.False(t, ok)
	require.Nil(t, client)
}

func TestRegistry_HasReturnsTrueForRegisteredClient(t *testing.T) {
	t.Parallel()

	reg := httpclient.NewRegistry()
	reg.Register("api", "https://api.example.com")

	require.True(t, reg.Has("api"))
}

func TestRegistry_HasReturnsFalseForUnregisteredClient(t *testing.T) {
	t.Parallel()

	reg := httpclient.NewRegistry()

	require.False(t, reg.Has("unknown"))
}

func TestRegistry_UnregisterRemovesRegisteredClient(t *testing.T) {
	t.Parallel()

	reg := httpclient.NewRegistry()
	reg.Register("api", "https://api.example.com")

	removed := reg.Unregister("api")

	require.True(t, removed)
	require.False(t, reg.Has("api"))
}

func TestRegistry_UnregisterReturnsFalseForUnregisteredClient(t *testing.T) {
	t.Parallel()

	reg := httpclient.NewRegistry()

	removed := reg.Unregister("unknown")

	require.False(t, removed)
}

func TestRegistry_NamesReturnsAllRegisteredNames(t *testing.T) {
	t.Parallel()

	reg := httpclient.NewRegistry()
	reg.Register("api1", "https://api1.example.com")
	reg.Register("api2", "https://api2.example.com")
	reg.Register("api3", "https://api3.example.com")

	names := reg.Names()
	sort.Strings(names)

	require.Equal(t, []string{"api1", "api2", "api3"}, names)
}

func TestRegistry_CountReturnsNumberOfRegisteredClients(t *testing.T) {
	t.Parallel()

	reg := httpclient.NewRegistry()

	require.Equal(t, 0, reg.Count())

	reg.Register("api1", "https://api1.example.com")
	require.Equal(t, 1, reg.Count())

	reg.Register("api2", "https://api2.example.com")
	require.Equal(t, 2, reg.Count())

	reg.Unregister("api1")
	require.Equal(t, 1, reg.Count())
}
