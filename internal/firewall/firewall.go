// Package firewall adds and removes the Windows Firewall rules of the app.
//
// It uses the Windows Firewall COM API (HNetCfg.FwPolicy2). All rules are in the rule group
// "CoDServerPicker". Each rule blocks outbound UDP from the game to the IP ranges of one
// location. Administrator rights are necessary to change rules.
package firewall

import (
	"fmt"
	"runtime"
	"sort"
	"strings"

	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// Group is the Windows Firewall rule group of all rules that the app makes.
const Group = "CoDServerPicker"

const (
	namePrefix = "CoDServerPicker "
	maxPerRule = 1000 // remote addresses in one rule
)

// COM properties are 32-bit integers.
const (
	dirOut      int32 = 2
	actionBlock int32 = 0
	protocolUDP int32 = 17
	profilesAll int32 = 0x7FFFFFFF
)

// Block is the set of ranges to block for one location.
type Block struct {
	Location string
	Ranges   []string
}

// Apply removes all rules of the app and then adds one or more rules for each block.
// If program is empty, the rules apply to all programs.
func Apply(program string, blocks []Block) error {
	return withPolicy(func(rules *ole.IDispatch) error {
		if err := removeAll(rules); err != nil {
			return err
		}
		for _, b := range blocks {
			for i := 0; i < len(b.Ranges); i += maxPerRule {
				chunk := b.Ranges[i:min(i+maxPerRule, len(b.Ranges))]
				name := fmt.Sprintf("%s%s #%d", namePrefix, b.Location, i/maxPerRule+1)
				if err := addRule(rules, name, program, chunk); err != nil {
					return fmt.Errorf("%s: %w", name, err)
				}
			}
		}
		return nil
	})
}

// Clear removes all rules of the app.
func Clear() error {
	return withPolicy(removeAll)
}

// Status is the current state of the app rules.
type Status struct {
	Locations []string // location IDs with at least one rule
	Program   string   // program of the rules, "" if they apply to all programs
	Unknown   []string // rule names in the group that the app did not make (old versions)
}

// Current reads the app rules. Administrator rights are not necessary.
func Current() (Status, error) {
	var st Status
	err := withPolicy(func(rules *ole.IDispatch) error {
		seen := map[string]bool{}
		return forEachRule(rules, func(r *ole.IDispatch, name string) error {
			loc, ok := strings.CutPrefix(name, namePrefix)
			if i := strings.LastIndex(loc, " #"); ok && i > 0 {
				loc = loc[:i]
				if !seen[loc] {
					seen[loc] = true
					st.Locations = append(st.Locations, loc)
				}
				if v, err := oleutil.GetProperty(r, "ApplicationName"); err == nil {
					st.Program = v.ToString()
				}
			} else {
				st.Unknown = append(st.Unknown, name)
			}
			return nil
		})
	})
	sort.Strings(st.Locations)
	return st, err
}

func withPolicy(fn func(rules *ole.IDispatch) error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		if oe, ok := err.(*ole.OleError); !ok || oe.Code() != 1 { // S_FALSE: already initialized
			return err
		}
	}
	defer ole.CoUninitialize()

	unknown, err := oleutil.CreateObject("HNetCfg.FwPolicy2")
	if err != nil {
		return fmt.Errorf("open Windows Firewall: %w", err)
	}
	defer unknown.Release()
	policy, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return err
	}
	defer policy.Release()
	v, err := oleutil.GetProperty(policy, "Rules")
	if err != nil {
		return err
	}
	rules := v.ToIDispatch()
	defer rules.Release()
	return fn(rules)
}

func forEachRule(rules *ole.IDispatch, fn func(r *ole.IDispatch, name string) error) error {
	return oleutil.ForEach(rules, func(v *ole.VARIANT) error {
		r := v.ToIDispatch()
		g, err := oleutil.GetProperty(r, "Grouping")
		if err != nil || g.ToString() != Group {
			return nil
		}
		n, err := oleutil.GetProperty(r, "Name")
		if err != nil {
			return err
		}
		return fn(r, n.ToString())
	})
}

func removeAll(rules *ole.IDispatch) error {
	var names []string
	if err := forEachRule(rules, func(_ *ole.IDispatch, name string) error {
		names = append(names, name)
		return nil
	}); err != nil {
		return err
	}
	for _, n := range names {
		if _, err := oleutil.CallMethod(rules, "Remove", n); err != nil {
			return fmt.Errorf("remove rule %q: %w", n, err)
		}
	}
	return nil
}

func addRule(rules *ole.IDispatch, name, program string, ranges []string) error {
	unknown, err := oleutil.CreateObject("HNetCfg.FWRule")
	if err != nil {
		return err
	}
	defer unknown.Release()
	r, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return err
	}
	defer r.Release()
	props := []struct {
		name  string
		value any
	}{
		{"Name", name},
		{"Description", "Made by CoD Server Picker. Blocks CoD servers at one location."},
		{"Grouping", Group},
		{"Direction", dirOut},
		{"Action", actionBlock},
		{"Protocol", protocolUDP},
		{"RemoteAddresses", strings.Join(ranges, ",")},
		{"Profiles", profilesAll},
		{"Enabled", true},
	}
	if program != "" {
		props = append(props, struct {
			name  string
			value any
		}{"ApplicationName", program})
	}
	for _, p := range props {
		if _, err := oleutil.PutProperty(r, p.name, p.value); err != nil {
			return fmt.Errorf("set %s: %w", p.name, err)
		}
	}
	_, err = oleutil.CallMethod(rules, "Add", r)
	return err
}
