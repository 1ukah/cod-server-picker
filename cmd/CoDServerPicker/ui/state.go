package ui

import (
	"regexp"
	"sort"

	"github.com/1ukah/cod-server-picker/internal/firewall"
	"github.com/1ukah/cod-server-picker/internal/regions"
)

// TriState tells how many locations of a group are blocked.
type TriState int

const (
	None TriState = iota
	Some
	All
)

// Selection holds the locations that the user wants to block (the selection) and the
// locations that the firewall blocks now (the applied state).
type Selection struct {
	order    []string
	known    map[string]bool
	selected map[string]bool
	applied  map[string]bool
}

func NewSelection(f *regions.File) *Selection {
	s := &Selection{selected: map[string]bool{}, applied: map[string]bool{}}
	s.SetFile(f)
	return s
}

// SetFile changes the list of known locations. It removes unknown locations from the selection.
func (s *Selection) SetFile(f *regions.File) {
	s.order = f.AllLocationIDs()
	s.known = make(map[string]bool, len(s.order))
	for _, id := range s.order {
		s.known[id] = true
	}
	for id := range s.selected {
		if !s.known[id] {
			delete(s.selected, id)
		}
	}
}

// SetApplied sets the applied state. It does not change the selection.
func (s *Selection) SetApplied(ids []string) {
	s.applied = make(map[string]bool, len(ids))
	for _, id := range ids {
		s.applied[id] = true
	}
}

// Reset makes the selection equal to the applied state.
func (s *Selection) Reset() {
	s.selected = map[string]bool{}
	for id := range s.applied {
		if s.known[id] {
			s.selected[id] = true
		}
	}
}

// Set blocks or allows one location in the selection.
func (s *Selection) Set(id string, blocked bool) {
	if !s.known[id] {
		return
	}
	if blocked {
		s.selected[id] = true
	} else {
		delete(s.selected, id)
	}
}

// SetMany blocks or allows a group of locations in the selection.
func (s *Selection) SetMany(ids []string, blocked bool) {
	for _, id := range ids {
		s.Set(id, blocked)
	}
}

// Replace blocks the given locations and allows all other locations.
func (s *Selection) Replace(ids []string) {
	s.selected = map[string]bool{}
	s.SetMany(ids, true)
}

// ApplyPreset blocks all locations outside the regions that the preset allows.
func (s *Selection) ApplyPreset(f *regions.File, p *regions.Preset) {
	s.Replace(f.BlockedBy(p))
}

func (s *Selection) BlockAll()   { s.Replace(s.order) }
func (s *Selection) UnblockAll() { s.Replace(nil) }

func (s *Selection) Blocked(id string) bool { return s.selected[id] }

// Count returns the number of selected locations in ids.
func (s *Selection) Count(ids []string) int {
	n := 0
	for _, id := range ids {
		if s.selected[id] {
			n++
		}
	}
	return n
}

// State returns the selection state of a group of locations.
func (s *Selection) State(ids []string) TriState {
	switch n := s.Count(ids); {
	case n == 0:
		return None
	case n == len(ids):
		return All
	}
	return Some
}

// Selected returns the selected location IDs in display order.
func (s *Selection) Selected() []string {
	var out []string
	for _, id := range s.order {
		if s.selected[id] {
			out = append(out, id)
		}
	}
	return out
}

// AllSelected reports whether the selection blocks every known location.
func (s *Selection) AllSelected() bool {
	return len(s.order) > 0 && len(s.selected) == len(s.order)
}

// Applied returns the known location IDs that the firewall blocks, in display order.
func (s *Selection) Applied() []string {
	var out []string
	for _, id := range s.order {
		if s.applied[id] {
			out = append(out, id)
		}
	}
	return out
}

// Stale returns the applied location IDs that are not in the server list.
func (s *Selection) Stale() []string {
	var out []string
	for id := range s.applied {
		if !s.known[id] {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// Dirty reports whether the selection is different from the applied state.
func (s *Selection) Dirty() bool {
	if len(s.selected) != len(s.applied) {
		return true
	}
	for id := range s.selected {
		if !s.applied[id] {
			return true
		}
	}
	return false
}

// PingStatus is the state of the ping test of one location.
type PingStatus int

const (
	PingNone PingStatus = iota
	PingOK
	PingFailed
)

type PingEntry struct {
	Status PingStatus
	MS     int
}

// PingLevel is the colour class of a ping result.
type PingLevel int

const (
	LevelNone PingLevel = iota
	LevelGood
	LevelMedium
	LevelBad
	LevelFailed
)

const (
	pingGoodBelow   = 80
	pingMediumBelow = 150
)

// Level returns the colour class: good below 80 ms, medium below 150 ms, bad above that.
func (e PingEntry) Level() PingLevel {
	switch {
	case e.Status == PingNone:
		return LevelNone
	case e.Status == PingFailed:
		return LevelFailed
	case e.MS < pingGoodBelow:
		return LevelGood
	case e.MS < pingMediumBelow:
		return LevelMedium
	}
	return LevelBad
}

// Pings holds the ping results by location ID.
type Pings map[string]PingEntry

// Best returns the lowest result of a group of locations.
// If no location replied, it returns a failed entry when at least one test failed.
func (p Pings) Best(ids []string) PingEntry {
	best := PingEntry{}
	for _, id := range ids {
		e := p[id]
		switch {
		case e.Status == PingOK && (best.Status != PingOK || e.MS < best.MS):
			best = e
		case e.Status == PingFailed && best.Status == PingNone:
			best = e
		}
	}
	return best
}

// buildBlocks makes the firewall blocks for the given location IDs. It ignores unknown IDs.
func buildBlocks(f *regions.File, ids []string) []firewall.Block {
	var blocks []firewall.Block
	for _, id := range ids {
		if l := f.Location(id); l != nil {
			blocks = append(blocks, firewall.Block{Location: id, Ranges: l.Ranges})
		}
	}
	return blocks
}

var versionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// validVersion reports whether v has the form X.Y.Z.
func validVersion(v string) bool {
	return versionPattern.MatchString(v)
}
