package config

import (
	"net/netip"
	"reflect"
	"testing"
)

func TestParsePrefixes(t *testing.T) {
	got, err := parsePrefixes(" 10.0.0.0/8, 192.168.49.2 ,, fd00::1/64")
	if err != nil {
		t.Fatalf("parsePrefixes() error = %v", err)
	}
	want := []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("192.168.49.2/32"),
		netip.MustParsePrefix("fd00::/64"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parsePrefixes() = %v, want %v", got, want)
	}

	if got, err := parsePrefixes(""); err != nil || len(got) != 0 {
		t.Fatalf("empty list = %v, %v", got, err)
	}
	for _, bad := range []string{"10.0.0.0/33", "proxy.local"} {
		if _, err := parsePrefixes(bad); err == nil {
			t.Fatalf("parsePrefixes(%q) accepted an invalid entry", bad)
		}
	}
}
