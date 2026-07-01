# gframework

Go microservices framework (module `github.com/andyle182810/gframework`, Go >= 1.26.4). ~23 independent
packages providing building blocks for backend services: Echo v5 HTTP server, Keycloak/JWT auth, Postgres,
Redis/Valkey messaging, S3-compatible storage, and lifecycle management. See `README.md` for the full package
table and the security-configuration checklist — read it before touching auth, CORS, or error-handler defaults.

## Commands

```bash
make test        # go test -v -count=1 ./...  (integration tests spin up Docker via testcontainers-go)
make lint         # go mod tidy && go vet ./... && golangci-lint fmt && golangci-lint run ./...
make lint-fix     # same as lint, but golangci-lint run --fix
make test-coverage
make benchmark
```

Tests that need Postgres/Valkey use `testutil.SetupPostgresContainer(t)` / `testutil.SetupValkeyContainer(t)`
(testcontainers-go) — Docker must be running. There is no build-tag separation between unit and integration
tests; `go test ./...` runs everything, so containers only start for the packages that need them.

Each directory under `examples/` is its own Go module (own `go.mod`) — build with `(cd examples/<name> && go build ./...)`,
not from the repo root.

## Conventions (enforced by ~95 golangci-lint v2 linters, see `.golangci.yaml`)

- **Functional options**: `New(cfg *Config, opts ...Option)` is the standard constructor shape across
  authtoken, cache, jwks, keycloak, httpclient, redispub, redissub, runner, spaces, taskqueue, validator, workerpool.
- **Interface-based design**: small, composed interfaces (e.g. postgres splits `DBPool`/`Health`/`Lifecycle`/
  `Executor`/`TxRunner`; middleware defines narrow `TokenProvider`/`Skipper` interfaces). Prefer accepting an
  interface over a concrete struct at package boundaries.
- **Sentinel errors**: package-level `Err`-prefixed vars (`ErrConfigNil`, `ErrLockNotObtained`, ...), wrapped
  with `fmt.Errorf("...: %w", err)` for `errors.Is`/`errors.As`. HTTP errors use `echo.HTTPError`, rendered by
  `middleware.ErrorHandler`.
- **Context first**: `context.Context` is the first parameter on any lifecycle/async method.
- **Structured logging**: zerolog throughout (`log.Warn().Str("source", "gframework").Msg(...)` style); request
  logs auto-redact sensitive query params (`token`, `code`, `api_key`, `password`, ...).
- **Services implement `runner.Service`** (`Start(ctx) error`, `Stop() error`, `Name() string`); register
  databases/caches/queues via `WithInfrastructureService` (start first, stop last), HTTP servers/subscribers via
  `WithCoreService`.
- Test files live in `package <name>_test` (external test package), use `t.Parallel()`, and lean on
  `testify` (`require`/`assert`).

### Lint specifics worth knowing before fixing lint failures

- `funlen`: 80 lines / 60 statements max. `lll`: 130 chars.
- `varnamelen` allows short names only for `id, db, ctx, err, v1, pg, wg, tx, tt, c, e, v, w, r` — anything else
  needs a real name.
- `exhaustruct` requires every struct field set at construction unless annotated `//nolint:exhaustruct`
  (commonly used on `Config`/`Server` literals with many optional fields — see `httpserver/server.go`).
- `depguard`, `gomoddirectives`, `wrapcheck` are explicitly disabled — don't reflexively "fix" their absence.
- `examples/` is excluded from most path-based lint rules (and from formatter paths).

## Commit messages

`type(scope): summary`, lowercase, no trailing period — scope is the package name (e.g.
`feat(websocket): add WebSocket hub library`, `test(redissub): replace fixed sleep with polling for health checks`,
`refactor: ParseSundayDate returns time.Time, not bool`, `ci: upgrade golangci-lint to v2`). Scope is omitted for
changes spanning multiple packages.
