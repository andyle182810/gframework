// Package keycloak provides a Keycloak integration with two complementary clients:
//
//   - AdminClient: a service-account-backed wrapper around gocloak/v13 for user
//     and realm-role lifecycle management. A service-account token is cached
//     in-memory behind an RWMutex with double-check locking and refreshed on
//     expiry with a configurable safety buffer (default 60 s). Safe for
//     concurrent use.
//
//   - UMAClient: a stateless UMA 2.0 permission-decision client. Given an end-user
//     access token, it calls the realm token endpoint with the uma-ticket grant
//     type and decision response mode to answer a single resource#scope check.
//
// Basic usage:
//
//	admin := keycloak.NewAdminClient(
//	    "https://kc.example.com",
//	    "my-realm",
//	    "svc-account",
//	    "svc-secret",
//	)
//
//	id, err := admin.CreateUser(ctx, keycloak.CreateUserParams{
//	    Username: "alice",
//	    Email:    "alice@example.com",
//	    Password: "s3cret",
//	})
//
//	uma := keycloak.NewUMAClient(
//	    "https://kc.example.com/realms/my-realm/protocol/openid-connect/token",
//	    "my-resource-server",
//	)
//
//	allowed, err := uma.Check(ctx, userToken, "document:42", "read")
package keycloak

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Nerzal/gocloak/v13"
)

const defaultTokenSafetyBuffer = 60 * time.Second

var (
	ErrRoleNotFound = errors.New("keycloak: realm role not found")
	ErrInvalidInput = errors.New("keycloak: invalid input")
)

type AdminClient struct {
	gocloak      *gocloak.GoCloak
	realm        string
	clientID     string
	clientSecret string

	tokenSafetyBuffer time.Duration

	tokenMu     sync.RWMutex
	cachedToken *gocloak.JWT
	tokenExpiry time.Time
}

func NewAdminClient(
	baseURL string,
	realm string,
	clientID string,
	clientSecret string,
	opts ...AdminOption,
) *AdminClient {
	c := &AdminClient{ //nolint:exhaustruct
		gocloak:           gocloak.NewClient(baseURL),
		realm:             realm,
		clientID:          clientID,
		clientSecret:      clientSecret,
		tokenSafetyBuffer: defaultTokenSafetyBuffer,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *AdminClient) token(ctx context.Context) (string, error) {
	c.tokenMu.RLock()
	if c.cachedToken != nil && time.Now().Before(c.tokenExpiry) {
		token := c.cachedToken.AccessToken
		c.tokenMu.RUnlock()

		return token, nil
	}
	c.tokenMu.RUnlock()

	return c.refreshToken(ctx)
}

func (c *AdminClient) refreshToken(ctx context.Context) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	if c.cachedToken != nil && time.Now().Before(c.tokenExpiry) {
		return c.cachedToken.AccessToken, nil
	}

	jwt, err := c.gocloak.LoginClient(ctx, c.clientID, c.clientSecret, c.realm)
	if err != nil {
		return "", err
	}

	c.cachedToken = jwt
	c.tokenExpiry = time.Now().Add(time.Duration(jwt.ExpiresIn)*time.Second - c.tokenSafetyBuffer)

	return jwt.AccessToken, nil
}

func (c *AdminClient) InvalidateToken() {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	c.cachedToken = nil
	c.tokenExpiry = time.Time{}
}

type CreateUserParams struct {
	Username  string
	Email     string
	FirstName string
	LastName  string
	Password  string
}

func (c *AdminClient) CreateUser(ctx context.Context, params CreateUserParams) (string, error) {
	if params.Username == "" || params.Email == "" {
		return "", fmt.Errorf("%w: CreateUser requires Username and Email", ErrInvalidInput)
	}

	token, err := c.token(ctx)
	if err != nil {
		return "", err
	}

	enabled := true
	verified := false
	user := gocloak.User{ //nolint:exhaustruct
		Username:      &params.Username,
		Email:         &params.Email,
		FirstName:     &params.FirstName,
		LastName:      &params.LastName,
		Enabled:       &enabled,
		EmailVerified: &verified,
	}

	id, err := c.gocloak.CreateUser(ctx, token, c.realm, user)
	if err != nil {
		return "", err
	}

	if params.Password != "" {
		if err := c.gocloak.SetPassword(ctx, token, id, c.realm, params.Password, false); err != nil {
			return id, err
		}
	}

	return id, nil
}

type UpdateUserParams struct {
	Email     string
	FirstName string
	LastName  string
}

