package format

import "fmt"

// Count formats an integer with thousand separators: 14832 → "14,832".
func Count(n int64) string {
	if n < 0 {
		return "-" + Count(-n)
	}
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	// Insert commas from the right
	out := make([]byte, 0, len(s)+(len(s)-1)/3)
	for i := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, s[i])
	}
	return string(out)
}

// Size formats a byte count as a human-readable string.
func Size(bytes int64) string {
	const (
		KiB = 1024
		MiB = KiB * 1024
		GiB = MiB * 1024
		TiB = GiB * 1024
	)
	switch {
	case bytes >= TiB:
		return fmt.Sprintf("%.1f TiB", float64(bytes)/float64(TiB))
	case bytes >= GiB:
		return fmt.Sprintf("%.1f GiB", float64(bytes)/float64(GiB))
	case bytes >= MiB:
		return fmt.Sprintf("%.1f MiB", float64(bytes)/float64(MiB))
	case bytes >= KiB:
		return fmt.Sprintf("%.1f KiB", float64(bytes)/float64(KiB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
