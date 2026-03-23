package version

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// These are set at build time via -ldflags.
var (
	Version   = "dev"
	Commit    = "none"
	Date      = "unknown"
	GoVersion = runtime.Version()
)

// Info returns a formatted version string.
func Info() string {
	return fmt.Sprintf("lintree %s (%s) built %s with %s", Version, shortCommit(), Date, GoVersion)
}

// Full returns detailed version info.
func Full() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "lintree %s\n", Version)
	fmt.Fprintf(&sb, "  commit:  %s\n", Commit)
	fmt.Fprintf(&sb, "  built:   %s\n", Date)
	fmt.Fprintf(&sb, "  go:      %s\n", GoVersion)
	fmt.Fprintf(&sb, "  os/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	return sb.String()
}

func shortCommit() string {
	if len(Commit) > 7 {
		return Commit[:7]
	}
	return Commit
}

// GitHubRelease represents a GitHub release API response.
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

const repoAPI = "https://api.github.com/repos/PatchMon/lintree/releases/latest"

// CheckForUpdate checks GitHub for a newer release.
// Returns (latestVersion, updateAvailable, error).
func CheckForUpdate() (string, bool, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(repoAPI)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", false, err
	}

	if Version == "dev" {
		return release.TagName, false, nil
	}

	if compareSemver(release.TagName, Version) > 0 {
		return release.TagName, true, nil
	}
	return release.TagName, false, nil
}

// UpdateCommand returns the command to update lintree.
func UpdateCommand() string {
	return "curl -fsSL https://get.lintree.sh | sh"
}

// compareSemver compares two semver strings (with optional "v" prefix).
// Returns >0 if a > b, <0 if a < b, 0 if equal.
func compareSemver(a, b string) int {
	pa := parseSemver(a)
	pb := parseSemver(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			return pa[i] - pb[i]
		}
	}
	return 0
}

func parseSemver(v string) [3]int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	var result [3]int
	for i := 0; i < len(parts) && i < 3; i++ {
		// Strip any pre-release suffix (e.g., "1-beta")
		num := strings.SplitN(parts[i], "-", 2)[0]
		result[i], _ = strconv.Atoi(num)
	}
	return result
}
