# frozen_string_literal: true

# HTTP client for the Go oEmbed service (services/oembed), which ports OEmbedFinder and the
# provider registry from config/initializers/oembed.rb.
#
# `lookup` returns a Result whose `embeddable` is the `{ html:, info: }` hash OEmbedFinder
# would build, or nil when the URL is not embeddable. It returns nil (no Result) when the
# service could not be reached or failed upstream, so the caller falls back to the gem.
class OEmbedServiceClient
  FEATURE_FLAG = :oembed_service
  # The service gives providers UPSTREAM_TIMEOUT (10s) and follows up to 4 redirects.
  TIMEOUT_SECONDS = 15
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

  Result = Struct.new(:embeddable)

  def self.enabled?
    OEMBED_SERVICE_URL.present? && Feature.active?(FEATURE_FLAG)
  end

  def initialize(base_url: OEMBED_SERVICE_URL, token: OEMBED_SERVICE_TOKEN)
    @base_url = base_url
    @token = token
  end

  def lookup(url, maxwidth:)
    response = HTTParty.post(
      "#{@base_url.to_s.chomp("/")}/v1/embeds",
      body: { url:, maxwidth: }.to_json,
      headers:,
      timeout: TIMEOUT_SECONDS,
    )

    unless response.code == 200
      Rails.logger.warn("OEmbedServiceClient: HTTP #{response.code} for url=#{url} body=#{response.body.to_s.truncate(200)}")
      return nil
    end

    payload = response.parsed_response
    if payload.is_a?(Hash) && payload.key?("embeddable")
      embeddable = payload["embeddable"]
      return Result.new(nil) if embeddable.nil?
      if embeddable.is_a?(Hash) && embeddable["html"].is_a?(String) && embeddable["info"].is_a?(Hash)
        return Result.new({ html: embeddable["html"], info: embeddable["info"] })
      end
    end

    Rails.logger.warn("OEmbedServiceClient: malformed response for url=#{url} body=#{response.body.to_s.truncate(200)}")
    nil
  rescue *NETWORK_ERRORS => e
    Rails.logger.warn("OEmbedServiceClient: #{e.class}: #{e.message}")
    nil
  end

  private
    def headers
      base = { "Content-Type" => "application/json", "Accept" => "application/json" }
      @token.present? ? base.merge("X-Internal-Token" => @token) : base
    end
end
