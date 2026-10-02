# Extracting a part of Gumroad into a Go service

A repeatable procedure for carving a bounded piece of the Rails app into a Go microservice that
lives in `services/<name>` and ships to Kubernetes, without a big-bang cutover. The first run of
this playbook produced [`services/tax-id-validation`](../../services/tax-id-validation/README.md);
use it as the reference implementation for every step below.

## 1. Pick a candidate

Good candidates share these traits. Score each one; anything missing two or more is a poor first
extraction.

| Trait                                  | Why it matters                                                  | tax-id-validation                               |
| -------------------------------------- | --------------------------------------------------------------- | ----------------------------------------------- |
| Single Ruby entry point                | One place to put the client                                     | `RegionalVatIdValidationService#process`        |
| Pure request → response                | No ActiveRecord writes, no callbacks, no jobs enqueued          | returns `true`/`false`                          |
| Few, small inputs                      | The JSON contract stays trivial                                 | `tax_id`, `country_code`, `state_code`          |
| Talks to external APIs or is CPU heavy | That is the part that benefits from isolation / another runtime | Vatstack, IRAS, VIES, Tax ID Pro, Revenu Québec |
| No shared DB tables                    | A service that needs the Rails DB is a distributed monolith     | none                                            |
| Existing specs                         | They become the parity test suite                               | `spec/services/*_validation_service_spec.rb`    |
| Safe to fall back                      | Rails can keep doing the work if the service is down            | yes                                             |

Other candidates that score well: `app/services/link_preview*`/URL unfurling, PDF/invoice
rendering, image and video thumbnailing (`app/sidekiq/*thumbnail*`), webhook/ping fan-out
(`PostToPingEndpointsWorker`), geo/IP resolution, and currency-rate fetching.

Poor candidates: anything inside `Purchase` state transitions, payouts, or anything reading
`User`/`Link` rows directly.

## 2. Map the boundary

```
rg -n "ClassName" app lib spec            # every caller and every spec
rg -n "GlobalConfig.get" config/initializers/<area>*.rb   # credentials and endpoints it needs
```

Write down: inputs, output, side effects (cache writes, logging), external endpoints with their
timeouts, cache TTLs, and the list of constants it reads from `lib/utilities/compliance` or
`config/`. Everything on this list must appear in the Go config or the port is incomplete.

## 3. Scaffold the service

```
services/<name>/
  go.mod                       module github.com/antiwork/gumroad/services/<name>; go 1.24
  cmd/server/main.go           config → clients → router → HTTP server, SIGTERM drain
  internal/config/config.go    env parsing, defaults mirroring the Ruby constants
  internal/<domain>/           the port; one file per Ruby class, same names
  internal/httpapi/server.go   /healthz /readyz /v1/<resource>, X-Internal-Token
  internal/cache/              only if the Ruby code used Rails.cache
  Dockerfile  .dockerignore  Makefile  README.md
  deploy/k8s/base/ + deploy/k8s/overlays/{staging,production}/
```

Copy `services/tax-id-validation/{Dockerfile,Makefile,.dockerignore,deploy}` and rename. Keep
to the standard library; the whole point is a small, boring binary.

## 4. Port behaviour, not code

For each Ruby class, read the spec first, then the implementation, then write the Go type and its
table-driven test from the spec cases. Preserve:

- the exact truthiness rules (e.g. Vatstack needs `valid && active`; IRAS needs `returnCode == "10"`
  and `Status == "Registered"`),
- timeouts (`VIES_LOOKUP_TIMEOUT_SECONDS` → `VIES_TIMEOUT`),
- fallbacks (VIES down → syntax-only is a _feature_ that must survive the port),
- cache TTLs and the rule that failures are not cached.

Where a gem did the work (`valvat`), read the gem's source for the rules you are reproducing and
link it in a comment. Note deliberate gaps in the README.

## 5. HTTP contract

`POST /v1/<resource>` with a flat JSON body, snake_case keys identical to the Ruby keyword
arguments. Responses:

| Status | Body                                | Rails behaviour                      |
| ------ | ----------------------------------- | ------------------------------------ |
| 200    | `{"valid": bool, ...}`              | use the result                       |
| 400    | `{"error": "invalid_request"}`      | bug — log and fall back              |
| 401    | `{"error": "unauthorized"}`         | misconfiguration — log and fall back |
| 502    | `{"error": "upstream_unavailable"}` | fall back to in-process              |

## 6. Rails client (strangler)

```ruby
# config/initializers/<name>_service.rb
<NAME>_SERVICE_URL   = GlobalConfig.get("<NAME>_SERVICE_URL")
<NAME>_SERVICE_TOKEN = GlobalConfig.get("<NAME>_SERVICE_TOKEN")

# app/services/<name>_service_client.rb
class <Name>ServiceClient
  FEATURE_FLAG = :<name>_service
  def self.enabled? = <NAME>_SERVICE_URL.present? && Feature.active?(FEATURE_FLAG)
  def call(...)  # returns the verdict, or nil on any failure
end

# original entry point
def process
  if <Name>ServiceClient.enabled?
    remote = <Name>ServiceClient.new.call(...)
    return remote unless remote.nil?
  end
  process_in_process
end
```

Rules: `nil` means "no answer", never raise; rescue only network/HTTP errors; one `Rails.logger.warn`
per failure so the fallback rate is visible; the client timeout is slightly above the service's
longest upstream timeout.

## 7. Tests

- Go: `make lint test` (gofmt, vet, `-race`). Use `httptest.Server` for every upstream; assert the
  request the Go code sends, not just the response handling.
- Rails: a client spec with `WebMock.stub_request` covering 200/false, non-200 → nil, timeout → nil,
  and the entry-point spec covering flag off → in-process, flag on → remote, remote nil → in-process.
- `bin/test-confidence` before committing, per `.agents/skills/commit`.

## 8. Container and Kubernetes

Multi-stage build to `distroless/static:nonroot`, `readOnlyRootFilesystem`, no capabilities,
requests/limits, readiness + liveness probes, `terminationGracePeriodSeconds` ≥ longest upstream
timeout + `SHUTDOWN_TIMEOUT`, PDB `minAvailable: 1`, HPA on CPU, NetworkPolicy allowing only the
Rails web/sidekiq pods. Secrets are never in the repo: `secret.example.yaml` documents the keys and
the real Secret is created by the ops tooling. Verify with `kubectl kustomize deploy/k8s/overlays/production`.

## 9. CI

Add the service directory to the path filter in `.github/workflows/go-services.yml`; the matrix
runs `make lint test` and `docker build` for each service.

## 10. Roll out and delete

Follow the [rollout sequence](README.md#rollout-sequence-for-a-service). The extraction is only
finished when the Ruby implementation is deleted; until then the Go service is a shadow, not a
replacement.

## Checklist

- [ ] boundary map written (inputs, outputs, endpoints, timeouts, TTLs, constants)
- [ ] every Ruby class has a Go counterpart with table-driven tests from the spec
- [ ] `make lint test` green, `docker build` succeeds, `kubectl kustomize` renders both overlays
- [ ] client returns `nil` on failure; entry point falls back; both have specs
- [ ] feature flag + URL/token initializer; nothing changes while the flag is off
- [ ] service README lists config, API, deliberate gaps
- [ ] `docs/microservices/README.md` table updated
