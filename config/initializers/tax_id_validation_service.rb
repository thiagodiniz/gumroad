# frozen_string_literal: true

# Base URL of the Go tax ID validation service (services/tax-id-validation). Leave unset to
# keep validating in-process; see docs/microservices/README.md for the rollout sequence.
TAX_ID_VALIDATION_SERVICE_URL = GlobalConfig.get("TAX_ID_VALIDATION_SERVICE_URL")
TAX_ID_VALIDATION_SERVICE_TOKEN = GlobalConfig.get("TAX_ID_VALIDATION_SERVICE_TOKEN")
