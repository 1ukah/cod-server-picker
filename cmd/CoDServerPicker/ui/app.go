// Package ui is the desktop user interface of CoD Server Picker.
//
// It is a separate package because the Windows resource of package main asks for
// administrator rights. A test program of package main cannot start without them.
package ui

import (
	"context"
	_ "embed"
	"image/color"
	"net/url"
	"os"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/1ukah/cod-server-picker/internal/audit"
	"github.com/1ukah/cod-server-picker/internal/config"
	"github.com/1ukah/cod-server-picker/internal/firewall"
	"github.com/1ukah/cod-server-picker/internal/game"
	"github.com/1ukah/cod-server-picker/internal/monitor"
	"github.com/1ukah/cod-server-picker/internal/ping"
	"github.com/1ukah/cod-server-picker/internal/regions"
	"github.com/1ukah/cod-server-picker/internal/update"
	"github.com/1ukah/cod-server-picker/internal/winexec"
)

const (
	appID   = "io.github.1ukah.codserverpicker"
	appName = "CoD Server Picker"
	repoURL = "https://github.com/1ukah/cod-server-picker"

	dialogWidth = 420
)

//go:embed assets/icon.png
var iconPNG []byte

// backend holds the functions that use Windows or the network. Tests replace them.
type backend struct {
	fwCurrent           func() (firewall.Status, error)
	fwApply             func(program string, blocks []firewall.Block) error
	fwClear             func() error
	findSteam           func() (string, error)
	findRunning         func() (string, error)
	findFromConnections func(ctx context.Context, timeout time.Duration) (string, error)
	gameRunning         func() bool
	gameValid           func(path string) bool
	auditEnabled        func() (bool, error)
	auditReset          func() error
	saveSettings        func(config.Settings) error
	updateRegions       func(ctx context.Context, dataDir string, current *regions.File) (*regions.File, bool, error)
	latestRelease       func(ctx context.Context) (update.Release, error)
	measureAll          func(ctx context.Context, targets []ping.Target, parallel int, onResult func(string, ping.Result, error))
	record              func(ctx context.Context, note string, known *regions.File, onNew func(monitor.Server)) (*monitor.Recording, error)
	openFolder          func(dir string)
}

func realBackend() backend {
	return backend{
		fwCurrent:           firewall.Current,
		fwApply:             firewall.Apply,
		fwClear:             firewall.Clear,
		findSteam:           game.FindSteam,
		findRunning:         game.FindRunning,
		findFromConnections: game.FindFromConnections,
		gameRunning:         game.IsRunning,
		gameValid:           game.Valid,
		auditEnabled:        audit.Enabled,
		auditReset:          audit.Reset,
		saveSettings:        config.Save,
		updateRegions: func(ctx context.Context, dataDir string, current *regions.File) (*regions.File, bool, error) {
			return regions.Update(ctx, regions.DefaultUpdateURL, dataDir, current)
		},
		latestRelease: update.Latest,
		measureAll:    ping.MeasureAll,
		record: func(ctx context.Context, note string, known *regions.File, onNew func(monitor.Server)) (*monitor.Recording, error) {
			return monitor.Run(ctx, game.ExeName, note, known, onNew)
		},
		openFolder: func(dir string) {
			winexec.Command("explorer.exe", dir).Start()
		},
	}
}

type screenID int

const (
	screenSetup screenID = iota
	screenMain
)

type options struct {
	version  string
	backend  backend
	admin    bool
	settings config.Settings
	dataDir  string
	file     *regions.File
	source   regions.Source
}

// App is the state of the user interface. Only the UI goroutine reads or writes its fields.
type App struct {
	version string
	be      backend
	admin   bool
	fy      fyne.App
	win     fyne.Window
	tr      *Translator

	settings config.Settings
	dataDir  string
	file     *regions.File
	source   regions.Source
	sel      *Selection
	pings    Pings

	ctx    context.Context
	cancel context.CancelFunc
	// bg counts the tasks that must stop before the app exits.
	bg sync.WaitGroup

	screen   screenID
	setup    setupState
	main     *mainView
	tab      int
	expanded map[string]bool

	busy      bool
	afterBusy []func()
	pinging   bool
	status    string
	oldRules  int
	release   *update.Release
	rec       recorder

	settingsDlg *dialog.CustomDialog
	langButton  *widget.Button
}

// Run starts the app and returns when the user closes the window.
func Run(version string) {
	admin := isAdmin()
	fy := app.NewWithID(appID)
	fy.SetIcon(fyne.NewStaticResource("icon.png", iconPNG))

	settings, _ := config.Load()
	dir, _ := config.Dir()
	file, source, err := regions.Load(dir)
	if err != nil {
		w := fy.NewWindow(appName)
		w.SetContent(widget.NewLabel(err.Error()))
		w.ShowAndRun()
		return
	}
	a, err := newApp(fy, options{
		version:  version,
		backend:  realBackend(),
		admin:    admin,
		settings: settings,
		dataDir:  dir,
		file:     file,
		source:   source,
	})
	if err != nil {
		w := fy.NewWindow(appName)
		w.SetContent(widget.NewLabel(err.Error()))
		w.ShowAndRun()
		return
	}
	// Fyne 2.8 can stay in its run loop after the window closes. This makes sure that the app exits.
	a.win.SetOnClosed(func() {
		go func() {
			a.shutdown()
			os.Exit(0)
		}()
	})
	a.win.Show()
	a.startup()
	fy.Run()
	a.shutdown()
}

