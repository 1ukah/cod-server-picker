// Command gen-regions builds internal/regions/regions.json.
//
// It reads data/catalog.json (locations and the server IPs seen at each location) and
// downloads the official IP lists of Google Cloud, Microsoft Azure and Vultr. For each
// location it finds the provider regions that contain the server IPs, and it adds all the
// IP ranges of those provider regions.
//
// The file is written only when the regions or ranges change.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/1ukah/cod-server-picker/internal/regions"
)

const (
	googleURL     = "https://www.gstatic.com/ipranges/cloud.json"
	vultrURL      = "https://geofeed.constant.com/"
	gcpingURL     = "https://global.gcping.com/api/endpoints"
	azurePageURL  = "https://www.microsoft.com/en-us/download/details.aspx?id=56519"
	azureFileExpr = `https://download\.microsoft\.com/[^"]+ServiceTags_Public_\d+\.json`
)

type catalog struct {
	Regions []regions.Region `json:"regions"`
	Presets []regions.Preset `json:"presets"`
}

type providerRange struct {
	key    string
	prefix netip.Prefix
}

func main() {
	catalogPath := flag.String("catalog", "data/catalog.json", "location catalog")
	outPath := flag.String("out", "internal/regions/regions.json", "output file")
	force := flag.Bool("force", false, "write the file even if nothing changed")
	flag.Parse()

	if err := run(*catalogPath, *outPath, *force); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(catalogPath, outPath string, force bool) error {
	data, err := os.ReadFile(catalogPath)
	if err != nil {
		return err
	}
	var cat catalog
	if err := json.Unmarshal(data, &cat); err != nil {
		return fmt.Errorf("%s: %w", catalogPath, err)
	}

	ctx := context.Background()
	var all []providerRange
	for _, src := range []struct {
		name  string
		fetch func(context.Context) ([]providerRange, error)
	}{{"Google Cloud", fetchGoogle}, {"Azure", fetchAzure}, {"Vultr", fetchVultr}} {
		r, err := src.fetch(ctx)
		if err != nil {
			return fmt.Errorf("%s: %w", src.name, err)
		}
		if len(r) == 0 {
			return fmt.Errorf("%s: list is empty", src.name)
		}
		fmt.Printf("%-12s %6d ranges\n", src.name, len(r))
		all = append(all, r...)
	}

	gcping, err := fetchGCPing(ctx)
	if err != nil {
		return fmt.Errorf("gcping: %w", err)
	}

	byKey := map[string][]netip.Prefix{}
	for _, r := range all {
		byKey[r.key] = append(byKey[r.key], r.prefix)
	}

	owner := map[string]string{} // provider key -> location id
	problems := 0
	out := regions.File{Schema: regions.Schema, Generated: time.Now().UTC().Truncate(time.Second), Presets: cat.Presets}
	for _, reg := range cat.Regions {
		for i := range reg.Locations {
			loc := &reg.Locations[i]
			loc.Providers, loc.Ranges, loc.PingIPs, loc.PingURLs = nil, nil, nil, nil
			for _, b := range loc.Beacons {
				ip, err := netip.ParseAddr(b)
				if err != nil {
					return fmt.Errorf("location %s: bad beacon %q", loc.ID, b)
				}
				key := match(all, ip)
				if key == "" {
					fmt.Printf("warning: %s beacon %s is not in any provider list\n", loc.ID, b)
					problems++
					continue
				}
				if prev, ok := owner[key]; ok && prev != loc.ID {
					fmt.Printf("warning: %s is used by both %s and %s\n", key, prev, loc.ID)
					problems++
				}
				owner[key] = loc.ID
				if !slices.Contains(loc.Providers, key) {
					loc.Providers = append(loc.Providers, key)
				}
				if region, ok := strings.CutPrefix(key, "gcp:"); ok {
					if u := gcping[region]; u != "" && !slices.Contains(loc.PingURLs, u) {
						loc.PingURLs = append(loc.PingURLs, u)
					}
				} else if ip.Is4() {
					loc.PingIPs = append(loc.PingIPs, b)
				}
			}
			slices.Sort(loc.Providers)
			if len(loc.PingIPs) == 0 && len(loc.PingURLs) == 0 {
				fmt.Printf("warning: %s has no ping target\n", loc.ID)
				problems++
			}
			var prefixes []netip.Prefix
			for _, key := range loc.Providers {
				prefixes = append(prefixes, byKey[key]...)
			}
			loc.Ranges = sortedStrings(prefixes)
			if len(loc.Ranges) == 0 {
				return fmt.Errorf("location %s has no ranges", loc.ID)
			}
			fmt.Printf("%-18s %-55s %5d ranges\n", loc.ID, strings.Join(loc.Providers, " "), len(loc.Ranges))
		}
		out.Regions = append(out.Regions, reg)
	}
	if err := out.Validate(); err != nil {
		return err
	}
	if problems > 0 {
		fmt.Printf("%d warning(s). Check data/catalog.json.\n", problems)
	}

	if !force {
		if old, err := os.ReadFile(outPath); err == nil {
			var prev regions.File
			if json.Unmarshal(old, &prev) == nil && sameContent(&prev, &out) {
				fmt.Println("No change. The file was not written.")
				return nil
			}
		}
	}
	buf, err := json.MarshalIndent(out, "", " ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(outPath, append(buf, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Println("Wrote", outPath)
	return nil
}

// match returns the provider key of the most specific range that contains ip.
func match(all []providerRange, ip netip.Addr) string {
	best, bits := "", -1
	for _, r := range all {
		if r.prefix.Contains(ip) && r.prefix.Bits() > bits {
			best, bits = r.key, r.prefix.Bits()
		}
	}
	return best
}

func sameContent(a, b *regions.File) bool {
	x, _ := json.Marshal(struct {
		R []regions.Region
		P []regions.Preset
	}{a.Regions, a.Presets})
	y, _ := json.Marshal(struct {
		R []regions.Region
		P []regions.Preset
	}{b.Regions, b.Presets})
	return bytes.Equal(x, y)
}

func sortedStrings(prefixes []netip.Prefix) []string {
	seen := map[netip.Prefix]bool{}
	var uniq []netip.Prefix
	for _, p := range prefixes {
		p = p.Masked()
		if !seen[p] {
			seen[p] = true
			uniq = append(uniq, p)
		}
	}
	slices.SortFunc(uniq, func(a, b netip.Prefix) int {
		if c := a.Addr().Compare(b.Addr()); c != 0 {
			return c
		}
		return a.Bits() - b.Bits()
	})
	out := make([]string, len(uniq))
	for i, p := range uniq {
		out[i] = p.String()
	}
	return out
}

func get(ctx context.Context, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "cod-server-picker gen-regions")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func fetchGoogle(ctx context.Context) ([]providerRange, error) {
	data, err := get(ctx, googleURL)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Prefixes []struct {
			IPv4  string `json:"ipv4Prefix"`
			IPv6  string `json:"ipv6Prefix"`
			Scope string `json:"scope"`
		} `json:"prefixes"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	var out []providerRange
	for _, p := range doc.Prefixes {
		for _, s := range []string{p.IPv4, p.IPv6} {
			if pre, err := netip.ParsePrefix(s); err == nil && p.Scope != "" && p.Scope != "global" {
				out = append(out, providerRange{"gcp:" + p.Scope, pre})
			}
		}
	}
	return out, nil
}

// fetchGCPing returns the gcping.com test URL of each Google Cloud region.
func fetchGCPing(ctx context.Context) (map[string]string, error) {
	data, err := get(ctx, gcpingURL)
	if err != nil {
		return nil, err
	}
	var doc map[string]struct {
		Region string `json:"Region"`
		URL    string `json:"URL"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, e := range doc {
		if e.Region != "" && strings.HasPrefix(e.URL, "https://") {
			out[e.Region] = e.URL
		}
	}
	return out, nil
}

func fetchAzure(ctx context.Context) ([]providerRange, error) {
	page, err := get(ctx, azurePageURL)
	if err != nil {
		return nil, err
	}
	url := regexp.MustCompile(azureFileExpr).Find(page)
	if url == nil {
		return nil, fmt.Errorf("no ServiceTags file link on %s", azurePageURL)
	}
	data, err := get(ctx, string(url))
	if err != nil {
		return nil, err
	}
	var doc struct {
		Values []struct {
			Name       string `json:"name"`
			Properties struct {
				AddressPrefixes []string `json:"addressPrefixes"`
			} `json:"properties"`
		} `json:"values"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	var out []providerRange
	for _, v := range doc.Values {
		region, ok := strings.CutPrefix(v.Name, "AzureCloud.")
		if !ok || region == "" {
			continue
		}
		for _, s := range v.Properties.AddressPrefixes {
			if pre, err := netip.ParsePrefix(s); err == nil {
				out = append(out, providerRange{"azure:" + region, pre})
			}
		}
	}
	return out, nil
}

func fetchVultr(ctx context.Context) ([]providerRange, error) {
	data, err := get(ctx, vultrURL)
	if err != nil {
		return nil, err
	}
	var out []providerRange
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, ",")
		if len(f) < 4 {
			continue
		}
		pre, err := netip.ParsePrefix(f[0])
		if err != nil {
			continue
		}
		key := "vultr:" + strings.ToLower(f[1]+"-"+strings.ReplaceAll(f[3], " ", "-"))
		out = append(out, providerRange{key, pre})
	}
	return out, sc.Err()
}
