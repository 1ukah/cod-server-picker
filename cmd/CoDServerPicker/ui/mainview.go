package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"image/color"

	"github.com/1ukah/cod-server-picker/internal/ping"
	"github.com/1ukah/cod-server-picker/internal/regions"
	"github.com/1ukah/cod-server-picker/internal/update"
)

const (
	pingParallel   = 12
	pingColumn     = 104
	progressWidth  = 140
	locationIndent = 44
)

type regionRow struct {
	id     string
	ids    []string
	check  *widget.Check
	count  *widget.Label
	ping   *widget.Label
	toggle *widget.Button
	body   *fyne.Container
}

type locationRow struct {
	check *widget.Check
	ping  *widget.Label
}

// mainView holds the widgets of the main screen that change after the screen is made.
type mainView struct {
	// updating is true while refresh sets the checkboxes. It stops the change handlers.
	updating  bool
	notices   *fyne.Container
	regions   []*regionRow
	locations map[string]*locationRow
	presets   *widget.Button
	pingBtn   *widget.Button
	apply     *widget.Button
	reset     *widget.Button
	progress  *widget.ProgressBarInfinite
	status    *widget.Label

	recNote   *widget.Entry
	recButton *widget.Button
	recList   *widget.List
	recResult *fyne.Container
}

func (a *App) buildMain() fyne.CanvasObject {
	mv := &mainView{locations: map[string]*locationRow{}}
	a.main = mv

	tabs := container.NewAppTabs(
		container.NewTabItem(a.t("tab.locations"), a.buildLocations(mv)),
		container.NewTabItem(a.t("tab.advanced"), a.buildAdvanced(mv)),
	)
	tabs.SelectIndex(a.tab)
	tabs.OnSelected = func(*container.TabItem) { a.tab = tabs.SelectedIndex() }

	mv.notices = container.NewVBox()
	mv.progress = widget.NewProgressBarInfinite()
	mv.progress.Stop()
	mv.progress.Hide()
	mv.status = widget.NewLabel(a.status)
	mv.status.Truncation = fyne.TextTruncateEllipsis

	top := container.NewVBox(a.topBar(), mv.notices)
	progressBox := container.NewGridWrap(fyne.NewSize(progressWidth, mv.status.MinSize().Height), mv.progress)
	bottom := container.NewVBox(widget.NewSeparator(), container.NewBorder(nil, nil, nil, progressBox, mv.status))
	a.refresh()
	return container.NewBorder(top, bottom, nil, nil, tabs)
}

func (a *App) buildLocations(mv *mainView) fyne.CanvasObject {
	mv.presets = widget.NewButtonWithIcon(a.t("presets"), theme.MenuDropDownIcon(), a.showPresets)
	mv.presets.IconPlacement = widget.ButtonIconTrailingText
	mv.pingBtn = widget.NewButtonWithIcon(a.t("ping.test"), theme.ViewRefreshIcon(), a.testPing)
	toolbar := container.NewBorder(nil, nil, mv.presets, mv.pingBtn)

	list := container.NewVBox()
	for i := range a.file.Regions {
		list.Add(a.buildRegion(mv, &a.file.Regions[i]))
	}

	mv.reset = widget.NewButton(a.t("reset"), func() {
		a.sel.Reset()
		a.refresh()
	})
	mv.apply = widget.NewButton(a.t("apply"), a.onApply)
	mv.apply.Importance = widget.HighImportance
	buttons := container.NewHBox(layout.NewSpacer(), mv.reset, mv.apply)

	top := container.NewVBox(a.buildInfo(), toolbar, widget.NewSeparator())
	return container.NewBorder(top, buttons, nil, nil, container.NewVScroll(list))
}