func newApp(fy fyne.App, o options) (*App, error) {
	tr, err := NewTranslator()
	if err != nil {
		return nil, err
	}
	tr.SetLang(o.settings.Language)
	o.settings.Language = tr.Lang()

	ctx, cancel := context.WithCancel(context.Background())
	a := &App{
		version:  o.version,
		be:       o.backend,
		admin:    o.admin,
		fy:       fy,
		tr:       tr,
		settings: o.settings,
		dataDir:  o.dataDir,
		file:     o.file,
		source:   o.source,
		sel:      NewSelection(o.file),
		pings:    Pings{},
		ctx:      ctx,
		cancel:   cancel,
		expanded: map[string]bool{},
	}
	a.screen = screenMain
	if a.settings.Platform == "" {
		a.screen = screenSetup
	}
	a.win = fy.NewWindow(appName)
	a.win.Resize(fyne.NewSize(560, 720))
	a.rebuild()
	return a, nil
}

// startup reads the firewall rules and looks for updates in the background.
func (a *App) startup() {
	updateRegions := a.settings.UpdateRegions
	checkApp := a.settings.CheckAppUpdates && validVersion(a.version)
	current := a.file
	a.run(func() {
		a.loadRules(true)
		if updateRegions {
			a.updateRegions(current)
		}
		if checkApp {
			a.checkAppUpdate()
		}
	})
}

// shutdown stops the background tasks. A recording restores the audit policy when it stops.
func (a *App) shutdown() {
	a.cancel()
	done := make(chan struct{})
	go func() {
		a.bg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(8 * time.Second):
	}
}

// run starts a background task. The app waits for these tasks when it exits.
func (a *App) run(task func()) {
	a.bg.Add(1)
	go func() {
		defer a.bg.Done()
		task()
	}()
}

func (a *App) t(key string, args ...any) string {
	return a.tr.T(key, args...)
}

// rebuild makes the window content again, for example after a language change.
func (a *App) rebuild() {
	if a.screen == screenSetup {
		a.main = nil
		a.win.SetContent(a.buildSetup())
		return
	}
	a.win.SetContent(a.buildMain())
}

func (a *App) save() {
	if err := a.be.saveSettings(a.settings); err != nil {
		a.setStatus(a.t("status.saveFailed"))
	}
}

func (a *App) setStatus(text string) {
	a.status = text
	if a.main != nil {
		a.main.status.SetText(text)
	}
}

func (a *App) blockedStatus() string {
	switch n := len(a.sel.Applied()); n {
	case 0:
		return a.t("status.blocked.none")
	case 1:
		return a.t("status.blocked.one")
	default:
		return a.t("status.blocked.many", n)
	}
}

func (a *App) platformName(p game.Platform) string {
	if p == "" {
		return a.t("settings.platform.none")
	}
	return a.tr.Name("platform.", string(p), string(p))
}

// program returns the program for the firewall rules. An empty program means all programs.
func (a *App) program() (string, bool) {
	if a.settings.AllPrograms {
		return "", true
	}
	if a.be.gameValid(a.settings.GamePath) {
		return a.settings.GamePath, true
	}
	return "", false
}

// loadRules reads the firewall rules. Call it from a background goroutine.
func (a *App) loadRules(reset bool) {
	st, err := a.be.fwCurrent()
	fyne.Do(func() {
		if err != nil {
			a.setStatus(a.t("error.read"))
			return
		}
		a.setRules(st, reset)
		a.setStatus(a.blockedStatus())
		a.refresh()
	})
}

func (a *App) setRules(st firewall.Status, reset bool) {
	a.sel.SetApplied(st.Locations)
	a.oldRules = len(st.Unknown)
	if reset {
		a.sel.Reset()
	}
}

func (a *App) setBusy(busy bool) {
	a.busy = busy
	if busy {
		a.refresh()
		return
	}
	next := a.afterBusy
	a.afterBusy = nil
	for _, fn := range next {
		fn()
	}
}

