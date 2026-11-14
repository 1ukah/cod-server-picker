package ui

import (
	"reflect"
	"testing"

	"github.com/1ukah/cod-server-picker/internal/regions"
)

func testFile() *regions.File {
	loc := func(id string) regions.Location {
		return regions.Location{ID: id, Name: id, Country: "XX", Ranges: []string{"192.0.2.0/24"}}
	}
	return &regions.File{
		Schema: regions.Schema,
		Regions: []regions.Region{
			{ID: "r1", Name: "Region 1", Locations: []regions.Location{loc("a"), loc("b")}},
			{ID: "r2", Name: "Region 2", Locations: []regions.Location{loc("c")}},
			{ID: "r3", Name: "Region 3", Locations: []regions.Location{loc("d"), loc("e")}},
		},
		Presets: []regions.Preset{{ID: "p1", Name: "Region 1 only", Allow: []string{"r1"}}},
	}
}

func TestSelectionDirtyAndReset(t *testing.T) {
	s := NewSelection(testFile())
	if s.Dirty() {
		t.Fatal("new selection is dirty")
	}
	s.SetApplied([]string{"a", "c"})
	if !s.Dirty() {
		t.Fatal("selection is equal to the applied state before Reset")
	}
	s.Reset()
	if s.Dirty() || !reflect.DeepEqual(s.Selected(), []string{"a", "c"}) {
		t.Fatalf("after Reset: dirty=%v selected=%v", s.Dirty(), s.Selected())
	}
	s.Set("b", true)
	if !s.Dirty() {
		t.Fatal("a new block does not make the selection dirty")
	}
	s.Set("b", false)
	if s.Dirty() {
		t.Fatal("the selection is dirty after the change is undone")
	}
	// The same count with different locations is also a difference.
	s.Set("a", false)
	s.Set("b", true)
	if !s.Dirty() {
		t.Fatal("a different location with the same count is not dirty")
	}
	s.Reset()
	if s.Dirty() {
		t.Fatal("Reset does not discard the changes")
	}
}

func TestSelectionIgnoresUnknownLocations(t *testing.T) {
	s := NewSelection(testFile())
	s.Set("zz", true)
	if len(s.Selected()) != 0 {
		t.Fatalf("unknown location selected: %v", s.Selected())
	}
	s.SetApplied([]string{"a", "old"})
	s.Reset()
	if !reflect.DeepEqual(s.Selected(), []string{"a"}) {
		t.Fatalf("selected = %v", s.Selected())
	}
	if !reflect.DeepEqual(s.Stale(), []string{"old"}) {
		t.Fatalf("stale = %v", s.Stale())
	}
	if !reflect.DeepEqual(s.Applied(), []string{"a"}) {
		t.Fatalf("applied = %v", s.Applied())
	}
	if !s.Dirty() {
		t.Fatal("a stale rule must make the selection dirty, so that Apply removes it")
	}
}

func TestSelectionRegionState(t *testing.T) {
	f := testFile()
	s := NewSelection(f)
	r1 := f.Regions[0].LocationIDs()
	if s.State(r1) != None {
		t.Fatal("want None")
	}
	s.Set("a", true)
	if s.State(r1) != Some || s.Count(r1) != 1 {
		t.Fatal("want Some")
	}
	s.SetMany(r1, true)
	if s.State(r1) != All {
		t.Fatal("want All")
	}
	s.SetMany(r1, false)
	if s.State(r1) != None {
		t.Fatal("want None after the region is allowed")
	}
}

func TestSelectionPresets(t *testing.T) {
	f := testFile()
	s := NewSelection(f)
	s.Set("a", true)
	s.ApplyPreset(f, &f.Presets[0])
	if !reflect.DeepEqual(s.Selected(), []string{"c", "d", "e"}) {
		t.Fatalf("preset selected %v", s.Selected())
	}
	if s.AllSelected() {
		t.Fatal("a preset must not block all locations")
	}
	s.BlockAll()
	if !s.AllSelected() || len(s.Selected()) != 5 {
		t.Fatalf("BlockAll selected %v", s.Selected())
	}
	s.UnblockAll()
	if s.AllSelected() || len(s.Selected()) != 0 {
		t.Fatalf("UnblockAll selected %v", s.Selected())
	}
}

func TestSelectionSetFile(t *testing.T) {
	f := testFile()
	s := NewSelection(f)
	s.BlockAll()
	next := testFile()
	next.Regions = next.Regions[:2]
	s.SetFile(next)
	if !reflect.DeepEqual(s.Selected(), []string{"a", "b", "c"}) {
		t.Fatalf("selected = %v", s.Selected())
	}
	if !s.AllSelected() {
		t.Fatal("all locations of the new file are selected")
	}
}

func TestPingLevel(t *testing.T) {
	cases := []struct {
		entry PingEntry
		want  PingLevel
	}{
		{PingEntry{}, LevelNone},
		{PingEntry{Status: PingFailed}, LevelFailed},
		{PingEntry{Status: PingOK, MS: 0}, LevelGood},
		{PingEntry{Status: PingOK, MS: 79}, LevelGood},
		{PingEntry{Status: PingOK, MS: 80}, LevelMedium},
		{PingEntry{Status: PingOK, MS: 149}, LevelMedium},
		{PingEntry{Status: PingOK, MS: 150}, LevelBad},
		{PingEntry{Status: PingOK, MS: 900}, LevelBad},
	}
	for _, c := range cases {
		if got := c.entry.Level(); got != c.want {
			t.Errorf("%+v: level %d, want %d", c.entry, got, c.want)
		}
	}
}

func TestPingsBest(t *testing.T) {
	p := Pings{
		"a": {Status: PingOK, MS: 120},
		"b": {Status: PingOK, MS: 40},
		"c": {Status: PingFailed},
	}
	if got := p.Best([]string{"a", "b", "c"}); got.Status != PingOK || got.MS != 40 {
		t.Errorf("best = %+v", got)
	}
	if got := p.Best([]string{"c", "d"}); got.Status != PingFailed {
		t.Errorf("best of failed and not tested = %+v", got)
	}
	if got := p.Best([]string{"d"}); got.Status != PingNone {
		t.Errorf("best of not tested = %+v", got)
	}
}

func TestBuildBlocks(t *testing.T) {
	blocks := buildBlocks(testFile(), []string{"a", "zz", "d"})
	if len(blocks) != 2 || blocks[0].Location != "a" || blocks[1].Location != "d" {
		t.Fatalf("blocks = %+v", blocks)
	}
	if len(blocks[0].Ranges) != 1 {
		t.Fatalf("ranges = %v", blocks[0].Ranges)
	}
}

func TestValidVersion(t *testing.T) {
	for v, want := range map[string]bool{"0.1.0": true, "12.3.45": true, "dev": false, "v1.2.3": false, "1.2": false, "": false} {
		if validVersion(v) != want {
			t.Errorf("validVersion(%q) = %v", v, !want)
		}
	}
}
