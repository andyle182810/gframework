# jwt-auth

JWT authentication and authorization with gframework:

- **JWKS key fetching** (`jwks`) with background refresh and key-rotation support
- **Hardened JWT validation** (`middleware.JWTWithConfig`): algorithm pinning
  (asymmetric only, on by default) plus issuer and audience checks
- **Realm-role authorization** (`middleware.RequireAnyRealmRole`)
- **Internal service-to-service auth** (`middleware.InternalAuthWithConfig`) via
  the `X-Internal-Authorization` header and an `azp` allow-list

## Run

Start Keycloak and create a realm called `demo` with a client `demo-api`
(set the client's audience mapper so tokens carry `aud: demo-api`):

```bash
docker compose up -d
# Keycloak admin console: http://localhost:8080 (admin/admin)
go run .
```

Get a token and call the API:

```bash
TOKEN=$(curl -s http://localhost:8080/realms/demo/protocol/openid-connect/token \
  -d grant_type=password -d client_id=demo-api \
  -d username=alice -d password=alice | jq -r .access_token)

curl -H "Authorization: Bearer $TOKEN" http://localhost:8081/api/me
curl -H "Authorization: Bearer $TOKEN" http://localhost:8081/api/admin/stats   # 403 without the admin role
```

## Why issuer/audience pinning matters

Without `Issuer`/`Audiences` configured, *any* token signed by *any* key in the
realm's JWKS is accepted — including tokens minted for entirely different
services. Pinning rejects those at the middleware layer.

For the calling side of `/internal/sync`, see the `httpclient` package: use
`WithTokenProvider` (service-account token) together with `WithInternalAuthHeader()`.