// buildInfo shows the game status and the server list information.
func (a *App) buildInfo() fyne.CanvasObject {
	box := container.NewVBox()
	note := ""
	switch {
	case a.settings.AllPrograms:
		box.Add(widget.NewLabel(a.t("game.allPrograms")))
	case a.be.gameValid(a.settings.GamePath):
		box.Add(widget.NewLabel(a.t("game.found")))
		note = a.settings.GamePath + "\n"
	default:
		warning := widget.NewLabelWithStyle(a.t("game.notFound"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		warning.Importance = widget.WarningImportance
		fix := widget.NewButton(a.t("game.find"), a.findGameAgain)
		box.Add(container.NewBorder(nil, nil, nil, fix, warning))
	}
	source := a.t("list.builtIn")
	if a.source == regions.SourceCache {
		source = a.t("list.downloaded")
	}
	date := a.file.Generated.Local().Format("2006-01-02")
	box.Add(smallText(note+a.t("list.info", source, date), fyne.TextWrapBreak))
	return box
}

func newPingLabel() (*widget.Label, fyne.CanvasObject) {
	l := widget.NewLabel("-")
	l.Alignment = fyne.TextAlignTrailing
	l.Importance = widget.LowImportance
	return l, container.NewGridWrap(fyne.NewSize(pingColumn, l.MinSize().Height), l)
}

func (a *App) buildRegion(mv *mainView, r *regions.Region) fyne.CanvasObject {
	row := &regionRow{id: r.ID, ids: r.LocationIDs(), body: container.NewVBox()}
	for _, l := range r.Locations {
		id := l.ID
		lr := &locationRow{}
		lr.check = widget.NewCheck(l.Name, func(blocked bool) {
			if mv.updating {
				return
			}
			a.sel.Set(id, blocked)
			a.refresh()
		})
		var pingBox fyne.CanvasObject
		lr.ping, pingBox = newPingLabel()
		country := widget.NewLabel(l.Country)
		country.Importance = widget.LowImportance
		indent := canvas.NewRectangle(color.Transparent)
		indent.SetMinSize(fyne.NewSize(locationIndent, 0))
		row.body.Add(container.NewBorder(nil, nil, indent, container.NewHBox(country, pingBox), lr.check))
		mv.locations[id] = lr
	}

	row.check = widget.NewCheck(a.tr.Name("region.", r.ID, r.Name), func(blocked bool) {
		if mv.updating {
			return
		}
		a.sel.SetMany(row.ids, blocked)
		a.refresh()
	})
	row.count = widget.NewLabel("")
	row.count.Importance = widget.LowImportance
	var pingBox fyne.CanvasObject
	row.ping, pingBox = newPingLabel()
	row.toggle = widget.NewButtonWithIcon("", theme.MenuExpandIcon(), nil)
	row.toggle.Importance = widget.LowImportance
	row.toggle.OnTapped = func() {
		a.expanded[row.id] = !a.expanded[row.id]
		a.showExpanded(row)
	}
	a.showExpanded(row)
	mv.regions = append(mv.regions, row)

	header := container.NewBorder(nil, nil, row.toggle, container.NewHBox(row.count, pingBox), row.check)
	return container.NewVBox(header, row.body, widget.NewSeparator())
}

func (a *App) showExpanded(row *regionRow) {
	if a.expanded[row.id] {
		row.toggle.SetIcon(theme.MenuDropDownIcon())
		row.body.Show()
	} else {
		row.toggle.SetIcon(theme.MenuExpandIcon())
		row.body.Hide()
	}
}

// refresh updates the main screen from the app state.
func (a *App) refresh() {
	mv := a.main
	if mv == nil {
		return
	}
	mv.updating = true
	defer func() { mv.updating = false }()

	for _, row := range mv.regions {
		n := a.sel.Count(row.ids)
		row.check.SetChecked(n == len(row.ids))
		text := ""
		if n > 0 {
			text = a.t("blocked.count", n, len(row.ids))
		}
		row.count.SetText(text)
	}
	for id, lr := range mv.locations {
		lr.check.SetChecked(a.sel.Blocked(id))
	}
	a.refreshPings()

	stale := len(a.sel.Stale()) > 0 || a.oldRules > 0
	setEnabled(mv.presets, !a.busy)
	setEnabled(mv.pingBtn, !a.pinging)
	setEnabled(mv.reset, !a.busy && a.sel.Dirty())
	setEnabled(mv.apply, a.admin && !a.busy && (a.sel.Dirty() || stale))

	if a.busy || a.pinging {
		mv.progress.Show()
		mv.progress.Start()
	} else {
		mv.progress.Stop()
		mv.progress.Hide()
	}

	mv.notices.RemoveAll()
	if !a.admin {
		mv.notices.Add(a.adminNotice())
	}
	if a.release != nil {
		url := a.release.URL
		if url == "" {
			url = update.ReleasesURL
		}
		open := widget.NewButton(a.t("notice.update.open"), func() { a.openURL(url) })
		mv.notices.Add(container.NewBorder(nil, nil, nil, open, widget.NewLabel(a.t("notice.update", a.release.Version))))
	}
	if stale {
		mv.notices.Add(wrapLabel(a.t("notice.oldRules")))
	}
	mv.notices.Refresh()
	a.refreshAdvanced()
}

func setEnabled(w fyne.Disableable, enabled bool) {
	if enabled {
		w.Enable()
	} else {
		w.Disable()
	}
}

func (a *App) refreshPings() {
	mv := a.main
	if mv == nil {
		return
	}
	for id, lr := range mv.locations {
		a.setPing(lr.ping, a.pings[id])
	}
	for _, row := range mv.regions {
		a.setPing(row.ping, a.pings.Best(row.ids))
	}
}

func (a *App) setPing(l *widget.Label, e PingEntry) {
	text := "-"
	importance := widget.LowImportance
	switch e.Level() {
	case LevelGood:
		importance = widget.SuccessImportance
	case LevelMedium:
		importance = widget.WarningImportance
	case LevelBad:
		importance = widget.DangerImportance
	}
	switch e.Status {
	case PingOK:
		text = a.t("ping.ms", e.MS)
	case PingFailed:
		text = a.t("ping.noReply")
	}
	if l.Text == text && l.Importance == importance {
		return
	}
	l.Importance = importance
	l.SetText(text)
	l.Refresh()
}

func (a *App) showPresets() {
	var items []*fyne.MenuItem
	for i := range a.file.Presets {
		p := &a.file.Presets[i]
		items = append(items, fyne.NewMenuItem(a.tr.Name("preset.", p.ID, p.Name), func() {
			a.sel.ApplyPreset(a.file, p)
			a.refresh()
		}))
	}
	items = append(items,
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem(a.t("presets.blockAll"), func() {
			a.sel.BlockAll()
			a.refresh()
		}),
		fyne.NewMenuItem(a.t("presets.unblockAll"), func() {
			a.sel.UnblockAll()
			a.refresh()
		}),
	)
	btn := a.main.presets
	widget.ShowPopUpMenuAtRelativePosition(fyne.NewMenu("", items...), a.win.Canvas(), fyne.NewPos(0, btn.Size().Height), btn)
}

func (a *App) testPing() {
	if a.pinging {
		return
	}
	var targets []ping.Target
	for _, r := range a.file.Regions {
		for _, l := range r.Locations {
			targets = append(targets, ping.Target{ID: l.ID, IPs: l.PingIPs, URLs: l.PingURLs})
		}
	}
	a.pinging = true
	a.pings = Pings{}
	a.setStatus(a.t("status.pingRunning"))
	a.refresh()
	a.run(func() {
		a.be.measureAll(a.ctx, targets, pingParallel, func(id string, r ping.Result, err error) {
			e := PingEntry{Status: PingOK, MS: int(r.RTT.Milliseconds())}
			if err != nil {
				e = PingEntry{Status: PingFailed}
			}
			fyne.Do(func() {
				a.pings[id] = e
				a.refreshPings()
			})
		})
		fyne.Do(func() {
			a.pinging = false
			a.setStatus(a.t("status.pingDone"))
			a.refresh()
		})
	})
}

// onApply asks the necessary questions and then applies the selection.
func (a *App) onApply() {
	ids := a.sel.Selected()
	apply := func() { a.applyIDs(ids, true, "") }
	checkRunning := func() {
		if a.be.gameRunning() {
			a.confirm(a.t("apply.title"), a.t("apply.confirmRunning"), a.t("apply"), apply)
			return
		}
		apply()
	}
	if a.sel.AllSelected() {
		a.confirm(a.t("apply.title"), a.t("apply.confirmAll"), a.t("apply"), checkRunning)
		return
	}
	checkRunning()
}
