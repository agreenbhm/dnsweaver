package powerdns

import (
	"testing"

	"gitlab.bluewillows.net/root/dnsweaver/pkg/provider"
)

func TestCanonicalizeAndStrip(t *testing.T) {
	if got := canonicalize("app.example.com"); got != "app.example.com." {
		t.Errorf("canonicalize = %q", got)
	}
	if got := canonicalize("app.example.com."); got != "app.example.com." {
		t.Errorf("canonicalize idempotent failed: %q", got)
	}
	if got := stripDot("app.example.com."); got != "app.example.com" {
		t.Errorf("stripDot = %q", got)
	}
	if got := stripDot("app.example.com"); got != "app.example.com" {
		t.Errorf("stripDot no-dot failed: %q", got)
	}
}

func TestTXTQuotingRoundTrip(t *testing.T) {
	cases := []string{
		"heritage=dnsweaver",
		"heritage=dnsweaver,instance=pi5",
		`value with "quotes" inside`,
	}
	for _, in := range cases {
		q := quoteTXT(in)
		if q[0] != '"' || q[len(q)-1] != '"' {
			t.Errorf("quoteTXT(%q) not wrapped: %q", in, q)
		}
		if got := unquoteTXT(q); got != in {
			t.Errorf("round-trip failed: in=%q quoted=%q out=%q", in, q, got)
		}
	}
	// Already-quoted input is left unchanged by quoteTXT.
	if got := quoteTXT(`"already"`); got != `"already"` {
		t.Errorf("quoteTXT double-wrapped: %q", got)
	}
}

func TestSRVEncodeParse(t *testing.T) {
	srv := &provider.SRVData{Priority: 10, Weight: 20, Port: 5060}
	content := encodeSRVContent(srv, "sip.example.com")
	if content != "10 20 5060 sip.example.com." {
		t.Errorf("encodeSRVContent = %q", content)
	}
	got, target, err := parseSRVContent(content)
	if err != nil {
		t.Fatalf("parseSRVContent error: %v", err)
	}
	if target != "sip.example.com" {
		t.Errorf("parsed target = %q", target)
	}
	if *got != *srv {
		t.Errorf("parsed SRV = %+v, want %+v", got, srv)
	}
	if _, _, err := parseSRVContent("10 20 sip.example.com."); err == nil {
		t.Error("expected error for malformed SRV content")
	}
}

func TestRecordContentEncode(t *testing.T) {
	cases := []struct {
		rec  provider.Record
		want string
	}{
		{provider.Record{Type: provider.RecordTypeA, Target: "192.0.2.1"}, "192.0.2.1"},
		{provider.Record{Type: provider.RecordTypeAAAA, Target: "2001:db8::1"}, "2001:db8::1"},
		{provider.Record{Type: provider.RecordTypeCNAME, Target: "host.example.com"}, "host.example.com."},
		{provider.Record{Type: provider.RecordTypeTXT, Target: "heritage=dnsweaver"}, `"heritage=dnsweaver"`},
		{provider.Record{Type: provider.RecordTypeSRV, Target: "sip.example.com", SRV: &provider.SRVData{Priority: 1, Weight: 2, Port: 3}}, "1 2 3 sip.example.com."},
	}
	for _, tt := range cases {
		got, err := recordContent(tt.rec)
		if err != nil {
			t.Errorf("recordContent(%v) error: %v", tt.rec.Type, err)
			continue
		}
		if got != tt.want {
			t.Errorf("recordContent(%v) = %q, want %q", tt.rec.Type, got, tt.want)
		}
	}
	if _, err := recordContent(provider.Record{Type: provider.RecordTypeSRV}); err == nil {
		t.Error("expected error for SRV record without SRV data")
	}
}

func TestDecodeContent(t *testing.T) {
	target, _, _ := decodeContent(provider.RecordTypeCNAME, "host.example.com.")
	if target != "host.example.com" {
		t.Errorf("CNAME decode = %q", target)
	}
	target, _, _ = decodeContent(provider.RecordTypeTXT, `"heritage=dnsweaver"`)
	if target != "heritage=dnsweaver" {
		t.Errorf("TXT decode = %q", target)
	}
	target, srv, err := decodeContent(provider.RecordTypeSRV, "1 2 3 sip.example.com.")
	if err != nil || srv == nil || target != "sip.example.com" || srv.Port != 3 {
		t.Errorf("SRV decode = %q %+v err=%v", target, srv, err)
	}
}
