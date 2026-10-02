// Package config reads the service configuration from the environment. Names match the
// Rails GlobalConfig keys (VATSTACK_API_KEY, IRAS_API_ID, ...) so the same secret can be
// mounted into both deployments during the migration.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Addr        string
	Environment string
	LogLevel    string

	// Shared secret the Rails client sends as X-Internal-Token. Empty disables the check,
	// which is only acceptable when a NetworkPolicy restricts callers.
	InternalToken string

	CacheTTL        time.Duration
	UpstreamTimeout time.Duration
	ViesTimeout     time.Duration
	ShutdownTimeout time.Duration

	VatstackURL    string
	VatstackAPIKey string

	IrasURL      string
	IrasClientID string
	IrasSecret   string

	TaxIDProURL    string
	TaxIDProAPIKey string

	RevenuQuebecURL string

	ViesURL               string
	VatRegistrationNumber string
}

func Load() (Config, error) {
	c := Config{
		Addr:                  ":" + getenv("PORT", "8080"),
		Environment:           getenv("APP_ENV", "development"),
		LogLevel:              getenv("LOG_LEVEL", "info"),
		InternalToken:         os.Getenv("INTERNAL_AUTH_TOKEN"),
		VatstackURL:           getenv("VATSTACK_URL", "https://api.vatstack.com/v1/validations"),
		VatstackAPIKey:        os.Getenv("VATSTACK_API_KEY"),
		IrasClientID:          os.Getenv("IRAS_API_ID"),
		IrasSecret:            os.Getenv("IRAS_API_SECRET"),
		TaxIDProURL:           getenv("TAX_ID_PRO_URL", "https://v3.api.taxid.pro/validate"),
		TaxIDProAPIKey:        os.Getenv("TAX_ID_PRO_API_KEY"),
		RevenuQuebecURL:       getenv("REVENU_QUEBEC_URL", "https://svcnab2b.revenuquebec.ca/2019/02/ValidationTVQ/"),
		ViesURL:               getenv("VIES_URL", "https://ec.europa.eu/taxation_customs/vies/rest-api/check-vat-number"),
		VatRegistrationNumber: getenv("VAT_REGISTRATION_NUMBER", "EU372030009"),
	}

	// IRAS hands out separate sandbox and production credentials; same split as
	// config/initializers/iras.rb.
	defaultIras := "https://apisandbox.iras.gov.sg/iras/sb/GSTListing/SearchGSTRegistered"
	if c.Environment == "production" {
		defaultIras = "https://apiservices.iras.gov.sg/iras/prod/GSTListing/SearchGSTRegistered"
	}
	c.IrasURL = getenv("IRAS_ENDPOINT", defaultIras)

	var err error
	if c.CacheTTL, err = duration("CACHE_TTL", 10*time.Minute); err != nil {
		return c, err
	}
	if c.UpstreamTimeout, err = duration("UPSTREAM_TIMEOUT", 5*time.Second); err != nil {
		return c, err
	}
	if c.ViesTimeout, err = duration("VIES_TIMEOUT", 30*time.Second); err != nil {
		return c, err
	}
	if c.ShutdownTimeout, err = duration("SHUTDOWN_TIMEOUT", 20*time.Second); err != nil {
		return c, err
	}
	return c, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// duration accepts Go durations ("30s", "10m") or a bare number of seconds.
func duration(key string, def time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	if secs, err := strconv.Atoi(v); err == nil {
		return time.Duration(secs) * time.Second, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return d, nil
}
