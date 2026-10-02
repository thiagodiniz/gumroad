# frozen_string_literal: true

# Base URL of the Go oEmbed service (services/oembed). Leave unset to keep resolving embeds
# in-process through the ruby-oembed gem; see docs/microservices/README.md for the rollout.
OEMBED_SERVICE_URL = GlobalConfig.get("OEMBED_SERVICE_URL")
OEMBED_SERVICE_TOKEN = GlobalConfig.get("OEMBED_SERVICE_TOKEN")
