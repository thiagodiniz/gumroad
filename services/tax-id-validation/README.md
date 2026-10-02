# tax-id-validation

Go port of `RegionalVatIdValidationService` and the registry validators behind it
(`AbnValidationService`, `GstValidationService`, `QstValidationService`, `MvaValidationService`,
`TrnValidationService`, `KraPinValidationService`, `FirsTinValidationService`,
`TraTinValidationService`, `OmanVatNumberValidationService`, `TaxIdValidationService`,
`VatValidationService`). Rails calls it through `TaxIdValidationServiceClient` and falls back to
the in-process validators whenever the service does not answer.

## API

```
POST /v1/validations
X-Internal-Token: <INTERNAL_AUTH_TOKEN>          # only when the token is configured
{"tax_id": "DE123456789", "country_code": "DE", "state_code": null}

200 {"valid": true, "validator": "eu_vat"}
400 {"error": "invalid_request", "message": "..."}  # malformed JSON / missing tax_id
401 {"error": "unauthorized"}
502 {"error": "upstream_unavailable", "message": "..."}  # registry unreachable; Rails falls back

GET /healthz   liveness
GET /readyz    readiness
```

Routing mirrors the Rails dispatcher: AU→Vatstack (`au_gst`), SG→IRAS, CA/QC→Revenu Québec,
NO→Vatstack (`no_vat`), BH/KE/NG/TZ/OM→format checks, Tax ID Pro countries→Tax ID Pro, everything
else→EU VAT syntax + VIES (syntax-only when VIES is unavailable, as in `VatValidationService`).
Verdicts are cached in-process for `CACHE_TTL` (10 min, the same TTL the Rails services use);
upstream failures are never cached.

## Configuration

| Variable                                          | Default          | Notes                                                                                                  |
| ------------------------------------------------- | ---------------- | ------------------------------------------------------------------------------------------------------ |
| `PORT`                                            | `8080`           |                                                                                                        |
| `APP_ENV`                                         | `development`    | `production` selects the IRAS production endpoint                                                      |
| `LOG_LEVEL`                                       | `info`           | `debug`, `info`, `warn`, `error`                                                                       |
| `INTERNAL_AUTH_TOKEN`                             | unset            | When set, `/v1/validations` requires a matching `X-Internal-Token`; required when `APP_ENV=production` |
| `CACHE_TTL`                                       | `10m`            | Go duration or bare seconds                                                                            |
| `UPSTREAM_TIMEOUT`                                | `5s`             | Vatstack, IRAS, Tax ID Pro, Revenu Québec                                                              |
| `VIES_TIMEOUT`                                    | `30s`            | Matches `VatValidationService::VIES_LOOKUP_TIMEOUT_SECONDS`                                            |
| `SHUTDOWN_TIMEOUT`                                | `20s`            | Drain window on SIGTERM                                                                                |
| `VATSTACK_API_KEY`, `VATSTACK_URL`                |                  | Same key as the Rails `VATSTACK_API_KEY`                                                               |
| `IRAS_API_ID`, `IRAS_API_SECRET`, `IRAS_ENDPOINT` |                  | Same credentials as Rails                                                                              |
| `TAX_ID_PRO_API_KEY`, `TAX_ID_PRO_URL`            |                  | Same key as Rails                                                                                      |
| `REVENU_QUEBEC_URL`, `VIES_URL`                   |                  | Override for tests / sandboxes                                                                         |
| `VAT_REGISTRATION_NUMBER`                         | `GB-...` default | Requester VAT number sent to VIES                                                                      |

## Develop

```
make lint test          # gofmt + go vet + go test -race
make run                # http://localhost:8080
make docker-build       # ghcr.io/thiagodiniz/tax-id-validation:<git sha>
make k8s-render OVERLAY=staging
```

Point Rails at a local instance with `TAX_ID_VALIDATION_SERVICE_URL=http://localhost:8080` and
`Feature.activate(:tax_id_validation_service)`.

## Deploy

`deploy/k8s` is a kustomize base with `staging` and `production` overlays (Deployment, Service,
ConfigMap, HPA, PDB, NetworkPolicy). Secrets are created out of band — see
`deploy/k8s/base/secret.example.yaml`. CI builds the image in `.github/workflows/go-services.yml` and, on `main`, pushes it to
`ghcr.io/thiagodiniz/tax-id-validation:<sha>` with `GITHUB_TOKEN`;
pin the tag in the overlay (`kustomize edit set image gumroad/tax-id-validation=ghcr.io/thiagodiniz/tax-id-validation:<sha>`) and
`kubectl apply -k deploy/k8s/overlays/<env>`.
