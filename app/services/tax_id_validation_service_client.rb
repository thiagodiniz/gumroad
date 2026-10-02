# frozen_string_literal: true

# HTTP client for the Go tax ID validation service (services/tax-id-validation), which
# ports RegionalVatIdValidationService and the registry-specific validators behind it.
#
# `validate` returns true/false when the service answered, and nil when it could not be
# reached or failed upstream, so the caller can fall back to the in-process validators.
class TaxIdValidationServiceClient
  FEATURE_FLAG = :tax_id_validation_service
  # The service applies its own per-registry timeouts (5s, 30s for VIES) and degrades to
  # syntax validation when VIES is slow, so the client only needs a little headroom.
  TIMEOUT_SECONDS = 35
  NETWORK_ERRORS = [
    Errno::ECONNREFUSED,
    Errno::ECONNRESET,
    Errno::EHOSTUNREACH,
    Net::OpenTimeout,
    Net::ReadTimeout,
    SocketError,
    HTTParty::Error,
    OpenSSL::SSL::SSLError,
  ].freeze

  def self.enabled?
    TAX_ID_VALIDATION_SERVICE_URL.present? && Feature.active?(FEATURE_FLAG)
  end

  def initialize(base_url: TAX_ID_VALIDATION_SERVICE_URL, token: TAX_ID_VALIDATION_SERVICE_TOKEN)
    @base_url = base_url
    @token = token
  end

  def validate(tax_id, country_code: nil, state_code: nil)
    response = HTTParty.post(
      "#{@base_url.to_s.chomp("/")}/v1/validations",
      body: { tax_id:, country_code:, state_code: }.to_json,
      headers:,
      timeout: TIMEOUT_SECONDS,
    )

    unless response.code == 200
      Rails.logger.warn("TaxIdValidationServiceClient: HTTP #{response.code} for country=#{country_code} body=#{response.body.to_s.truncate(200)}")
      return nil
    end

    verdict = response.parsed_response.is_a?(Hash) ? response.parsed_response["valid"] : nil
    return verdict if verdict == true || verdict == false

    Rails.logger.warn("TaxIdValidationServiceClient: malformed response for country=#{country_code} body=#{response.body.to_s.truncate(200)}")
    nil
  rescue *NETWORK_ERRORS => e
    Rails.logger.warn("TaxIdValidationServiceClient: #{e.class}: #{e.message}")
    nil
  end

  private
    def headers
      base = { "Content-Type" => "application/json", "Accept" => "application/json" }
      @token.present? ? base.merge("X-Internal-Token" => @token) : base
    end
end
