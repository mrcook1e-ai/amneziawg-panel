package awg

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
)

// testKeyGen stands in for `awg genkey`, which is not available in unit tests.
func testKeyGen() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b[:]), nil
}

const genIterations = 40

func TestGenerateObfuscation_AllPresetsValidate(t *testing.T) {
	for _, preset := range []string{PresetAWG1, PresetAWG2, PresetAWG31} {
		t.Run(preset, func(t *testing.T) {
			for i := 0; i < genIterations; i++ {
				spec, err := GenerateObfuscation(preset, testKeyGen)
				if err != nil {
					t.Fatalf("iter %d: %v", i, err)
				}
				if err := spec.Validate(); err != nil {
					t.Fatalf("iter %d: generated spec fails Validate: %v", i, err)
				}
				if spec.Jmax <= spec.Jmin {
					t.Fatalf("Jmax<=Jmin: %d %d", spec.Jmin, spec.Jmax)
				}
				if spec.S1 == spec.S2 {
					t.Fatalf("S1 == S2 (%d): padded init/response sizes collide", spec.S1)
				}
			}
		})
	}
}

func TestGenerateObfuscation_AWG1IsPre2Vocabulary(t *testing.T) {
	// An AWG 1.0 parser aborts on any key it does not know, so the 1.0 preset
	// must carry nothing from 2.0 (S3/S4, I*, H ranges) or 3.x.
	for i := 0; i < genIterations; i++ {
		spec, err := GenerateObfuscation(PresetAWG1, testKeyGen)
		if err != nil {
			t.Fatal(err)
		}
		if spec.S3 != 0 || spec.S4 != 0 {
			t.Fatalf("AWG 1.0 must not set S3/S4: S3=%d S4=%d", spec.S3, spec.S4)
		}
		if spec.I1 != "" || spec.I2 != "" || spec.I3 != "" || spec.I4 != "" || spec.I5 != "" {
			t.Fatalf("AWG 1.0 must not set I1-I5: %+v", spec)
		}
		for _, h := range []string{spec.H1, spec.H2, spec.H3, spec.H4} {
			if isRangeValue(h) {
				t.Fatalf("AWG 1.0 H must be a fixed value, got %q", h)
			}
		}
		assertNoAWG3(t, spec)

		p := &Profile{}
		applySpec(p, spec)
		if got := p.Generation(); got != GenAWG1 {
			t.Fatalf("generation = %q, want %q", got, GenAWG1)
		}
	}
}

func TestGenerateObfuscation_AWG2MatchesOfficialShape(t *testing.T) {
	for i := 0; i < genIterations; i++ {
		spec, err := GenerateObfuscation(PresetAWG2, testKeyGen)
		if err != nil {
			t.Fatal(err)
		}
		if spec.S3 == 0 || spec.S4 == 0 {
			t.Fatalf("AWG 2.0 must set S3/S4: %+v", spec)
		}
		// Official AWG 2.0 default: stable I1 only; no random I2–I5.
		if spec.I1 != defaultI1CPS {
			t.Fatalf("I1 = %q, want official default", spec.I1)
		}
		if spec.I2 != "" || spec.I3 != "" || spec.I4 != "" || spec.I5 != "" {
			t.Fatalf("I2–I5 must be empty: %+v", spec)
		}
		for _, h := range []string{spec.H1, spec.H2, spec.H3, spec.H4} {
			if !isRangeValue(h) {
				t.Fatalf("AWG 2.0 H must be a range, got %q", h)
			}
		}
		assertNoAWG3(t, spec)

		p := &Profile{}
		applySpec(p, spec)
		if got := p.Generation(); got != GenAWG2 {
			t.Fatalf("generation = %q, want %q", got, GenAWG2)
		}
	}
}

func TestGenerateObfuscation_AWG31MatchesOfficialDefaults(t *testing.T) {
	for i := 0; i < genIterations; i++ {
		spec, err := GenerateObfuscation(PresetAWG31, testKeyGen)
		if err != nil {
			t.Fatal(err)
		}
		// The ChaCha20 nonce for header protection is read from the first 12
		// bytes of the S prefix; amneziawg-go rejects the UAPI set otherwise.
		for j, sv := range []int{spec.S1, spec.S2, spec.S3, spec.S4} {
			if sv < hpkNonceBytes {
				t.Fatalf("S%d = %d, must be >= %d with HeaderProtectionKey set", j+1, sv, hpkNonceBytes)
			}
		}
		// Header protection makes hiding H pointless; official defaults go
		// back to the standard WireGuard message types.
		if spec.H1 != "1" || spec.H2 != "2" || spec.H3 != "3" || spec.H4 != "4" {
			t.Fatalf("H must be the standard 1/2/3/4, got %q %q %q %q", spec.H1, spec.H2, spec.H3, spec.H4)
		}
		if spec.HeaderProtectionKey == "" {
			t.Fatal("HeaderProtectionKey must be set")
		}
		var probe string
		if err := parseHPKField(&probe, spec.HeaderProtectionKey); err != nil {
			t.Fatalf("HeaderProtectionKey invalid: %v", err)
		}
		if !spec.RandomTrailers || !spec.DisableCookies {
			t.Fatalf("RandomTrailers/DisableCookies must be on: %+v", spec)
		}
		for _, f := range []struct{ name, got, want string }{
			{"ContentPaddingAddition", spec.ContentPaddingAddition, "10-100"},
			{"RekeyAfterTime", spec.RekeyAfterTime, "100-120"},
			{"RekeyTimeout", spec.RekeyTimeout, "3-7"},
			{"RejectAfterTime", spec.RejectAfterTime, "150-180"},
			{"KeepaliveTimeout", spec.KeepaliveTimeout, "5-15"},
			{"MaxHandshakeAttempts", spec.MaxHandshakeAttempts, "15-20"},
		} {
			if f.got != f.want {
				t.Fatalf("%s = %q, want %q", f.name, f.got, f.want)
			}
		}

		p := &Profile{}
		applySpec(p, spec)
		if got := p.Generation(); got != GenAWG31 {
			t.Fatalf("generation = %q, want %q", got, GenAWG31)
		}
	}
}

