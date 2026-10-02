package ssrf

import (
	"context"
	"errors"
	"net/netip"
	"testing"
)

func TestUnsafe(t *testing.T) {
	unsafe := []string{
		"127.0.0.1", "10.0.0.5", "172.16.3.4", "192.168.1.1", "169.254.169.254", "100.64.0.1",
		"0.0.0.0", "224.0.0.1", "255.255.255.255", "198.18.0.1",
		"::1", "fe80::1", "fc00::1", "fd12::1", "ff02::1", "2001:db8::1", "2002:c000:201::1",
		"::ffff:127.0.0.1", "::ffff:10.1.2.3", "::10.0.0.1", "::ffff:0:10.0.0.1", "64:ff9b::10.0.0.1",
		"64:ff9b:1:a00:0:100::", // RFC 6052 /48: 10.0.0.1 split around the u-bits in byte 8
	}
	safe := []string{"93.184.216.34", "8.8.8.8", "2606:4700::6810:84e5", "::ffff:93.184.216.34", "64:ff9b::808:808", "64:ff9b:1:5db8:d8:2200::"}
	for _, s := range unsafe {
		if !Unsafe(netip.MustParseAddr(s)) {
			t.Errorf("%s should be unsafe", s)
		}
	}
	for _, s := range safe {
		if Unsafe(netip.MustParseAddr(s)) {
			t.Errorf("%s should be safe", s)
		}
	}
}

func TestPublicAddress(t *testing.T) {
	fixed := func(answers ...string) Resolver {
		return func(context.Context, string) ([]netip.Addr, error) {
			out := []netip.Addr{}
			for _, a := range answers {
				out = append(out, netip.MustParseAddr(a))
			}
			return out, nil
		}
	}
	if _, err := PublicAddress(context.Background(), fixed(), "x"); !errors.Is(err, ErrUnresolvedHostname) {
		t.Errorf("empty: %v", err)
	}
	if _, err := PublicAddress(context.Background(), fixed("10.0.0.5", "127.0.0.1"), "x"); !errors.Is(err, ErrPrivateIPAddress) {
		t.Errorf("private only: %v", err)
	}
	a, err := PublicAddress(context.Background(), fixed("10.0.0.5", "93.184.216.34"), "x")
	if err != nil || a.String() != "93.184.216.34" {
		t.Errorf("mixed: %v %v", a, err)
	}
	if a, err := DefaultResolver(context.Background(), "93.184.216.34"); err != nil || len(a) != 1 {
		t.Errorf("literal: %v %v", a, err)
	}
	if a, err := DefaultResolver(context.Background(), "does-not-exist.invalid"); err != nil || len(a) != 0 {
		t.Errorf("nxdomain should be empty, not an error: %v %v", a, err)
	}
}
