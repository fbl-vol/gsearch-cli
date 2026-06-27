package main

import (
	"testing"
	"time"
)

func TestParseInterspersedFlagsAfterPositionals(t *testing.T) {
	fs := newFlagSet("test")
	options := addSearchFlags(fs, 4326)
	compact := fs.Bool("compact", false, "emit compact JSON")

	positionals, err := parseInterspersed(fs, []string{"husnummer", "genvej", "--limit", "3", "--compact"})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := options.Limit, 3; got != want {
		t.Fatalf("limit = %d, want %d", got, want)
	}
	if !*compact {
		t.Fatal("compact flag was not parsed")
	}
	if got, want := len(positionals), 2; got != want {
		t.Fatalf("positionals length = %d, want %d", got, want)
	}
	if positionals[0] != "husnummer" || positionals[1] != "genvej" {
		t.Fatalf("unexpected positionals: %#v", positionals)
	}
}

func TestParseInterspersedDurationFlag(t *testing.T) {
	fs := newFlagSet("test")
	options := addSearchFlags(fs, 4326)

	positionals, err := parseInterspersed(fs, []string{"doctor", "--timeout", "30s"})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := options.Timeout, 30*time.Second; got != want {
		t.Fatalf("timeout = %s, want %s", got, want)
	}
	if got, want := positionals[0], "doctor"; got != want {
		t.Fatalf("positional = %q, want %q", got, want)
	}
}

func TestRedactToken(t *testing.T) {
	message := `Get "https://api.dataforsyningen.dk/rest/gsearch/v2.0/husnummer?limit=3&token=abc&q=x": EOF`
	redacted := redactToken(message)
	if redacted == message {
		t.Fatal("message was not redacted")
	}
	if contains(redacted, "abc") {
		t.Fatalf("redacted message still contains token: %s", redacted)
	}
}

func TestFirstPositionNestedGeoJSON(t *testing.T) {
	position, ok := firstPosition([]any{[]any{[]any{12.3, 55.6}}})
	if !ok {
		t.Fatal("expected nested position")
	}
	if position[0] != 12.3 || position[1] != 55.6 {
		t.Fatalf("position = %#v", position)
	}
}

func TestParseRadii(t *testing.T) {
	radii, err := parseRadii("100, 300,1000")
	if err != nil {
		t.Fatal(err)
	}
	if len(radii) != 3 || radii[0] != 100 || radii[2] != 1000 {
		t.Fatalf("unexpected radii: %#v", radii)
	}
}

func contains(s string, needle string) bool {
	for i := 0; i+len(needle) <= len(s); i++ {
		if s[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