// applyIDs replaces the firewall rules with blocks for the given locations.
// If reset is true, the selection becomes equal to the new applied state.
func (a *App) applyIDs(ids []string, reset bool, doneKey string) {
	if !a.admin {
		return
	}
	if a.busy {
		a.afterBusy = append(a.afterBusy, func() { a.applyIDs(ids, reset, doneKey) })
		return
	}
	program, ok := a.program()
	if !ok {
		a.showMessage(a.t("error.title"), a.t("error.noGame"))
		return
	}
	blocks := buildBlocks(a.file, ids)
	a.setBusy(true)
	a.run(func() {
		err := a.be.fwApply(program, blocks)
		st, readErr := a.be.fwCurrent()
		fyne.Do(func() {
			if readErr == nil {
				a.setRules(st, reset && err == nil)
			}
			if err != nil {
				a.showError(a.t("error.apply"), err)
			} else if doneKey != "" {
				a.setStatus(a.t(doneKey))
			} else {
				a.setStatus(a.blockedStatus())
			}
			a.setBusy(false)
			a.refresh()
		})
	})
}

// reapply writes the applied blocks again, for example after the program or the ranges change.
func (a *App) reapply(doneKey string) {
	if ids := a.sel.Applied(); len(ids) > 0 {
		a.applyIDs(ids, false, doneKey)
	}
}

func (a *App) clearRules() {
	if !a.admin || a.busy {
		return
	}
	a.setBusy(true)
	a.run(func() {
		err := a.be.fwClear()
		st, readErr := a.be.fwCurrent()
		fyne.Do(func() {
			if readErr == nil {
				a.setRules(st, err == nil)
			}
			if err != nil {
				a.showError(a.t("error.apply"), err)
			} else {
				a.setStatus(a.t("status.cleared"))
			}
			a.setBusy(false)
			a.refresh()
		})
	})
}

// updateRegions downloads a newer server list. Call it from a background goroutine.
func (a *App) updateRegions(current *regions.File) {
	f, updated, err := a.be.updateRegions(a.ctx, a.dataDir, current)
	if err != nil || !updated {
		return
	}
	fyne.Do(func() {
		a.file = f
		a.source = regions.SourceCache
		a.sel.SetFile(f)
		a.rebuild()
		if len(a.sel.Applied()) > 0 && a.admin {
			a.reapply("status.listUpdatedBlocks")
			return
		}
		a.setStatus(a.t("status.listUpdated"))
	})
}

// checkAppUpdate looks for a newer release. Call it from a background goroutine.
func (a *App) checkAppUpdate() {
	latest, err := a.be.latestRelease(a.ctx)
	if err != nil || !update.Newer(latest.Version, a.version) {
		return
	}
	fyne.Do(func() {
		a.release = &latest
		a.refresh()
	})
}

func (a *App) openURL(s string) {
	if u, err := url.Parse(s); err == nil {
		a.fy.OpenURL(u)
	}
}

// topBar shows the app name and the settings button.
func (a *App) topBar() fyne.CanvasObject {
	title := widget.NewLabelWithStyle(appName, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	gear := widget.NewButtonWithIcon("", theme.SettingsIcon(), a.showSettings)
	return container.NewBorder(nil, nil, title, gear)
}

func (a *App) adminNotice() fyne.CanvasObject {
	l := widget.NewLabelWithStyle(a.t("admin.notice"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	l.Importance = widget.WarningImportance
	l.Wrapping = fyne.TextWrapWord
	return l
}

func wrapLabel(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.Wrapping = fyne.TextWrapWord
	return l
}

func boldLabel(text string) *widget.Label {
	l := widget.NewLabelWithStyle(text, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	l.Wrapping = fyne.TextWrapWord
	return l
}

// smallText is a note in small grey text.
func smallText(text string, wrap fyne.TextWrap) *widget.RichText {
	rt := widget.NewRichText(&widget.TextSegment{Text: text, Style: widget.RichTextStyle{
		ColorName: theme.ColorNamePlaceHolder,
		SizeName:  theme.SizeNameCaptionText,
	}})
	rt.Wrapping = wrap
	return rt
}

// minWidth gives obj a minimum width, so that text with word wrap has space.
func minWidth(width float32, obj fyne.CanvasObject) fyne.CanvasObject {
	spacer := canvas.NewRectangle(color.Transparent)
	spacer.SetMinSize(fyne.NewSize(width, 0))
	return container.NewStack(spacer, obj)
}

func (a *App) showMessage(title, text string) {
	dialog.NewCustom(title, a.t("ok"), minWidth(dialogWidth, wrapLabel(text)), a.win).Show()
}

// showError shows a plain message and the technical error in small text.
func (a *App) showError(text string, err error) {
	content := container.NewVBox(wrapLabel(text))
	if err != nil {
		content.Add(smallText(a.t("error.detail", err.Error()), fyne.TextWrapBreak))
	}
	dialog.NewCustom(a.t("error.title"), a.t("ok"), minWidth(dialogWidth, content), a.win).Show()
}

func (a *App) confirm(title, text, okText string, onOK func()) {
	dialog.NewCustomConfirm(title, okText, a.t("cancel"), minWidth(dialogWidth, wrapLabel(text)), func(ok bool) {
		if ok {
			onOK()
		}
	}, a.win).Show()
}
