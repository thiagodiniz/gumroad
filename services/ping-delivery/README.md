# ping-delivery

Go port of the network half of `PostToIndividualPingEndpointWorker`
(`app/sidekiq/post_to_individual_ping_endpoint_worker.rb`): the SSRF-filtered POST to a seller's
webhook endpoint, re-sent across a bounded number of redirects, with the `ssrf_filter` gem's
reserved-address policy. Rails keeps payload encoding, `PingDelivery` records and retry
scheduling, calls the service through `PingDeliveryServiceClient`, and falls back to
`SsrfFilter.post` in-process whenever the service does not answer.

## API

```
POST /v1/deliveries
X-Internal-Token: <INTERNAL_AUTH_TOKEN>          # only when the token is configured
{"url": "https://seller.example/hook", "body": "sale_id=...", "content_type": "application/x-www-form-urlencoded"}

200 {"outcome": "responded", "status": 200, "retryable": false}            # endpoint's final status (3xx = redirect limit hit)
200 {"outcome": "error", "error_class": "Net::ReadTimeout", "retryable": true}
200 {"outcome": "error", "error_class": "SsrfFilter::PrivateIPAddress", "retryable": false}
400 {"error": "invalid_request"}                  # malformed JSON / missing url or content_type
401 {"error": "unauthorized"}

GET /healthz   liveness
GET /readyz    readiness
```

Every answer is a verdict, never a 502: the only upstream is the seller's endpoint, and its
failures are what the worker records and retries. `error_class` carries the Ruby exception name
the in-process path would have raised (`SsrfFilter::UnresolvedHostname`, `Net::OpenTimeout`,
`Errno::ECONNREFUSED`, `OpenSSL::SSL::SSLError`, `URI::InvalidURIError`, ...) so `PingDelivery`
rows look the same from either path; `retryable` mirrors the worker's `RETRYABLE_EXCEPTIONS` vs
`INTERNET_EXCEPTIONS` split.

Behaviour mirrors `SsrfFilter.post` with the worker's options: the hostname is resolved first,
addresses in the gem's IPv4/IPv6 reserved ranges (including IPv4-mapped/translated and NAT64
encodings) are refused, the request connects to the exact public IP that passed (Host/SNI keep
the hostname), the same POST body is re-sent on each of up to `MAX_REDIRECTS` hops, URL userinfo
is sent as basic auth and dropped on cross-origin redirects, and a 3xx past the limit is handed
back instead of raised.

## Configuration

| Variable              | Default       | Notes                                                                                                 |
| --------------------- | ------------- | ----------------------------------------------------------------------------------------------------- |
| `PORT`                | `8080`        |                                                                                                       |
| `APP_ENV`             | `development` |                                                                                                       |
| `LOG_LEVEL`           | `info`        | `debug`, `info`, `warn`, `error`; endpoint URLs and payloads are never logged                         |
| `INTERNAL_AUTH_TOKEN` | unset         | When set, `/v1/deliveries` requires a matching `X-Internal-Token`; required when `APP_ENV=production` |
| `OPEN_TIMEOUT`        | `5s`          | `REQUEST_TIMEOUT_SECONDS` (connect + TLS)                                                             |
| `READ_TIMEOUT`        | `5s`          | `REQUEST_TIMEOUT_SECONDS` (response headers)                                                          |
| `MAX_REDIRECTS`       | `3`           | `PostToIndividualPingEndpointWorker::MAX_REDIRECTS`                                                   |
| `SHUTDOWN_TIMEOUT`    | `20s`         | Drain window on SIGTERM                                                                               |

The service makes outbound connections to arbitrary public addresses by design; its egress
NetworkPolicy must still block the cluster's own CIDRs, since the address policy above is the
only other guard.

## Develop

```
make lint test          # gofmt + go vet + go test -race
make run                # http://localhost:8080
make docker-build       # ghcr.io/thiagodiniz/ping-delivery:<git sha>
make k8s-render OVERLAY=staging
```

Point Rails at a local instance with `PING_DELIVERY_SERVICE_URL=http://localhost:8080` and
`Feature.activate(:ping_delivery_service)`. `docker compose --profile ping-delivery up` publishes
it on `127.0.0.1:8082`.

## Deploy

`deploy/k8s` is a kustomize base with `staging` and `production` overlays (Deployment, Service,
ConfigMap, HPA, PDB, NetworkPolicy). Secrets are created out of band — see
`deploy/k8s/base/secret.example.yaml`. CI builds the image in `.github/workflows/go-services.yml`
and, on `main`, pushes it to `ghcr.io/thiagodiniz/ping-delivery:<sha>` with `GITHUB_TOKEN`; pin
the tag in the overlay (`kustomize edit set image gumroad/ping-delivery=ghcr.io/thiagodiniz/ping-delivery:<sha>`)
and `kubectl apply -k deploy/k8s/overlays/<env>`.
