package validator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/antiwork/gumroad/services/tax-id-validation/internal/cache"
)

func TestCached(t *testing.T) {
	inner := &stub{name: "abn", valid: true}
	c := NewCached(inner, cache.New(time.Minute))
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if ok, err := c.Validate(ctx, "1"); !ok || err != nil {
			t.Fatalf("got %v, %v", ok, err)
		}
	}
	if inner.calls != 1 {
		t.Fatalf("inner called %d times, want 1", inner.calls)
	}
	if _, _ = c.Validate(ctx, "2"); inner.calls != 2 {
		t.Fatal("different ids must not share an entry")
	}
}

func TestCachedDoesNotStoreErrors(t *testing.T) {
	inner := &stub{name: "abn", err: ErrUpstream}
	c := NewCached(inner, cache.New(time.Minute))
	ctx := context.Background()
	if _, err := c.Validate(ctx, "1"); !errors.Is(err, ErrUpstream) {
		t.Fatalf("err = %v", err)
	}
	inner.err = nil
	inner.valid = true
	if ok, err := c.Validate(ctx, "1"); !ok || err != nil {
		t.Fatalf("after recovery got %v, %v", ok, err)
	}
	if inner.calls != 2 {
		t.Fatalf("inner called %d times, want 2", inner.calls)
	}
}

func TestCachedCountryKeysByCountry(t *testing.T) {
	inner := &countryStub{stub: stub{name: "tax_id_pro", valid: true}}
	c := NewCachedCountry(inner, cache.New(time.Minute))
	ctx := context.Background()
	_, _ = c.Validate(ctx, "1", "JP")
	_, _ = c.Validate(ctx, "1", "JP")
	_, _ = c.Validate(ctx, "1", "IN")
	if inner.calls != 2 {
		t.Fatalf("inner called %d times, want 2", inner.calls)
	}
}
