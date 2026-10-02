# frozen_string_literal: true

# Base URL of the Go ping delivery service (services/ping-delivery). Leave unset to keep
# posting webhooks in-process through SsrfFilter; see docs/microservices/README.md for the rollout.
PING_DELIVERY_SERVICE_URL = GlobalConfig.get("PING_DELIVERY_SERVICE_URL")
PING_DELIVERY_SERVICE_TOKEN = GlobalConfig.get("PING_DELIVERY_SERVICE_TOKEN")