func (c *AdminClient) UpdateUser(ctx context.Context, id string, params UpdateUserParams) error {
	if id == "" {
		return fmt.Errorf("%w: UpdateUser requires id", ErrInvalidInput)
	}

	token, err := c.token(ctx)
	if err != nil {
		return err
	}

	user := gocloak.User{ //nolint:exhaustruct
		ID:        &id,
		Email:     &params.Email,
		FirstName: &params.FirstName,
		LastName:  &params.LastName,
	}

	if err := c.gocloak.UpdateUser(ctx, token, c.realm, user); err != nil {
		return err
	}

	return nil
}

func (c *AdminClient) SetEnabled(ctx context.Context, id string, enabled bool) error {
	if id == "" {
		return fmt.Errorf("%w: SetEnabled requires id", ErrInvalidInput)
	}

	token, err := c.token(ctx)
	if err != nil {
		return err
	}

	flag := enabled
	user := gocloak.User{ //nolint:exhaustruct
		ID:      &id,
		Enabled: &flag,
	}

	if err := c.gocloak.UpdateUser(ctx, token, c.realm, user); err != nil {
		return err
	}

	return nil
}

func (c *AdminClient) GetUser(ctx context.Context, id string) (*gocloak.User, error) {
	if id == "" {
		return nil, fmt.Errorf("%w: GetUser requires id", ErrInvalidInput)
	}

	token, err := c.token(ctx)
	if err != nil {
		return nil, err
	}

	user, err := c.gocloak.GetUserByID(ctx, token, c.realm, id)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (c *AdminClient) ListUsers(ctx context.Context, offset, limit int) ([]*gocloak.User, error) {
	if offset < 0 || limit <= 0 {
		return nil, fmt.Errorf("%w: ListUsers requires offset>=0 and limit>0", ErrInvalidInput)
	}

	token, err := c.token(ctx)
	if err != nil {
		return nil, err
	}

	params := gocloak.GetUsersParams{ //nolint:exhaustruct
		First: &offset,
		Max:   &limit,
	}

	users, err := c.gocloak.GetUsers(ctx, token, c.realm, params)
	if err != nil {
		return nil, err
	}

	return users, nil
}

func (c *AdminClient) UserRoles(ctx context.Context, id string) ([]*gocloak.Role, error) {
	if id == "" {
		return nil, fmt.Errorf("%w: UserRoles requires id", ErrInvalidInput)
	}

	token, err := c.token(ctx)
	if err != nil {
		return nil, err
	}

	roles, err := c.gocloak.GetRealmRolesByUserID(ctx, token, c.realm, id)
	if err != nil {
		return nil, err
	}

	return roles, nil
}

func (c *AdminClient) AddRole(ctx context.Context, id, name string) error {
	return c.applyRole(ctx, id, name, "add", c.gocloak.AddRealmRoleToUser)
}

func (c *AdminClient) RemoveRole(ctx context.Context, id, name string) error {
	return c.applyRole(ctx, id, name, "remove", c.gocloak.DeleteRealmRoleFromUser)
}

type realmRoleOp func(ctx context.Context, token, realm, userID string, roles []gocloak.Role) error

func (c *AdminClient) applyRole(
	ctx context.Context,
	id, name, action string,
	op realmRoleOp,
) error {
	if id == "" || name == "" {
		return fmt.Errorf("%w: %s role requires id and name", ErrInvalidInput, action)
	}

	token, err := c.token(ctx)
	if err != nil {
		return err
	}

	role, err := c.gocloak.GetRealmRole(ctx, token, c.realm, name)
	if err != nil {
		return err
	}

	if err := op(ctx, token, c.realm, id, []gocloak.Role{*role}); err != nil {
		return err
	}

	return nil
}

func (c *AdminClient) Roles(ctx context.Context) ([]*gocloak.Role, error) {
	token, err := c.token(ctx)
	if err != nil {
		return nil, err
	}

	roles, err := c.gocloak.GetRealmRoles(ctx, token, c.realm, gocloak.GetRoleParams{}) //nolint:exhaustruct
	if err != nil {
		return nil, err
	}

	return roles, nil
}

func (c *AdminClient) SendActionsEmail(ctx context.Context, id string, actions []string) error {
	if id == "" || len(actions) == 0 {
		return fmt.Errorf("%w: SendActionsEmail requires id and at least one action", ErrInvalidInput)
	}

	token, err := c.token(ctx)
	if err != nil {
		return err
	}

	params := gocloak.ExecuteActionsEmail{ //nolint:exhaustruct
		UserID:  &id,
		Actions: &actions,
	}

	if err := c.gocloak.ExecuteActionsEmail(ctx, token, c.realm, params); err != nil {
		return err
	}

	return nil
}
