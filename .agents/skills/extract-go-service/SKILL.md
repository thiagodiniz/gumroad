---
name: extract-go-service
description: Extract a bounded part of the Rails app into a Go microservice under services/<name> with Kubernetes manifests and a feature-flagged Rails client that falls back to the in-process code. Use when asked to "extract", "split out", "move to a microservice", "port to Go", or "add a Go service" for some part of Gumroad.
argument-hint: [Ruby class or area to extract]
---

# Extract a Go service

Follow `docs/microservices/extraction-playbook.md`; `services/tax-id-validation` is the reference
implementation. Copy its layout rather than inventing one.

## Steps

1. Confirm the candidate with the trait table in the playbook (single entry point, pure
   request→response, no DB access, existing specs, safe fallback). If it fails two traits, say so
   and propose a better one before writing code.
2. Map the boundary: `rg -n "<Class>" app lib spec`, the initializers it reads via
   `GlobalConfig.get`, timeouts, cache TTLs, constants from `lib/utilities`. Record them; each
   becomes a Go config field.
3. Scaffold `services/<name>` by copying `Dockerfile`, `.dockerignore`, `Makefile`, `deploy/` from
   `services/tax-id-validation`; `go mod init github.com/antiwork/gumroad/services/<name>`.
4. Port one Ruby class at a time: read its spec, write the Go type and a table-driven test from the
   spec cases, then the implementation. Preserve truthiness rules, timeouts, and fallbacks exactly.
5. Add `internal/httpapi` with `/healthz`, `/readyz`, `POST /v1/<resource>`, optional
   `X-Internal-Token`, and `502 upstream_unavailable` for upstream failures.
6. Add the Rails side: `config/initializers/<name>_service.rb` (URL + token via `GlobalConfig`),
   `app/services/<name>_service_client.rb` (returns `nil` on any failure), and the
   `if Client.enabled? … return unless nil` prelude in the original entry point.
7. Specs: client spec with `WebMock.stub_request` (200, non-200→nil, timeout→nil) and entry-point
   spec (flag off, flag on, fallback).
8. Verify: `cd services/<name> && make lint test && docker build .`,
   `kubectl kustomize deploy/k8s/overlays/production`, `bundle exec rubocop <changed rb files>`,
   `bundle exec rspec <new specs>`, `bin/test-confidence`.
9. Add the service to the matrix in `.github/workflows/go-services.yml` and to the table in
   `docs/microservices/README.md`.
10. Commit with `.agents/skills/commit`. Do not delete the Ruby implementation in the same PR.

## Guardrails

- Never read the Rails database from the service; if the candidate needs it, it is not a candidate.
- Secrets only via environment / Kubernetes Secret; `secret.example.yaml` documents the keys.
- The client rescues network/HTTP errors only and logs one warning per fallback.
- Standard library only unless the playbook author approves a dependency.
