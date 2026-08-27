package main

import (
	"os"
	"strings"
	"testing"
)

// Upstream printed "⭐ SPONSOR" next to every tier<=1 preset in
// `cc-connect provider presets`, because those presets' invite URLs carried its
// referral codes. This fork strips the codes (see
// core/provider_presets_source_test.go), which makes the badge a claim about a
// relationship that does not exist — the same dangling-claim problem as the
// "exclusive discount for cc-connect users" copy removed from the presets file.
//
// A source-level guard rather than an output assertion: runProviderPresets
// fetches over the network, so exercising it in a unit test would make the test
// depend on connectivity. What needs protecting is that a merge from upstream
// cannot quietly restore the label.
func TestProviderPresetsOutputCarriesNoSponsorBadge(t *testing.T) {
	const path = "provider.go"
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	// Only the literals that would reach the user's terminal. provider.go's own
	// comment deliberately avoids spelling the badge out, so a hit means real code.
	for _, badge := range []string{"SPONSOR", "⭐"} {
		if strings.Contains(string(raw), badge) {
			t.Errorf("%s prints %q; the presets no longer carry referral codes, so the badge asserts a sponsorship this build does not have", path, badge)
		}
	}
}
