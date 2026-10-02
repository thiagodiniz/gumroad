# frozen_string_literal: true

# HTTP client for the Go ping delivery service (services/ping-delivery), which performs the
# SSRF-filtered POST (with redirects) that PostToIndividualPingEndpointWorker otherwise does
# in-process via SsrfFilter.post. Encoding, PingDelivery records and retry scheduling stay here.
#
# `deliver` returns an Outcome — either the endpoint's final HTTP status, or the Ruby exception
# class the in-process path would have raised plus whether it is retryable — and nil when the
# service itself could not be reached, so the worker falls back to SsrfFilter.post.
class PingDeliveryServiceClient
  FEATURE_FLAG = :ping_delivery_service
  # The service may spend (open + read timeout) × (redirects + 1) = 40s on one delivery.
  TIMEOUT_SECONDS = 45
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

  Outcome = Struct.new(:status, :error_class, :retryable, keyword_init: true) do
    def error? = error_class.present?
  end

  def self.enabled?
    PING_DELIVERY_SERVICE_URL.present? && Feature.active?(FEATURE_FLAG)
  end

  def initialize(base_url: PING_DELIVERY_SERVICE_URL, token: PING_DELIVERY_SERVICE_TOKEN)
    @base_url = base_url
    @token = token
  end

  def deliver(url:, body:, content_type:)
    response = HTTParty.post(
      "#{@base_url.to_s.chomp("/")}/v1/deliveries",
      body: { url:, body:, content_type: }.to_json,
      headers:,
      timeout: TIMEOUT_SECONDS,
    )

    # Endpoint URLs and payloads carry license keys and buyer emails, so they are never logged.
    unless response.code == 200
      Rails.logger.warn("PingDeliveryServiceClient: HTTP #{response.code}")
      return nil
    end

    payload = response.parsed_response
    if payload.is_a?(Hash)
      case payload["outcome"]
      when "responded"
        return Outcome.new(status: payload["status"]) if payload["status"].is_a?(Integer)
      when "error"
        if payload["error_class"].is_a?(String) && payload["error_class"].present?
          return Outcome.new(error_class: payload["error_class"], retryable: payload["retryable"] == true)
        end
      end
    end

    Rails.logger.warn("PingDeliveryServiceClient: malformed response")
    nil
  rescue *NETWORK_ERRORS => e
    Rails.logger.warn("PingDeliveryServiceClient: #{e.class}: #{e.message}")
    nil
  end

  private
    def headers
      base = { "Content-Type" => "application/json", "Accept" => "application/json" }
      @token.present? ? base.merge("X-Internal-Token" => @token) : base
    end
end
