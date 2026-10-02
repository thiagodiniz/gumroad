# oembed

Go port of `OEmbedFinder` (`lib/utilities/o_embed_finder.rb`) and the oEmbed provider registry
that `config/initializers/oembed.rb` builds on the ruby-oembed gem (built-in providers plus
Wistia, Sketchfab and FrameRate). Rails calls it through `OEmbedServiceClient` and falls back
to the in-process gem whenever the service does not answer.

## API

```
POST /v1/embeds
X-Internal-Token: <INTERNAL_AUTH_TOKEN>          # only when the token is configured
{"url": "https://vimeo.com/71588076", "maxwidth": 670}

200 {"embeddable": {"html": "<iframe ...>", "info": {"width": 670, "height": 377, "thumbnail_url": "..."}}}
200 {"embeddable": null}                          # no provider, provider rejected the URL, or photo/link
400 {"error": "invalid_request"}                  # malformed JSON / url is not http(s)
401 {"error": "unauthorized"}
502 {"error": "upstream_unavailable"}             # provider unreachable or 5xx; Rails falls back

GET /healthz   liveness
GET /readyz    readiness
```

Behaviour mirrors the Ruby finder: providers are matched in the gem's registration order
(first pattern wins; Facebook/Instagram are skipped because Rails never configures their
access token), the provider is queried with `format=json&maxwidth=<n>&url=<url>` following at
most 4 redirects, only `video`/`rich` responses are embeddable, SoundCloud/YouTube/Wistia/
Sketchfab/Vimeo HTML is rewritten exactly as `OEmbedFinder` does, and `info` carries only
`width`, `height` and `thumbnail_url` as the provider sent them. `maxwidth` defaults to
`AssetPreview::DEFAULT_DISPLAY_WIDTH` (670).

## Configuration

| Variable              | Default       | Notes                                                                                             |
| --------------------- | ------------- | ------------------------------------------------------------------------------------------------- |
| `PORT`                | `8080`        |                                                                                                   |
| `APP_ENV`             | `development` |                                                                                                   |
| `LOG_LEVEL`           | `info`        | `debug`, `info`, `warn`, `error`                                                                  |
| `INTERNAL_AUTH_TOKEN` | unset         | When set, `/v1/embeds` requires a matching `X-Internal-Token`; required when `APP_ENV=production` |
| `UPSTREAM_TIMEOUT`    | `10s`         | Per provider request (the gem used Net::HTTP's 60s default)                                       |
| `MAX_REDIRECTS`       | `4`           | Same as `OEmbed::HttpHelper`                                                                      |
| `SHUTDOWN_TIMEOUT`    | `20s`         | Drain window on SIGTERM                                                                           |

No provider credentials are needed; all endpoints are public.

## Develop

```
make lint test          # gofmt + go vet + go test -race
make run                # http://localhost:8080
make docker-build       # ghcr.io/thiagodiniz/oembed:<git sha>
make k8s-render OVERLAY=staging
```

Point Rails at a local instance with `OEMBED_SERVICE_URL=http://localhost:8080` and
`Feature.activate(:oembed_service)`. `docker compose --profile oembed up` publishes it on
`127.0.0.1:8081`.

## Deploy

`deploy/k8s` is a kustomize base with `staging` and `production` overlays (Deployment, Service,
ConfigMap, HPA, PDB, NetworkPolicy). Secrets are created out of band — see
`deploy/k8s/base/secret.example.yaml`. CI builds the image in `.github/workflows/go-services.yml`
and, on `main`, pushes it to `ghcr.io/thiagodiniz/oembed:<sha>` with `GITHUB_TOKEN`; pin the tag
in the overlay (`kustomize edit set image gumroad/oembed=ghcr.io/thiagodiniz/oembed:<sha>`) and
`kubectl apply -k deploy/k8s/overlays/<env>`.
