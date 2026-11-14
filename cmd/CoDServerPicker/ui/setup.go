package ui

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"github.com/1ukah/cod-server-picker/internal/game"
)

const (
	searchInterval      = 2 * time.Second
	searchFallbackAfter = 20 * time.Second
	searchFallbackTime  = 5 * time.Minute
)

type setupStep int

const (
	stepPlatform setupStep = iota
	stepMethod
	stepSearching
)

// setupState is the state of the platform setup.
type setupState struct {
	step        setupStep
	platform    game.Platform
	allPrograms bool
	steamFailed bool
	// rerun is true when the user starts the setup from the main screen.
	rerun bool
	// cancel stops the automatic search. It is nil when no search runs.
	cancel context.CancelFunc
	search int
}

var platforms = []game.Platform{game.Steam, game.BattleNet, game.Xbox, game.Other}

func (a *App) buildSetup() fyne.CanvasObject {
	var body fyne.CanvasObject
	switch a.setup.step {
	case stepMethod:
		body = a.buildSetupMethod()
	case stepSearching:
		body = a.buildSetupSearching()
	default:
		body = a.buildSetupPlatform()
	}
	top := container.NewVBox(a.topBar())
	if !a.admin {
		top.Add(a.adminNotice())
	}
	top.Add(widget.NewSeparator())
	return container.NewBorder(top, nil, nil, nil, container.NewPadded(body))
}

func (a *App) buildSetupPlatform() fyne.CanvasObject {
	box := container.NewVBox(boldLabel(a.t("setup.platform")))
	for _, p := range platforms {
		box.Add(widget.NewButton(a.platformName(p), func() { a.selectPlatform(p) }))
	}
	if a.setup.rerun {
		box.Add(widget.NewSeparator())
		box.Add(widget.NewButton(a.t("cancel"), a.leaveSetup))
	}
	return box
}

func (a *App) buildSetupMethod() fyne.CanvasObject {
	box := container.NewVBox()
	if a.setup.steamFailed {
		box.Add(wrapLabel(a.t("setup.steamFailed")))
	}
	box.Add(boldLabel(a.t("setup.method")))
	box.Add(widget.NewButton(a.t("setup.selectFile"), a.pickGameFile))
	box.Add(widget.NewButton(a.t("setup.auto"), a.startSearch))

	if a.setup.platform == game.Xbox {
		box.Add(widget.NewSeparator())
		box.Add(a.xboxWarning())
		allPrograms := widget.NewCheck(a.t("xbox.allPrograms"), func(on bool) {
			a.setup.allPrograms = on
			a.rebuild()
		})
		allPrograms.Checked = a.setup.allPrograms
		box.Add(allPrograms)
		box.Add(smallText(a.t("allPrograms.explain"), fyne.TextWrapWord))
		if a.setup.allPrograms {
			box.Add(widget.NewButton(a.t("xbox.continue"), func() { a.finishSetup("") }))
		}
	}
	box.Add(widget.NewSeparator())
	box.Add(widget.NewButton(a.t("back"), func() {
		a.setup.step = stepPlatform
		a.setup.steamFailed = false
		a.rebuild()
	}))
	return box
}

func (a *App) xboxWarning() fyne.CanvasObject {
	untested := wrapLabel(a.t("xbox.untested"))
	untested.Importance = widget.WarningImportance
	return container.NewVBox(untested, wrapLabel(a.t("xbox.protected")))
}

func (a *App) buildSetupSearching() fyne.CanvasObject {
	return container.NewVBox(
		boldLabel(a.t("setup.search.title")),
		wrapLabel(a.t("setup.search.step1")),
		wrapLabel(a.t("setup.search.step2")),
		widget.NewProgressBarInfinite(),
		smallText(a.t("setup.search.wait"), fyne.TextWrapWord),
		widget.NewButton(a.t("cancel"), func() {
			a.stopSearch()
			a.setup.step = stepMethod
			a.rebuild()
		}),
	)
}

// startSetup shows the platform setup again. The settings stay the same until it is complete.
func (a *App) startSetup() {
	a.closeSettings()
	a.setup = setupState{rerun: true}
	a.screen = screenSetup
	a.rebuild()
}

