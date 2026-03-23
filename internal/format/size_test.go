package format

import "testing"

func TestSize(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		// Byte range
		{"zero bytes", 0, "0 B"},
		{"one byte", 1, "1 B"},
		{"1023 bytes", 1023, "1023 B"},

		// KB range
		{"exactly 1 KB", 1024, "1.0 KB"},
		{"1.5 KB", 1536, "1.5 KB"},
		{"just under 1 MB", 1048575, "1024.0 KB"},

		// MB range
		{"exactly 1 MB", 1048576, "1.0 MB"},
		{"1.5 MB", 1048576 + 524288, "1.5 MB"},
		{"999 MB", 999 * 1048576, "999.0 MB"},

		// GB range
		{"exactly 1 GB", 1 << 30, "1.0 GB"},
		{"2.5 GB", int64(2.5 * float64(1<<30)), "2.5 GB"},

		// TB range
		{"exactly 1 TB", 1 << 40, "1.0 TB"},
		{"3.7 TB", 4068193022771, "3.7 TB"},

		// Negative value (falls through to default)
		{"negative bytes", -100, "-100 B"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Size(tt.bytes)
			if got != tt.want {
				t.Errorf("Size(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestCount(t *testing.T) {
	tests := []struct {
		name string
		n    int64
		want string
	}{
		{"zero", 0, "0"},
		{"small", 42, "42"},
		{"hundreds", 999, "999"},
		{"thousands", 1000, "1,000"},
		{"large", 14832, "14,832"},
		{"millions", 1234567, "1,234,567"},
		{"negative", -14832, "-14,832"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Count(tt.n)
			if got != tt.want {
				t.Errorf("Count(%d) = %q, want %q", tt.n, got, tt.want)
			}
		})
	}
}
