// Package update finds out if a newer app release is available on GitHub.
// It does not download or install anything.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ReleasesURL is the page where users download the app.
const ReleasesURL = "https://github.com/1ukah/cod-server-picker/releases/latest"

const latestAPI = "https://api.github.com/repos/1ukah/cod-server-picker/releases/latest"

// Release is the latest published release.
type Release struct {
	Version string // for example "1.2.0", without "v"
	URL     string // release page
}

// Latest returns the latest published release.
func Latest(ctx context.Context) (Release, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestAPI, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("latest release: HTTP %d", resp.StatusCode)
	}
	var r struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return Release{}, err
	}
	return Release{Version: strings.TrimPrefix(r.TagName, "v"), URL: r.HTMLURL}, nil
}

// Newer reports whether version a is newer than version b. Both use the form X.Y.Z.
// An invalid version (for example "dev") is never newer, and is older than any valid version.
func Newer(a, b string) bool {
	x, okA := parse(a)
	y, okB := parse(b)
	if !okA {
		return false
	}
	if !okB {
		return true
	}
	for i := range x {
		if x[i] != y[i] {
			return x[i] > y[i]
		}
	}
	return false
}

func parse(v string) ([3]int, bool) {
	var out [3]int
	parts := strings.Split(strings.TrimPrefix(strings.TrimSpace(v), "v"), ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}
