// Package ssrf ports the address policy of the ssrf_filter gem (1.5.0): a hostname is
// resolved up front, every address is checked against the reserved ranges, and the caller
// connects to the exact public IP that passed, closing the DNS-rebinding window between
// validate and connect.
package ssrf

import (
	"context"
	"errors"
	"math/rand/v2"
	"net"
	"net/netip"
)

// Error classes carry the Ruby exception names so PingDelivery rows written from either
// path look the same.
var (
	ErrUnresolvedHostname = errors.New("SsrfFilter::UnresolvedHostname")
	ErrPrivateIPAddress   = errors.New("SsrfFilter::PrivateIPAddress")
	ErrInvalidUriScheme   = errors.New("SsrfFilter::InvalidUriScheme")
)

var ipv4Blacklist = mustPrefixes(
	"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16",
	"172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24", "192.168.0.0/16", "198.18.0.0/15",
	"198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4", "255.255.255.255/32",
)

var ipv6Blacklist = mustPrefixes(
	"::1/128", "100::/64", "2001::/32", "2001:10::/28", "2001:20::/28", "2001:db8::/32",
	"2002::/16", "fc00::/7", "fe80::/10", "ff00::/8",
)

// IPv6 prefixes that embed an IPv4 address in their last 32 bits (RFC 4291 compatible and
// mapped, RFC 2765 translated, RFC 6052 NAT64 well-known). The embedded address is judged by
// the IPv4 list.
var ipv4EmbeddingPrefixes = mustPrefixes("::/96", "::ffff:0:0/96", "::ffff:0:0:0/96", "64:ff9b::/96")

// RFC 8215 local-use NAT64 prefix with RFC 6052 /48 encoding: the IPv4 address is split
// around the u-bits at bits 64-71.
var nat64LocalPrefix = netip.MustParsePrefix("64:ff9b:1::/48")

func mustPrefixes(cidrs ...string) []netip.Prefix {
	out := make([]netip.Prefix, len(cidrs))
	for i, c := range cidrs {
		out[i] = netip.MustParsePrefix(c)
	}
	return out
}

func inAny(prefixes []netip.Prefix, a netip.Addr) bool {
	for _, p := range prefixes {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// Unsafe reports whether a may not be connected to.
func Unsafe(a netip.Addr) bool {
	if a.Is4() {
		return inAny(ipv4Blacklist, a)
	}
	if a.Is4In6() {
		// Go treats ::ffff:a.b.c.d as the embedded IPv4 address; so does the gem's mapped check.
		return inAny(ipv4Blacklist, a.Unmap())
	}
	if !a.Is6() {
		return true
	}
	if inAny(ipv6Blacklist, a) {
		return true
	}
	b := a.As16()
	if inAny(ipv4EmbeddingPrefixes, a) {
		return inAny(ipv4Blacklist, netip.AddrFrom4([4]byte{b[12], b[13], b[14], b[15]}))
	}
	if nat64LocalPrefix.Contains(a) {
		return Unsafe(netip.AddrFrom4([4]byte{b[6], b[7], b[9], b[10]}))
	}
	return false
}

// Resolver looks a hostname up; it exists so tests can pin answers.
type Resolver func(ctx context.Context, host string) ([]netip.Addr, error)

func DefaultResolver(ctx context.Context, host string) ([]netip.Addr, error) {
	if a, err := netip.ParseAddr(host); err == nil {
		return []netip.Addr{a}, nil
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		// Resolv.getaddresses returns [] for every failure mode, and the gem turns that into
		// UnresolvedHostname (retryable), so NXDOMAIN and SERVFAIL are deliberately not told apart.
		return nil, nil
	}
	return ips, nil
}

// PublicAddress resolves host and returns one of its public addresses at random, like
// `public_addresses.sample`.
func PublicAddress(ctx context.Context, resolve Resolver, host string) (netip.Addr, error) {
	addrs, err := resolve(ctx, host)
	if err != nil || len(addrs) == 0 {
		return netip.Addr{}, ErrUnresolvedHostname
	}
	public := addrs[:0:0]
	for _, a := range addrs {
		if !Unsafe(a) {
			public = append(public, a)
		}
	}
	if len(public) == 0 {
		return netip.Addr{}, ErrPrivateIPAddress
	}
	return public[rand.IntN(len(public))], nil
}
