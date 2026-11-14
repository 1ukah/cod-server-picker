// Command cspcli is the command-line version of CoD Server Picker, for tests and advanced users.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/1ukah/cod-server-picker/internal/audit"
	"github.com/1ukah/cod-server-picker/internal/config"
	"github.com/1ukah/cod-server-picker/internal/firewall"
	"github.com/1ukah/cod-server-picker/internal/game"
	"github.com/1ukah/cod-server-picker/internal/monitor"
	"github.com/1ukah/cod-server-picker/internal/ping"
	"github.com/1ukah/cod-server-picker/internal/regions"
)

// version is set at build time with -ldflags "-X main.version=X.Y.Z".
var version = "dev"

const usage = `cspcli - CoD Server Picker command line

Usage:
  cspcli version                      show the version
  cspcli regions                      show regions and locations
  cspcli ping [id ...]                ping all locations, or the given region or location IDs
  cspcli find                         find cod.exe (Steam, then the running game)
  cspcli status                       show blocked locations
  cspcli block <id|all|preset:id> ... block locations or regions (admin)
  cspcli unblock                      remove all rules (admin)
  cspcli record                       record game servers until Ctrl+C (admin)
  cspcli audit [reset]                show or turn off connection auditing (admin)
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(1)
	}
	if err := run(os.Args[1], os.Args[2:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(cmd string, args []string) error {
	dir, err := config.Dir()
	if err != nil {
		return err
	}
	f, src, err := regions.Load(dir)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	switch cmd {
	case "version":
		fmt.Println(version)
		return nil

	case "regions":
		fmt.Printf("Regions list: %s, generated %s\n", src, f.Generated.Format(time.DateTime))
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		for _, r := range f.Regions {
			for _, l := range r.Locations {
				fmt.Fprintf(w, "%s\t%s\t%s\t%d ranges\t%s\n", r.ID, l.ID, l.Name, len(l.Ranges), strings.Join(l.Providers, " "))
			}
		}
		return w.Flush()

	case "ping":
		ids, err := expand(f, args)
		if err != nil {
			return err
		}
		if len(args) == 0 {
			ids = f.AllLocationIDs()
		}
		var targets []ping.Target
		for _, id := range ids {
			l := f.Location(id)
			targets = append(targets, ping.Target{ID: id, IPs: l.PingIPs, URLs: l.PingURLs})
		}
		start := time.Now()
		var mu sync.Mutex
		results := map[string]string{}
		ping.MeasureAll(ctx, targets, 12, func(id string, r ping.Result, err error) {
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				results[id] = "no reply"
			} else {
				results[id] = fmt.Sprintf("%4d ms  %-8s %s", r.RTT.Milliseconds(), r.Method, r.Hop)
			}
		})
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		for _, id := range ids {
			fmt.Fprintf(w, "%s\t%s\t%s\n", f.RegionOf(id).Name, f.Location(id).Name, results[id])
		}
		w.Flush()
		fmt.Printf("Done in %s\n", time.Since(start).Round(time.Millisecond))
		return nil

	case "find":
		if p, err := game.FindSteam(); err == nil {
			fmt.Println("Steam:  ", p)
		} else {
			fmt.Println("Steam:  ", err)
		}
		if p, err := game.FindRunning(); err == nil {
			fmt.Println("Running:", p)
		} else {
			fmt.Println("Running:", err)
		}
		return nil

	case "status":
		st, err := firewall.Current()
		if err != nil {
			return err
		}
		if len(st.Locations) == 0 && len(st.Unknown) == 0 {
			fmt.Println("Nothing is blocked.")
			return nil
		}
		fmt.Println("Program:", orAll(st.Program))
		for _, id := range st.Locations {
			if l := f.Location(id); l != nil {
				fmt.Printf("  blocked: %s (%s)\n", l.Name, id)
			} else {
				fmt.Printf("  blocked: %s (not in the regions list)\n", id)
			}
		}
		for _, n := range st.Unknown {
			fmt.Println("  other rule:", n)
		}
		return nil

	case "block":
		ids, err := expand(f, args)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			return fmt.Errorf("give location IDs, region IDs, preset:<id> or all")
		}
		program := ""
		s, _ := config.Load()
		if !s.AllPrograms {
			program = s.GamePath
			if program == "" {
				if program, err = game.FindSteam(); err != nil {
					return fmt.Errorf("cod.exe not found; set gamePath in settings.json")
				}
			}
		}
		var blocks []firewall.Block
		for _, id := range ids {
			blocks = append(blocks, firewall.Block{Location: id, Ranges: f.Location(id).Ranges})
		}
		start := time.Now()
		if err := firewall.Apply(program, blocks); err != nil {
			return err
		}
		fmt.Printf("Blocked %d locations for %s in %s\n", len(ids), orAll(program), time.Since(start).Round(time.Millisecond))
		return nil

	case "unblock":
		return firewall.Clear()

	case "audit":
		if len(args) > 0 && args[0] == "reset" {
			if err := audit.Reset(); err != nil {
				return err
			}
		}
		on, err := audit.Enabled()
		if err != nil {
			return err
		}
		fmt.Println("Connection auditing on:", on)
		return nil

	case "record":
		fmt.Println("Recording. Play the game. Press Ctrl+C to stop.")
		rec, err := monitor.Run(ctx, game.ExeName, "", f, func(s monitor.Server) {
			loc := s.Location
			if loc == "" {
				loc = "UNKNOWN"
			}
			fmt.Printf("%s  %-4s %-40s %s\n", time.Now().Format("15:04:05"), s.Proto, s.IP, loc)
		})
		if rec != nil {
			p, serr := rec.Save(dir + `\recordings`)
			if serr == nil {
				fmt.Println("Saved", p)
			}
		}
		return err
	}
	fmt.Print(usage)
	return nil
}

// expand turns location IDs, region IDs, "all" and "preset:<id>" into location IDs.
func expand(f *regions.File, args []string) ([]string, error) {
	var ids []string
	for _, a := range args {
		switch {
		case a == "all":
			ids = append(ids, f.AllLocationIDs()...)
		case strings.HasPrefix(a, "preset:"):
			p := f.Preset(strings.TrimPrefix(a, "preset:"))
			if p == nil {
				return nil, fmt.Errorf("unknown preset %q", a)
			}
			ids = append(ids, f.BlockedBy(p)...)
		case f.Region(a) != nil:
			ids = append(ids, f.Region(a).LocationIDs()...)
		case f.Location(a) != nil:
			ids = append(ids, a)
		default:
			return nil, fmt.Errorf("unknown ID %q", a)
		}
	}
	sort.Strings(ids)
	return slices.Compact(ids), nil
}

func orAll(p string) string {
	if p == "" {
		return "all programs"
	}
	return p
}
