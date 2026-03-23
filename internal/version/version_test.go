package version

import "testing"

func TestCompareSemver(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want int // >0, <0, or 0
	}{
		{"patch bump", "v1.2.3", "v1.2.4", -1},
		{"major greater", "v2.0.0", "v1.9.9", 1},
		{"equal versions", "v1.0.0", "v1.0.0", 0},
		{"minor 10 vs 9", "1.10.0", "1.9.0", 1},
		{"no v prefix", "1.2.3", "1.2.3", 0},
		{"mixed prefix", "v1.0.0", "1.0.0", 0},
		{"minor bump", "v0.2.0", "v0.1.0", 1},
		{"patch only", "v0.0.2", "v0.0.1", 1},
		{"higher patch lower minor", "v1.0.5", "v1.1.0", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareSemver(tt.a, tt.b)
			switch {
			case tt.want > 0 && got <= 0:
				t.Errorf("compareSemver(%q, %q) = %d, want positive", tt.a, tt.b, got)
			case tt.want < 0 && got >= 0:
				t.Errorf("compareSemver(%q, %q) = %d, want negative", tt.a, tt.b, got)
			case tt.want == 0 && got != 0:
				t.Errorf("compareSemver(%q, %q) = %d, want 0", tt.a, tt.b, got)
			}
		})
	}
}

func TestParseSemver(t *testing.T) {
	tests := []struct {
		name string
		v    string
		want [3]int
	}{
		{"standard", "v1.2.3", [3]int{1, 2, 3}},
		{"no prefix", "1.2.3", [3]int{1, 2, 3}},
		{"pre-release suffix", "1.2.3-beta", [3]int{1, 2, 3}},
		{"pre-release on patch", "v2.0.1-rc1", [3]int{2, 0, 1}},
		{"two parts only", "1.2", [3]int{1, 2, 0}},
		{"one part", "3", [3]int{3, 0, 0}},
		{"double digit", "1.10.0", [3]int{1, 10, 0}},
		{"large numbers", "v12.345.6", [3]int{12, 345, 6}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSemver(tt.v)
			if got != tt.want {
				t.Errorf("parseSemver(%q) = %v, want %v", tt.v, got, tt.want)
			}
		})
	}
}
