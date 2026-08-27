package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Presets are fetched over the network at runtime, not compiled in, so cleaning
// the checked-in provider-presets.json accomplishes nothing while a source still
// points at upstream — the affiliate-laden list returns within presetsCacheTTL.
// Both the primary and the fallback have to be checked: a fallback pointing at
// upstream silently undoes the removal exactly when the primary is unreachable.
func TestPresetSourcesDoNotPointAtUpstream(t *testing.T) {
	sources := map[string]string{
		"defaultPresetsURL":       defaultPresetsURL,
		"fallbackPresetsURL":      fallbackPresetsURL,
		"defaultSkillPresetsURL":  defaultSkillPresetsURL,
		"fallbackSkillPresetsURL": fallbackSkillPresetsURL,
	}
	for name, url := range sources {
		for _, upstream := range []string{"chenhg5/cc-connect", "cg33/cc-connect"} {
			if strings.Contains(url, upstream) {
				t.Errorf("%s = %q still resolves to upstream (%s); it would re-pull the referral codes stripped from the local presets", name, url, upstream)
			}
		}
		if !strings.HasPrefix(url, "https://") {
			t.Errorf("%s = %q must be https", name, url)
		}
	}
}

// The checked-in presets are what those URLs serve, so the referral codes must be
// gone from the file itself too. Guards a merge that reintroduces them.
func TestCheckedInProviderPresetsCarryNoReferralCodes(t *testing.T) {
	path := filepath.Join("..", "provider-presets.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var presets ProviderPresetsResponse
	if err := json.Unmarshal(raw, &presets); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	if len(presets.Providers) == 0 {
		t.Fatal("no providers parsed; the guard below would pass vacuously")
	}

	// Query-string referral params, path-shaped invite codes, and the promo copy
	// that only held while those codes were ours to hand out.
	referralParam := regexp.MustCompile(`(?i)[?&](aff|ref|track_id|referral_code|invitecode|ytag|code|source|from|ch)=`)
	referralPath := regexp.MustCompile(`(?i)/(invite|i|r|ref)/[^/]+/?$`)

	for _, p := range presets.Providers {
		if p.InviteURL == "" {
			continue
		}
		if m := referralParam.FindString(p.InviteURL); m != "" {
			t.Errorf("provider %q invite_url carries referral param %q: %s", p.Name, strings.Trim(m, "?&="), p.InviteURL)
		}
		if referralPath.MatchString(p.InviteURL) {
			t.Errorf("provider %q invite_url is a path-shaped referral code: %s", p.Name, p.InviteURL)
		}
	}

	// Promo copy naming this project is a claim this deployment cannot honour once
	// the referral codes are gone. Case-insensitive: upstream wrote both
	// "cc-connect" and "CC-Connect".
	lower := strings.ToLower(string(raw))
	for _, claim := range []string{"cc-connect", "cc_connect", "专享", "sponsor"} {
		if strings.Contains(lower, strings.ToLower(claim)) {
			t.Errorf("provider-presets.json still contains promo marker %q", claim)
		}
	}
}
