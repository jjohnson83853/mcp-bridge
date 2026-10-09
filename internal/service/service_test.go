package service_test

import (
	"net/http"
	"testing"

	"github.com/jjohnson83853/mcp-bridge/internal/service"
)

func TestAdvertisedProtocolVersionsExcludeLatest(t *testing.T) {
	// RED: narrowing contract — 2026-07-28 must never be advertised for calls.
	got := service.AdvertisedProtocolVersions()
	if len(got) != 4 {
		t.Fatalf("AdvertisedProtocolVersions = %v, want 4 entries (no 2026-07-28)", got)
	}
	for _, v := range got {
		if v == "2026-07-28" {
			t.Fatalf("AdvertisedProtocolVersions contains excluded latest 2026-07-28: %v", got)
		}
	}
	want := []string{"2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"}
	for i, v := range want {
		if got[i] != v {
			t.Fatalf("AdvertisedProtocolVersions[%d] = %q, want %q (stable newest-first order)", i, got[i], v)
		}
	}
}

func TestAdvertisedProtocolVersionsReturnsCopy(t *testing.T) {
	got := service.AdvertisedProtocolVersions()
	got[0] = "2026-07-28"
	again := service.AdvertisedProtocolVersions()
	if again[0] == "2026-07-28" {
		t.Fatal("AdvertisedProtocolVersions returned internal state; mutation leaked")
	}
}

func TestNoReplayPointerRequiredOnClientOptions(t *testing.T) {
	// RED: no-replay contract — every constructed client MUST carry a non-nil
	// Disabled MultiRoundTrip pointer; the middleware must never auto-retry
	// input_required results (replay counter ambiguity).
	opts := service.NoReplayClientOptions()
	if opts.MultiRoundTrip == nil {
		t.Fatal("NoReplayClientOptions().MultiRoundTrip is nil; want non-nil pointer")
	}
	if !opts.MultiRoundTrip.Disabled {
		t.Fatal("NoReplayClientOptions().MultiRoundTrip.Disabled is false; replay middleware would auto-retry")
	}
}

func TestInitializeOfferLatestNegotiatesDown(t *testing.T) {
	// RED: initialize offering 2026-07-28 must negotiate DOWN, not accept latest.
	negotiated := service.NegotiateInitializeOffer("2026-07-28")
	if negotiated != "2025-11-25" {
		t.Fatalf("initialize offer 2026-07-28 negotiated %q, want DOWN to 2025-11-25", negotiated)
	}
}

func TestInitializeOfferOlderRequestNegotiatesAgreement(t *testing.T) {
	// POSITIVE: older client request negotiates the newest listed agreement.
	if got := service.NegotiateInitializeOffer("2025-06-18"); got != "2025-06-18" {
		t.Fatalf("offer 2025-06-18 negotiated %q, want 2025-06-18", got)
	}
	if got := service.NegotiateInitializeOffer("2024-11-05"); got != "2024-11-05" {
		t.Fatalf("offer 2024-11-05 negotiated %q, want 2024-11-05", got)
	}
}

func TestInitializeOfferNoAgreementYieldsEmpty(t *testing.T) {
	// The spec table: an offer with no listed agreement negotiates nothing;
	// call-based requests at such versions are unsupported.
	if got := service.NegotiateInitializeOffer("1999-01-01"); got != "" {
		t.Fatalf("offer 1999-01-01 negotiated %q, want empty", got)
	}
}

func TestCallVersionValidationMatrix(t *testing.T) {
	// Matrix: call-based requests are validated against the narrowed list.
	// 400/CodeUnsupportedProtocolVersion applies to latest call-based requests.
	h := http.Header{service.ProtocolVersionHeader: {"2026-07-28"}}
	if err := service.ValidateCallVersion("2026-07-28", h); err == nil {
		t.Fatal("latest tools/call accepted; want rejection (400/CodeUnsupportedProtocolVersion)")
	}
	old := http.Header{service.ProtocolVersionHeader: {"2025-11-25"}}
	if err := service.ValidateCallVersion("2025-11-25", old); err != nil {
		t.Fatalf("older agreed call rejected: %v", err)
	}
	if err := service.ValidateCallVersion("2025-11-25", http.Header{}); err == nil {
		t.Fatal("call with _meta.protocolVersion but missing header accepted; want rejection")
	}
	if err := service.ValidateCallVersion("2025-11-25", http.Header{service.ProtocolVersionHeader: {"2025-06-18"}}); err == nil {
		t.Fatal("call with mismatched header/meta accepted; want rejection")
	}
}

func TestDiscoverRequestExemptFromLatestRejection(t *testing.T) {
	// Discover is exempt from the stateful latest rejection: an authenticated
	// discover request must not be treated as a 400-unsupported call.
	if err := service.ValidateCallVersion("server/discover", http.Header{service.ProtocolVersionHeader: {"2026-07-28"}}); err != nil {
		t.Fatalf("server/discover rejected; want discover exemption: %v", err)
	}
}
