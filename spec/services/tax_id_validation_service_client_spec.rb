# frozen_string_literal: true

require "spec_helper"

describe TaxIdValidationServiceClient do
  let(:base_url) { "http://tax-id-validation.internal" }
  let(:client) { described_class.new(base_url:, token: "secret") }

  describe ".enabled?" do
    it "is false when the service URL is not configured" do
      stub_const("TAX_ID_VALIDATION_SERVICE_URL", nil)
      Feature.activate(described_class::FEATURE_FLAG)

      expect(described_class.enabled?).to be(false)
    end

    it "is false when the feature flag is off" do
      stub_const("TAX_ID_VALIDATION_SERVICE_URL", base_url)
      Feature.deactivate(described_class::FEATURE_FLAG)

      expect(described_class.enabled?).to be(false)
    end

    it "is true when the URL is configured and the flag is on" do
      stub_const("TAX_ID_VALIDATION_SERVICE_URL", base_url)
      Feature.activate(described_class::FEATURE_FLAG)

      expect(described_class.enabled?).to be(true)
    end
  end

  describe "#validate" do
    it "posts the request and returns the service's verdict" do
      stub = WebMock.stub_request(:post, "#{base_url}/v1/validations")
        .with(
          body: { tax_id: "DE123456789", country_code: "DE", state_code: nil }.to_json,
          headers: { "Content-Type" => "application/json", "X-Internal-Token" => "secret" },
        )
        .to_return(status: 200, body: { valid: true, validator: "eu_vat" }.to_json, headers: { "Content-Type" => "application/json" })

      expect(client.validate("DE123456789", country_code: "DE")).to be(true)
      expect(stub).to have_been_requested
    end

    it "returns false when the service says the id is invalid" do
      WebMock.stub_request(:post, "#{base_url}/v1/validations")
        .to_return(status: 200, body: { valid: false, validator: "abn" }.to_json, headers: { "Content-Type" => "application/json" })

      expect(client.validate("1234", country_code: "AU")).to be(false)
    end

    it "returns nil on non-200 responses so the caller can fall back" do
      WebMock.stub_request(:post, "#{base_url}/v1/validations")
        .to_return(status: 502, body: { error: "upstream_unavailable" }.to_json, headers: { "Content-Type" => "application/json" })

      expect(client.validate("1234", country_code: "AU")).to be_nil
    end

    it "returns nil when a 200 response has no boolean verdict" do
      WebMock.stub_request(:post, "#{base_url}/v1/validations")
        .to_return(status: 200, body: { validator: "abn" }.to_json, headers: { "Content-Type" => "application/json" })

      expect(client.validate("1234", country_code: "AU")).to be_nil
    end

    it "returns nil on network errors" do
      WebMock.stub_request(:post, "#{base_url}/v1/validations").to_timeout

      expect(client.validate("1234", country_code: "AU")).to be_nil
    end

    it "omits the auth header when no token is configured" do
      stub = WebMock.stub_request(:post, "#{base_url}/v1/validations")
        .with { |request| !request.headers.key?("X-Internal-Token") }
        .to_return(status: 200, body: { valid: true }.to_json, headers: { "Content-Type" => "application/json" })

      described_class.new(base_url:, token: nil).validate("1234", country_code: "AU")
      expect(stub).to have_been_requested
    end
  end
end