// findGameAgain finds the game for the platform that is set.
func (a *App) findGameAgain() {
	if a.settings.Platform == "" {
		a.startSetup()
		return
	}
	a.closeSettings()
	a.setup = setupState{rerun: true}
	a.selectPlatform(a.settings.Platform)
}

func (a *App) leaveSetup() {
	a.stopSearch()
	a.setup = setupState{}
	a.screen = screenMain
	a.rebuild()
}

func (a *App) selectPlatform(p game.Platform) {
	a.setup.platform = p
	a.setup.allPrograms = false
	a.setup.steamFailed = false
	if p != game.Steam {
		a.showMethod()
		return
	}
	a.run(func() {
		path, err := a.be.findSteam()
		fyne.Do(func() {
			if err == nil {
				a.finishSetup(path)
				return
			}
			a.setup.steamFailed = true
			a.showMethod()
		})
	})
}

func (a *App) showMethod() {
	a.setup.step = stepMethod
	a.screen = screenSetup
	a.rebuild()
}

func (a *App) pickGameFile() {
	d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil || r == nil {
			return
		}
		r.Close()
		path := filepath.FromSlash(r.URI().Path())
		if !a.be.gameValid(path) {
			a.showMessage(a.t("error.title"), a.t("setup.invalidFile"))
			return
		}
		a.finishSetup(path)
	}, a.win)
	d.SetFilter(storage.NewExtensionFileFilter([]string{".exe"}))
	d.Resize(fyne.NewSize(540, 600))
	d.Show()
}

// startSearch looks for the running game until it is found or the user cancels.
func (a *App) startSearch() {
	ctx, cancel := context.WithCancel(a.ctx)
	a.setup.search++
	search := a.setup.search
	a.setup.cancel = cancel
	a.setup.step = stepSearching
	a.rebuild()

	admin := a.admin
	report := func(path string) {
		fyne.Do(func() {
			if a.setup.search != search || a.setup.cancel == nil {
				return
			}
			a.stopSearch()
			a.finishSetup(path)
		})
	}
	a.run(func() {
		var fallback sync.WaitGroup
		defer func() {
			cancel()
			fallback.Wait()
		}()
		found := make(chan string, 1)
		tick := time.NewTicker(searchInterval)
		defer tick.Stop()
		startFallback := time.After(searchFallbackAfter)
		for {
			if path, err := a.be.findRunning(); err == nil {
				report(path)
				return
			}
			select {
			case <-ctx.Done():
				return
			case path := <-found:
				report(path)
				return
			case <-startFallback:
				if !admin {
					continue
				}
				// The connection log finds the game if the process list does not.
				fallback.Add(1)
				go func() {
					defer fallback.Done()
					if path, err := a.be.findFromConnections(ctx, searchFallbackTime); err == nil {
						found <- path
					}
				}()
			case <-tick.C:
			}
		}
	})
}

func (a *App) stopSearch() {
	if a.setup.cancel != nil {
		a.setup.cancel()
		a.setup.cancel = nil
	}
}

// finishSetup saves the platform and the game path, and shows the main screen.
func (a *App) finishSetup(path string) {
	changed := path != a.settings.GamePath
	a.settings.Platform = a.setup.platform
	a.settings.GamePath = path
	if a.setup.allPrograms {
		changed = changed || !a.settings.AllPrograms
		a.settings.AllPrograms = true
	}
	a.save()
	a.setup = setupState{}
	a.screen = screenMain
	a.rebuild()

	if path != "" && game.IsProtectedPath(path) && !a.settings.AllPrograms {
		a.showProtectedWarning(changed)
		return
	}
	if changed {
		a.reapply("status.rulesUpdated")
	}
}

// showProtectedWarning offers rules for all programs when Windows protects the game folder.
func (a *App) showProtectedWarning(changed bool) {
	content := container.NewVBox(
		a.xboxWarning(),
		smallText(a.t("allPrograms.explain"), fyne.TextWrapWord),
	)
	dialog.NewCustomConfirm(a.t("protected.title"), a.t("xbox.allPrograms"), a.t("protected.dismiss"),
		minWidth(dialogWidth, content), func(all bool) {
			if all {
				a.settings.AllPrograms = true
				a.save()
				a.rebuild()
				changed = true
			}
			if changed {
				a.reapply("status.rulesUpdated")
			}
		}, a.win).Show()
}
