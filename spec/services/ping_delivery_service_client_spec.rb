# frozen_string_literal: true

require "spec_helper"

describe PingDeliveryServiceClient do
  let(:base_url) { "http://ping-delivery.internal" }
  let(:client) { described_class.new(base_url:, token: "secret") }
  let(:endpoint) { "https://notification.com/webhook?token=endpoint-secret" }

  describe ".enabled?" do
    it "is false when the service URL is not configured" do
      stub_const("PING_DELIVERY_SERVICE_URL", nil)
      Feature.activate(described_class::FEATURE_FLAG)

      expect(described_class.enabled?).to be(false)
    end

    it "is false when the feature flag is off" do
      stub_const("PING_DELIVERY_SERVICE_URL", base_url)
      Feature.deactivate(described_class::FEATURE_FLAG)

      expect(described_class.enabled?).to be(false)
    end

    it "is true when the URL is configured and the flag is on" do
      stub_const("PING_DELIVERY_SERVICE_URL", base_url)
      Feature.activate(described_class::FEATURE_FLAG)

      expect(described_class.enabled?).to be(true)
    end
  end

  describe "#deliver" do
    def stub_service(status:, body:)
      WebMock.stub_request(:post, "#{base_url}/v1/deliveries")
        .to_return(status:, body: body.to_json, headers: { "Content-Type" => "application/json" })
    end

    def deliver
      client.deliver(url: endpoint, body: "license_key=license-secret", content_type: Mime[:url_encoded_form].to_s)
    end

    it "posts the encoded payload and returns the endpoint's status" do
      stub = WebMock.stub_request(:post, "#{base_url}/v1/deliveries")
        .with(
          body: { url: endpoint, body: "license_key=license-secret", content_type: Mime[:url_encoded_form].to_s }.to_json,
          headers: { "Content-Type" => "application/json", "X-Internal-Token" => "secret" },
        )
        .to_return(status: 200, body: { outcome: "responded", status: 500, retryable: false }.to_json, headers: { "Content-Type" => "application/json" })

      outcome = deliver

      expect(outcome.status).to eq(500)
      expect(outcome.error?).to be(false)
      expect(stub).to have_been_requested
    end

    it "returns the error class and retryability when the endpoint could not be reached" do
      stub_service(status: 200, body: { outcome: "error", error_class: "Net::ReadTimeout", retryable: true })

      outcome = deliver

      expect(outcome.error?).to be(true)
      expect(outcome.error_class).to eq("Net::ReadTimeout")
      expect(outcome.retryable).to be(true)
      expect(outcome.status).to be_nil
    end

    it "returns nil on non-200 responses so the worker can fall back" do
      stub_service(status: 503, body: { error: "unavailable" })

      expect(deliver).to be_nil
    end

    it "returns nil when a 200 response is malformed" do
      stub_service(status: 200, body: { outcome: "responded" })
      expect(deliver).to be_nil

      stub_service(status: 200, body: { outcome: "error", retryable: true })
      expect(deliver).to be_nil
    end

    it "returns nil on network errors" do
      WebMock.stub_request(:post, "#{base_url}/v1/deliveries").to_timeout

      expect(deliver).to be_nil
    end

    it "does not log the endpoint URL or payload" do
      stub_service(status: 503, body: { error: "unavailable" })
      messages = []
      allow(Rails.logger).to receive(:warn) { |message| messages << message }

      deliver

      expect(messages).not_to be_empty
      expect(messages.join("\n")).not_to include(endpoint, "endpoint-secret", "license-secret")
    end

    it "omits the auth header when no token is configured" do
      stub = WebMock.stub_request(:post, "#{base_url}/v1/deliveries")
        .with { |request| !request.headers.key?("X-Internal-Token") }
        .to_return(status: 200, body: { outcome: "responded", status: 200 }.to_json, headers: { "Content-Type" => "application/json" })

      described_class.new(base_url:, token: nil).deliver(url: endpoint, body: "a=1", content_type: Mime[:json].to_s)
      expect(stub).to have_been_requested
    end
  end
end
