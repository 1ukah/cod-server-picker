package ui

import (
	"context"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/1ukah/cod-server-picker/internal/config"
	"github.com/1ukah/cod-server-picker/internal/firewall"
	"github.com/1ukah/cod-server-picker/internal/game"
	"github.com/1ukah/cod-server-picker/internal/monitor"
	"github.com/1ukah/cod-server-picker/internal/ping"
	"github.com/1ukah/cod-server-picker/internal/regions"
	"github.com/1ukah/cod-server-picker/internal/update"
)

const testGamePath = `C:\Games\Call of Duty\cod.exe`

// fakeSystem replaces Windows and the network in the window tests.
type fakeSystem struct {
	mu        sync.Mutex
	rules     firewall.Status
	programs  []string
	saved     []config.Settings
	steamPath string
	running   bool
	auditOn   bool
	newFile   *regions.File
	release   update.Release
}

func (f *fakeSystem) backend() backend {
	return backend{
		fwCurrent: func() (firewall.Status, error) {
			f.mu.Lock()
			defer f.mu.Unlock()
			return f.rules, nil
		},
		fwApply: func(program string, blocks []firewall.Block) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.programs = append(f.programs, program)
			f.rules = firewall.Status{Program: program}
			for _, b := range blocks {
				f.rules.Locations = append(f.rules.Locations, b.Location)
			}
			return nil
		},
		fwClear: func() error {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.rules = firewall.Status{}
			return nil
		},
		findSteam: func() (string, error) {
			if f.steamPath == "" {
				return "", game.ErrNotFound
			}
			return f.steamPath, nil
		},
		findRunning: func() (string, error) { return "", game.ErrNotFound },
		findFromConnections: func(context.Context, time.Duration) (string, error) {
			return "", game.ErrNotFound
		},
		gameRunning:  func() bool { return f.running },
		gameValid:    func(path string) bool { return path == testGamePath },
		auditEnabled: func() (bool, error) { return f.auditOn, nil },
		auditReset: func() error {
			f.auditOn = false
			return nil
		},
		saveSettings: func(s config.Settings) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.saved = append(f.saved, s)
			return nil
		},
		updateRegions: func(_ context.Context, _ string, current *regions.File) (*regions.File, bool, error) {
			if f.newFile == nil {
				return current, false, errors.New("offline")
			}
			return f.newFile, true, nil
		},
		latestRelease: func(context.Context) (update.Release, error) { return f.release, nil },
		measureAll: func(_ context.Context, targets []ping.Target, _ int, onResult func(string, ping.Result, error)) {
			for i, target := range targets {
				if i%7 == 6 {
					onResult(target.ID, ping.Result{}, ping.ErrNoReply)
					continue
				}
				onResult(target.ID, ping.Result{RTT: time.Duration(20+i*9) * time.Millisecond}, nil)
			}
		},
		record: func(ctx context.Context, _ string, _ *regions.File, onNew func(monitor.Server)) (*monitor.Recording, error) {
			onNew(monitor.Server{IP: "203.0.113.7", Proto: "udp"})
			<-ctx.Done()
			return nil, nil
		},
		openFolder: func(string) {},
	}
}

func (f *fakeSystem) lastSaved(t *testing.T) config.Settings {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.saved) == 0 {
		t.Fatal("the settings were not saved")
	}
	return f.saved[len(f.saved)-1]
}

func newTestApp(t *testing.T, f *fakeSystem, settings config.Settings, admin bool) *App {
	t.Helper()
	file, err := regions.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	a, err := newApp(test.NewTempApp(t), options{
		version:  "1.0.0",
		backend:  f.backend(),
		admin:    admin,
		settings: settings,
		dataDir:  t.TempDir(),
		file:     file,
		source:   regions.SourceEmbedded,
	})
	if err != nil {
		t.Fatal(err)
	}
	a.win.Resize(fyne.NewSize(560, 720))
	t.Cleanup(a.shutdown)
	return a
}

func readySettings() config.Settings {
	s := config.Defaults()
	s.Platform = game.Steam
	s.GamePath = testGamePath
	return s
}

// capture writes a picture of the window if CSP_CAPTURE_DIR is set. Use it to check the layout.
func capture(t *testing.T, a *App, name string) {
	t.Helper()
	dir := os.Getenv("CSP_CAPTURE_DIR")
	if dir == "" {
		return
	}
	out, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	if err := png.Encode(out, a.win.Canvas().Capture()); err != nil {
		t.Fatal(err)
	}
}

func TestFirstStartShowsSetup(t *testing.T) {
	f := &fakeSystem{steamPath: testGamePath}
	a := newTestApp(t, f, config.Defaults(), true)
	if a.screen != screenSetup || a.main != nil {
		t.Fatal("the first start must show the platform setup")
	}
	capture(t, a, "setup-platform")

	a.selectPlatform(game.Steam)
	a.bg.Wait()
	if a.screen != screenMain || a.main == nil {
		t.Fatal("Steam with a found game must go to the main screen")
	}
	s := f.lastSaved(t)
	if s.Platform != game.Steam || s.GamePath != testGamePath {
		t.Fatalf("saved settings = %+v", s)
	}
}

