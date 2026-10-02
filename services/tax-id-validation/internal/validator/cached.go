package validator

import "context"

// Store is the subset of the cache the decorators need.
type Store interface {
	Get(key string) (value bool, ok bool)
	Set(key string, value bool)
}

// Cached memoizes successful verdicts of a network-backed validator. Errors are never
// cached, so a registry outage does not pin an "invalid" for the TTL. Mirrors the
// Rails.cache.fetch wrapping in the Ruby services (keyed per validator, not globally).
type Cached struct {
	inner Validator
	store Store
}

func NewCached(inner Validator, store Store) *Cached { return &Cached{inner: inner, store: store} }

func (c *Cached) Name() string { return c.inner.Name() }

func (c *Cached) Validate(ctx context.Context, id string) (bool, error) {
	key := c.inner.Name() + ":" + id
	if v, ok := c.store.Get(key); ok {
		return v, nil
	}
	v, err := c.inner.Validate(ctx, id)
	if err != nil {
		return false, err
	}
	c.store.Set(key, v)
	return v, nil
}

// CachedCountry is Cached for the taxid.pro shape.
type CachedCountry struct {
	inner CountryValidator
	store Store
}

func NewCachedCountry(inner CountryValidator, store Store) *CachedCountry {
	return &CachedCountry{inner: inner, store: store}
}

func (c *CachedCountry) Name() string { return c.inner.Name() }

func (c *CachedCountry) Validate(ctx context.Context, id, countryCode string) (bool, error) {
	key := c.inner.Name() + ":" + countryCode + ":" + id
	if v, ok := c.store.Get(key); ok {
		return v, nil
	}
	v, err := c.inner.Validate(ctx, id, countryCode)
	if err != nil {
		return false, err
	}
	c.store.Set(key, v)
	return v, nil
}
