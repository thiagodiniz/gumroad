package cache

import (
	"testing"
	"time"
)

func TestCacheExpiry(t *testing.T) {
	now := time.Unix(0, 0)
	c := New(10 * time.Minute)
	c.now = func() time.Time { return now }

	c.Set("k", true)
	if v, ok := c.Get("k"); !ok || !v {
		t.Fatal("fresh entry must be returned")
	}
	now = now.Add(10*time.Minute + time.Second)
	if _, ok := c.Get("k"); ok {
		t.Fatal("expired entry must be a miss")
	}
	c.Set("a", false)
	c.Set("b", false)
	now = now.Add(time.Hour)
	c.Sweep()
	if c.Len() != 0 {
		t.Fatalf("sweep left %d entries", c.Len())
	}
}

func TestCacheStoresFalse(t *testing.T) {
	c := New(time.Minute)
	c.Set("k", false)
	if v, ok := c.Get("k"); !ok || v {
		t.Fatalf("got %v, %v", v, ok)
	}
}