func TestGenerateObfuscation_AWG31KeysAreUniquePerProfile(t *testing.T) {
	// One device = one profile = one interface. A header protection key shared
	// between subscribers would let any of them decrypt the others' headers.
	seen := map[string]bool{}
	for i := 0; i < genIterations; i++ {
		spec, err := GenerateObfuscation(PresetAWG31, testKeyGen)
		if err != nil {
			t.Fatal(err)
		}
		if seen[spec.HeaderProtectionKey] {
			t.Fatalf("HeaderProtectionKey reused across profiles: %q", spec.HeaderProtectionKey)
		}
		seen[spec.HeaderProtectionKey] = true
	}
}

func TestGenerateObfuscation_AWG31RequiresKeyGen(t *testing.T) {
	if _, err := GenerateObfuscation(PresetAWG31, nil); err == nil {
		t.Fatal("expected an error when no key generator is supplied")
	}
	// The other generations need no key material and must still work.
	for _, preset := range []string{PresetAWG1, PresetAWG2} {
		if _, err := GenerateObfuscation(preset, nil); err != nil {
			t.Fatalf("%s without key generator: %v", preset, err)
		}
	}
}

func TestNormalizePreset_LegacyAliases(t *testing.T) {
	for _, in := range []string{PresetAuto, PresetStealth, PresetFast} {
		if got := NormalizePreset(in); got != PresetAWG2 {
			t.Fatalf("NormalizePreset(%q) = %q, want %q", in, got, PresetAWG2)
		}
	}
	for _, in := range []string{"", "nonsense"} {
		if got := NormalizePreset(in); got != DefaultPreset {
			t.Fatalf("NormalizePreset(%q) = %q, want %q", in, got, DefaultPreset)
		}
	}
	for _, in := range []string{PresetAWG1, PresetAWG2, PresetAWG31} {
		if got := NormalizePreset(in); got != in {
			t.Fatalf("NormalizePreset(%q) = %q, want itself", in, got)
		}
	}
}

// TestGenerateObfuscation_LegacyAliasesStillProduceAWG2 pins the promise made
// to cached cabinet bundles: an old client sending "auto" gets what it always
// got, not a generation its app may not speak.
func TestGenerateObfuscation_LegacyAliasesStillProduceAWG2(t *testing.T) {
	for _, preset := range []string{PresetAuto, PresetStealth, PresetFast, ""} {
		spec, err := GenerateObfuscation(preset, testKeyGen)
		if err != nil {
			t.Fatalf("%q: %v", preset, err)
		}
		p := &Profile{}
		applySpec(p, spec)
		if got := p.Generation(); got != GenAWG2 {
			t.Fatalf("preset %q produced generation %q, want %q", preset, got, GenAWG2)
		}
	}
}

func assertNoAWG3(t *testing.T, spec ObfuscationSpec) {
	t.Helper()
	if spec.HeaderProtectionKey != "" || spec.ContentPaddingAddition != "" ||
		spec.RekeyAfterTime != "" || spec.RekeyTimeout != "" ||
		spec.RejectAfterTime != "" || spec.KeepaliveTimeout != "" ||
		spec.MaxHandshakeAttempts != "" ||
		spec.RandomTrailers || spec.DisableCookies {
		t.Fatalf("pre-3.x preset must not set any AWG 3.x field: %+v", spec)
	}
}

// TestGeneratedConfsAreCleanPerGeneration renders both sides of every preset
// and asserts the conf vocabulary stays inside what that generation's parser
// accepts — amneziawg-tools aborts the whole interface on one unknown key.
func TestGeneratedConfsAreCleanPerGeneration(t *testing.T) {
	forbidden := map[string][]string{
		PresetAWG1:  {"S3 ", "S4 ", "I1 ", "HeaderProtectionKey", "RandomTrailers", "DisableCookies", "ContentPaddingAddition"},
		PresetAWG2:  {"HeaderProtectionKey", "RandomTrailers", "DisableCookies", "ContentPaddingAddition", "MaxHandshakeAttempts"},
		PresetAWG31: {"Itime", "J1 ", "J2 ", "J3 "},
	}
	for preset, banned := range forbidden {
		t.Run(preset, func(t *testing.T) {
			spec, err := GenerateObfuscation(preset, testKeyGen)
			if err != nil {
				t.Fatal(err)
			}
			p := testProfile()
			applySpec(p, spec)
			c := testClient()

			server, err := RenderProfile(ProfileRenderArgs{
				Profile: p, Peers: []*Client{c}, SubnetCIDR: "10.8.0.0/24", Egress: "eth0",
			})
			if err != nil {
				t.Fatal(err)
			}
			client, err := RenderClient(ClientRenderArgs{
				Profile: p, Client: c, DNS: "1.1.1.1", MTU: 1280,
				AllowedIPs: "0.0.0.0/0", Endpoint: "vpn.example:51820", KeepaliveSecs: 25,
			})
			if err != nil {
				t.Fatal(err)
			}
			for _, key := range banned {
				if strings.Contains(string(server), key) {
					t.Fatalf("server conf for %s must not contain %q:\n%s", preset, key, server)
				}
				if strings.Contains(string(client), key) {
					t.Fatalf("client conf for %s must not contain %q:\n%s", preset, key, client)
				}
			}
		})
	}
}