func TestSetupWithoutSteamGame(t *testing.T) {
	f := &fakeSystem{}
	a := newTestApp(t, f, config.Defaults(), true)
	a.selectPlatform(game.Steam)
	a.bg.Wait()
	if a.screen != screenSetup || a.setup.step != stepMethod || !a.setup.steamFailed {
		t.Fatalf("setup = %+v", a.setup)
	}
	if len(f.saved) != 0 {
		t.Fatal("the settings must not change before the game is found")
	}

	a.selectPlatform(game.Xbox)
	capture(t, a, "setup-xbox")
	a.setup.allPrograms = true
	a.finishSetup("")
	if s := f.lastSaved(t); s.Platform != game.Xbox || !s.AllPrograms || s.GamePath != "" {
		t.Fatalf("saved settings = %+v", s)
	}
	if program, ok := a.program(); !ok || program != "" {
		t.Fatalf("program = %q, %v", program, ok)
	}
}

func TestSetupSearchCanBeCancelled(t *testing.T) {
	f := &fakeSystem{}
	a := newTestApp(t, f, config.Defaults(), false)
	a.selectPlatform(game.BattleNet)
	a.startSearch()
	if a.setup.step != stepSearching || a.setup.cancel == nil {
		t.Fatal("the search did not start")
	}
	capture(t, a, "setup-search")
	a.stopSearch()
	a.bg.Wait()
	if a.setup.cancel != nil {
		t.Fatal("the search did not stop")
	}
}

func TestMainScreenApply(t *testing.T) {
	f := &fakeSystem{rules: firewall.Status{Locations: []string{"us-virginia"}, Program: testGamePath}}
	a := newTestApp(t, f, readySettings(), true)
	a.loadRules(true)
	mv := a.main

	if !mv.locations["us-virginia"].check.Checked {
		t.Fatal("the blocked location is not preselected")
	}
	if !mv.apply.Disabled() || !mv.reset.Disabled() {
		t.Fatal("Apply and Reset must be off when nothing changed")
	}

	region := mv.regions[len(mv.regions)-1]
	test.Tap(region.toggle)
	test.Tap(region.check)
	if a.sel.State(region.ids) != All || !mv.locations[region.ids[0]].check.Checked {
		t.Fatal("the region checkbox does not block its locations")
	}
	if mv.apply.Disabled() || mv.reset.Disabled() {
		t.Fatal("Apply and Reset must be on after a change")
	}
	capture(t, a, "main-changed")

	test.Tap(mv.reset)
	if a.sel.Dirty() || mv.locations[region.ids[0]].check.Checked {
		t.Fatal("Reset does not discard the changes")
	}

	test.Tap(mv.locations[region.ids[0]].check)
	want := a.sel.Selected()
	test.Tap(mv.apply)
	a.bg.Wait()
	got := append([]string(nil), f.rules.Locations...)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("applied %v, want %v", got, want)
	}
	if len(f.programs) != 1 || f.programs[0] != testGamePath {
		t.Fatalf("programs = %v", f.programs)
	}
	if a.sel.Dirty() || !mv.apply.Disabled() || a.busy {
		t.Fatal("the screen is not up to date after Apply")
	}
	if a.status != a.t("status.blocked.many", 2) {
		t.Fatalf("status = %q", a.status)
	}
}

func TestApplyNeedsTheGame(t *testing.T) {
	f := &fakeSystem{}
	s := readySettings()
	s.GamePath = `C:\missing\cod.exe`
	a := newTestApp(t, f, s, true)
	a.sel.Set("us-virginia", true)
	a.onApply()
	a.bg.Wait()
	if len(f.programs) != 0 {
		t.Fatal("the app must not make rules for all programs when the game is not found")
	}
}

func TestNoAdministratorRights(t *testing.T) {
	f := &fakeSystem{auditOn: true}
	a := newTestApp(t, f, readySettings(), false)
	a.sel.Set("us-virginia", true)
	a.refresh()
	if !a.main.apply.Disabled() || !a.main.recButton.Disabled() {
		t.Fatal("actions that need administrator rights must be off")
	}
	if len(a.main.notices.Objects) == 0 {
		t.Fatal("the administrator message is not shown")
	}
	a.applyIDs([]string{"us-virginia"}, true, "")
	a.clearRules()
	a.bg.Wait()
	if len(f.programs) != 0 {
		t.Fatal("rules changed without administrator rights")
	}
	capture(t, a, "main-no-admin")
}

