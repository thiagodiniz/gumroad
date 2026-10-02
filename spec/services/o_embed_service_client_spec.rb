# frozen_string_literal: true

require "spec_helper"

describe OEmbedServiceClient do
  let(:base_url) { "http://oembed.internal" }
  let(:client) { described_class.new(base_url:, token: "secret") }
  let(:url) { "https://vimeo.com/71588076" }

  describe ".enabled?" do
    it "is false when the service URL is not configured" do
      stub_const("OEMBED_SERVICE_URL", nil)
      Feature.activate(described_class::FEATURE_FLAG)

      expect(described_class.enabled?).to be(false)
    end

    it "is false when the feature flag is off" do
      stub_const("OEMBED_SERVICE_URL", base_url)
      Feature.deactivate(described_class::FEATURE_FLAG)

      expect(described_class.enabled?).to be(false)
    end

    it "is true when the URL is configured and the flag is on" do
      stub_const("OEMBED_SERVICE_URL", base_url)
      Feature.activate(described_class::FEATURE_FLAG)

      expect(described_class.enabled?).to be(true)
    end
  end

  describe "#lookup" do
    def stub_service(status:, body:)
      WebMock.stub_request(:post, "#{base_url}/v1/embeds")
        .to_return(status:, body: body.to_json, headers: { "Content-Type" => "application/json" })
    end

    it "posts the request and returns the embeddable in OEmbedFinder's shape" do
      stub = WebMock.stub_request(:post, "#{base_url}/v1/embeds")
        .with(
          body: { url:, maxwidth: 670 }.to_json,
          headers: { "Content-Type" => "application/json", "X-Internal-Token" => "secret" },
        )
        .to_return(
          status: 200,
          body: { embeddable: { html: "<iframe></iframe>", info: { width: 670, height: 377, thumbnail_url: "https://i.vimeocdn.com/t.jpg" } } }.to_json,
          headers: { "Content-Type" => "application/json" },
        )

      result = client.lookup(url, maxwidth: 670)

      expect(result.embeddable).to eq(
        html: "<iframe></iframe>",
        info: { "width" => 670, "height" => 377, "thumbnail_url" => "https://i.vimeocdn.com/t.jpg" },
      )
      expect(stub).to have_been_requested
    end

    it "returns a Result with a nil embeddable when the service says the URL is not embeddable" do
      stub_service(status: 200, body: { embeddable: nil })

      result = client.lookup(url, maxwidth: 670)

      expect(result).not_to be_nil
      expect(result.embeddable).to be_nil
    end

    it "returns nil on non-200 responses so the caller can fall back" do
      stub_service(status: 502, body: { error: "upstream_unavailable" })

      expect(client.lookup(url, maxwidth: 670)).to be_nil
    end

    it "returns nil when a 200 response is malformed" do
      stub_service(status: 200, body: { embeddable: { html: 1 } })
      expect(client.lookup(url, maxwidth: 670)).to be_nil

      stub_service(status: 200, body: { something: "else" })
      expect(client.lookup(url, maxwidth: 670)).to be_nil
    end

    it "returns nil on network errors" do
      WebMock.stub_request(:post, "#{base_url}/v1/embeds").to_timeout

      expect(client.lookup(url, maxwidth: 670)).to be_nil
    end

    it "omits the auth header when no token is configured" do
      stub = WebMock.stub_request(:post, "#{base_url}/v1/embeds")
        .with { |request| !request.headers.key?("X-Internal-Token") }
        .to_return(status: 200, body: { embeddable: nil }.to_json, headers: { "Content-Type" => "application/json" })

      described_class.new(base_url:, token: nil).lookup(url, maxwidth: 670)
      expect(stub).to have_been_requested
    end
  end
end
