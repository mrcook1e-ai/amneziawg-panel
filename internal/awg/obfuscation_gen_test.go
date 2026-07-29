package awg

import (
	"testing"
)

func TestGenerateObfuscation_Presets(t *testing.T) {
	for _, preset := range []string{"", PresetAuto, PresetFast, PresetStealth, "unknown"} {
		t.Run(preset, func(t *testing.T) {
			for i := 0; i < 30; i++ {
				spec, err := GenerateObfuscation(preset)
				if err != nil {
					t.Fatalf("iter %d: %v", i, err)
				}
				if spec.I1 != "" || spec.I2 != "" || spec.I3 != "" || spec.I4 != "" || spec.I5 != "" {
					t.Fatalf("I* must be empty: %+v", spec)
				}
				if spec.Jc < 3 || spec.Jc > 8 {
					t.Fatalf("Jc out of band: %d", spec.Jc)
				}
				if spec.Jmax <= spec.Jmin {
					t.Fatalf("Jmax<=Jmin: %d %d", spec.Jmin, spec.Jmax)
				}
				if spec.Jmax > 400 {
					t.Fatalf("Jmax too large for WAN: %d", spec.Jmax)
				}
				if spec.S4 > maxS4Padding {
					t.Fatalf("S4 %d > max", spec.S4)
				}
				// Init UDP payload stays modest (148 + S1).
				if 148+spec.S1 > 220 {
					t.Fatalf("init size too large: S1=%d", spec.S1)
				}
			}
		})
	}
}

func TestGenerateObfuscation_AutoIsTightestJunk(t *testing.T) {
	// Auto should stay under the "stealth" junk ceiling so the default
	// path is the most connectable.
	var autoMax, stealthMax int
	for i := 0; i < 40; i++ {
		a, err := GenerateObfuscation(PresetAuto)
		if err != nil {
			t.Fatal(err)
		}
		s, err := GenerateObfuscation(PresetStealth)
		if err != nil {
			t.Fatal(err)
		}
		if a.Jmax > autoMax {
			autoMax = a.Jmax
		}
		if s.Jmax > stealthMax {
			stealthMax = s.Jmax
		}
	}
	if autoMax > 200 {
		t.Fatalf("auto Jmax ceiling drifted: %d", autoMax)
	}
	if stealthMax < autoMax {
		t.Fatalf("stealth should allow larger junk than auto (%d vs %d)", stealthMax, autoMax)
	}
}