func TestPresetsAndPing(t *testing.T) {
	f := &fakeSystem{}
	a := newTestApp(t, f, readySettings(), true)
	a.sel.ApplyPreset(a.file, a.file.Preset("south-america"))
	a.refresh()
	for _, row := range a.main.regions {
		if want := row.id != "south-america"; row.check.Checked != want {
			t.Errorf("region %s: checked = %v", row.id, row.check.Checked)
		}
	}
	a.expanded["us-east"] = true
	a.rebuild()
	a.testPing()
	a.bg.Wait()
	if a.pinging || len(a.pings) != len(a.file.AllLocationIDs()) {
		t.Fatalf("ping results: %d", len(a.pings))
	}
	if got := a.main.locations["us-virginia"].ping.Text; got != "20 ms" {
		t.Fatalf("ping text = %q", got)
	}
	capture(t, a, "main-ping")
}

func TestLanguageChange(t *testing.T) {
	f := &fakeSystem{}
	a := newTestApp(t, f, readySettings(), true)
	a.showSettings()
	capture(t, a, "settings-en")
	overlays := len(a.win.Canvas().Overlays().List())
	test.Tap(a.langButton)
	if len(a.win.Canvas().Overlays().List()) != overlays+1 {
		t.Fatal("the language list did not open")
	}
	capture(t, a, "settings-language-list")
	a.win.Canvas().Overlays().Top().Hide()
	for _, lang := range []string{"pt", "es"} {
		a.setLanguage(lang)
		a.bg.Wait()
		if got := a.main.apply.Text; got != a.tr.tables[lang]["apply"] {
			t.Errorf("%s: Apply button text = %q", lang, got)
		}
		if a.settingsDlg == nil {
			t.Errorf("%s: the settings dialog did not open again", lang)
		}
		if s := f.lastSaved(t); s.Language != lang {
			t.Errorf("saved language = %q", s.Language)
		}
		capture(t, a, "settings-"+lang)
	}
	a.closeSettings()
	capture(t, a, "main-es")
}

func TestAllProgramsAppliesBlocksAgain(t *testing.T) {
	f := &fakeSystem{rules: firewall.Status{Locations: []string{"us-virginia"}, Program: testGamePath}}
	a := newTestApp(t, f, readySettings(), true)
	a.loadRules(true)
	a.settings.AllPrograms = true
	a.rebuild()
	a.reapply("status.rulesUpdated")
	a.bg.Wait()
	if len(f.programs) != 1 || f.programs[0] != "" {
		t.Fatalf("programs = %q", f.programs)
	}
	if !reflect.DeepEqual(f.rules.Locations, []string{"us-virginia"}) {
		t.Fatalf("rules = %v", f.rules.Locations)
	}
}

func TestStartupUpdates(t *testing.T) {
	newer, err := regions.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	newer.Generated = newer.Generated.Add(24 * time.Hour)
	f := &fakeSystem{
		rules:   firewall.Status{Locations: []string{"us-virginia"}, Program: testGamePath, Unknown: []string{"old rule"}},
		newFile: newer,
		release: update.Release{Version: "1.2.0", URL: update.ReleasesURL},
	}
	a := newTestApp(t, f, readySettings(), true)
	a.startup()
	a.bg.Wait()
	if a.file != newer || a.source != regions.SourceCache {
		t.Fatal("the new server list is not in use")
	}
	if len(f.programs) != 1 || !reflect.DeepEqual(f.rules.Locations, []string{"us-virginia"}) {
		t.Fatalf("blocks were not applied again: %v %v", f.programs, f.rules.Locations)
	}
	if a.status != a.t("status.listUpdatedBlocks") {
		t.Fatalf("status = %q", a.status)
	}
	if a.release == nil || a.release.Version != "1.2.0" {
		t.Fatal("the update notice is not shown")
	}
	capture(t, a, "main-update")
}

func TestRecording(t *testing.T) {
	f := &fakeSystem{}
	a := newTestApp(t, f, readySettings(), true)
	a.tab = 1
	a.rebuild()
	a.toggleRecording()
	deadline := time.Now().Add(3 * time.Second)
	for {
		done := make(chan int, 1)
		fyne.DoAndWait(func() { done <- len(a.rec.servers) })
		if <-done == 1 || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !a.rec.running {
		t.Fatal("the recording did not start")
	}
	capture(t, a, "advanced-recording")
	a.toggleRecording()
	a.bg.Wait()
	if a.rec.running || a.main.recButton.Text != a.t("record.start") {
		t.Fatal("the recording did not stop")
	}
}

func TestSettingsAudit(t *testing.T) {
	f := &fakeSystem{auditOn: true}
	a := newTestApp(t, f, readySettings(), true)
	a.showSettings()
	a.bg.Wait()
	a.closeSettings()
	a.startSetup()
	if a.screen != screenSetup || !a.setup.rerun {
		t.Fatal("Change platform must show the setup again")
	}
	capture(t, a, "setup-rerun")
	a.leaveSetup()
	if a.screen != screenMain {
		t.Fatal("Cancel must go back to the main screen")
	}
}
