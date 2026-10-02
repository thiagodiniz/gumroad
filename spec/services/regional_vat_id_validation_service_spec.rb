# frozen_string_literal: true

require "spec_helper"

describe RegionalVatIdValidationService do
  describe "#process" do
    it "returns false for a blank id without calling any validator" do
      expect(VatValidationService).not_to receive(:new)

      expect(described_class.new("", country_code: "DE").process).to be(false)
    end

    describe "in-process dispatch" do
      before { allow(TaxIdValidationServiceClient).to receive(:enabled?).and_return(false) }

      [
        ["AU", nil, AbnValidationService],
        ["SG", nil, GstValidationService],
        ["CA", "QC", QstValidationService],
        ["NO", nil, MvaValidationService],
        ["BH", nil, TrnValidationService],
        ["KE", nil, KraPinValidationService],
        ["NG", nil, FirsTinValidationService],
        ["TZ", nil, TraTinValidationService],
        ["OM", nil, OmanVatNumberValidationService],
        ["DE", nil, VatValidationService],
        ["CA", "ON", VatValidationService],
      ].each do |country_code, state_code, validator|
        it "routes #{country_code}#{state_code ? "/#{state_code}" : ""} to #{validator}" do
          instance = instance_double(validator, process: true)
          expect(validator).to receive(:new).with("ID123").and_return(instance)

          expect(described_class.new("ID123", country_code:, state_code:).process).to be(true)
        end
      end

      it "routes Tax ID Pro countries with the country code" do
        instance = instance_double(TaxIdValidationService, process: true)
        expect(TaxIdValidationService).to receive(:new).with("ID123", "JP").and_return(instance)

        expect(described_class.new("ID123", country_code: "JP").process).to be(true)
      end
    end

    describe "delegation to the tax ID validation service" do
      let(:client) { instance_double(TaxIdValidationServiceClient) }

      before do
        allow(TaxIdValidationServiceClient).to receive(:enabled?).and_return(true)
        allow(TaxIdValidationServiceClient).to receive(:new).and_return(client)
      end

      it "returns the service's verdict without running the in-process validator" do
        expect(client).to receive(:validate).with("DE123", country_code: "DE", state_code: nil).and_return(false)
        expect(VatValidationService).not_to receive(:new)

        expect(described_class.new("DE123", country_code: "DE").process).to be(false)
      end

      it "falls back to the in-process validator when the service is unavailable" do
        expect(client).to receive(:validate).and_return(nil)
        instance = instance_double(VatValidationService, process: true)
        expect(VatValidationService).to receive(:new).with("DE123").and_return(instance)

        expect(described_class.new("DE123", country_code: "DE").process).to be(true)
      end
    end
  end
end
