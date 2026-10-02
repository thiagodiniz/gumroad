# Microservices

Gumroad is a Rails monolith. Where a part of it has a crisp boundary and gains from running
separately (independent scaling, different runtime, isolation from the Rails deploy), it can be
extracted into a Go service that lives in this repository under `services/<name>` and deploys to
Kubernetes from `services/<name>/deploy/k8s`.

| Service                                                                    | Replaces                                               | Rails entry point              |
| -------------------------------------------------------------------------- | ------------------------------------------------------ | ------------------------------ |
| [`services/tax-id-validation`](../../services/tax-id-validation/README.md) | `RegionalVatIdValidationService` + registry validators | `TaxIdValidationServiceClient` |
| [`services/oembed`](../../services/oembed/README.md)                       | `OEmbedFinder` + the ruby-oembed provider registry     | `OEmbedServiceClient`          |

Every extraction follows the [extraction playbook](extraction-playbook.md). The short version:
the Rails code keeps working exactly as before, a client is added behind a feature flag and a
service URL, the client returns `nil` (not an exception) when the service is unreachable, and the
caller falls back to the in-process implementation. Nothing is deleted from Rails until the
service has carried production traffic.

## Rollout sequence for a service

1. Build and push the image (CI pushes `ghcr.io/<owner>/<service>:<sha>` on merge to `main`), create the Kubernetes Secret, `kubectl apply -k
services/<name>/deploy/k8s/overlays/staging`.
2. Set `<NAME>_SERVICE_URL` (and token) in the Rails environment. The flag stays off, so nothing changes.
3. `Flipper.enable_percentage_of_time(:<flag>, 5)` (the client checks the flag without an actor, so
   `activate_percentage`/percentage-of-actors would never match) → watch the service's error rate
   and the Rails fallback warnings (`<Client>: ...` in the Rails logs).
4. Ramp to 100 %. Leave the in-process implementation in place for at least one release.
5. Delete the Ruby implementation, the flag, and the fallback path; the client becomes the only route.

## Shared conventions

- One Go module per service, `go 1.24`, standard library only unless there is a strong reason.
- `cmd/server/main.go` wires config → clients → HTTP server; `internal/` holds everything else.
- `GET /healthz`, `GET /readyz`, optional `X-Internal-Token` auth, JSON errors
  `{"error": "<snake_case_code>", "message": "..."}`; `502 upstream_unavailable` tells Rails to fall back.
- Credentials reuse the `GlobalConfig` names the Rails code already uses so the same secret store
  feeds both.
- Image: multi-stage `golang:1.24-alpine` → `gcr.io/distroless/static-debian12:nonroot`.
- `deploy/k8s/base` + `overlays/{staging,production}` (kustomize); resources, probes, PDB, HPA,
  NetworkPolicy restricting ingress to the Rails pods.
- `make lint test docker-build k8s-render` works from the service directory; CI runs the same
  targets via `.github/workflows/go-services.yml`.
