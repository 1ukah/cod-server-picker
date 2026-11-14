package ui

import (
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/1ukah/cod-server-picker/internal/config"
	"github.com/1ukah/cod-server-picker/internal/update"
)

func (a *App) closeSettings() {
	if a.settingsDlg != nil {
		d := a.settingsDlg
		a.settingsDlg = nil
		d.Hide()
	}
}

func (a *App) showSettings() {
	a.closeSettings()
	a.langButton = a.languageSelector()
	box := container.NewVBox(
		container.NewBorder(nil, nil, widget.NewLabel(a.t("settings.language")), a.langButton),
		a.settingCheck("settings.updateRegions", a.settings.UpdateRegions, func(on bool) { a.settings.UpdateRegions = on }),
		a.settingCheck("settings.updateApp", a.settings.CheckAppUpdates, func(on bool) { a.settings.CheckAppUpdates = on }),
		widget.NewSeparator(),
	)
	for _, section := range []fyne.CanvasObject{a.gameSettings(), a.auditSettings(), a.clearSettings(), a.about()} {
		box.Add(section)
	}
	d := dialog.NewCustom(a.t("settings.title"), a.t("close"), container.NewVScroll(box), a.win)
	d.Resize(fyne.NewSize(500, 640))
	a.settingsDlg = d
	d.Show()
}

// settingCheck is a checkbox that saves the settings after each change.
func (a *App) settingCheck(key string, value bool, set func(bool)) *widget.Check {
	c := widget.NewCheck(a.t(key), func(on bool) {
		set(on)
		a.save()
	})
	c.Checked = value
	return c
}

func (a *App) setLanguage(lang string) {
	if lang == a.settings.Language {
		return
	}
	a.settings.Language = lang
	a.tr.SetLang(lang)
	a.save()
	if a.main != nil {
		a.setStatus(a.blockedStatus())
	}
	a.rebuild()
	a.showSettings()
}

// languageSelector is a button with the flag and the name of the current language.
// It opens a list with one flag and name for each language.
func (a *App) languageSelector() *widget.Button {
	current := a.settings.Language
	btn := widget.NewButtonWithIcon(languageNames[current], flagResource(current), nil)
	btn.Alignment = widget.ButtonAlignLeading
	btn.OnTapped = func() {
		var pop *widget.PopUp
		rows := container.NewVBox()
		for _, lang := range config.Languages {
			row := widget.NewButtonWithIcon(languageNames[lang], flagResource(lang), func() {
				pop.Hide()
				a.setLanguage(lang)
			})
			row.Alignment = widget.ButtonAlignLeading
			row.Importance = widget.LowImportance
			rows.Add(row)
		}
		pop = widget.NewPopUp(rows, a.win.Canvas())
		pop.ShowAtRelativePosition(fyne.NewPos(0, btn.Size().Height), btn)
	}
	return btn
}

func (a *App) gameSettings() fyne.CanvasObject {
	path := a.settings.GamePath
	if path == "" {
		path = a.t("settings.path.none")
	}
	allPrograms := a.settingCheck("settings.allPrograms", a.settings.AllPrograms, func(on bool) { a.settings.AllPrograms = on })
	save := allPrograms.OnChanged
	allPrograms.OnChanged = func(on bool) {
		save(on)
		a.rebuild()
		a.reapply("status.rulesUpdated")
	}
	return container.NewVBox(
		boldLabel(a.t("settings.game")),
		widget.NewLabel(a.t("settings.platform", a.platformName(a.settings.Platform))),
		smallText(path, fyne.TextWrapBreak),
		container.NewGridWithColumns(2,
			widget.NewButton(a.t("settings.changePlatform"), a.startSetup),
			widget.NewButton(a.t("settings.findAgain"), a.findGameAgain),
		),
		allPrograms,
		smallText(a.t("allPrograms.explain"), fyne.TextWrapWord),
		widget.NewSeparator(),
	)
}

// auditSettings has the button that turns off the connection log setting.
// The button is on only when the setting is on and no task of the app uses it.
func (a *App) auditSettings() fyne.CanvasObject {
	btn := widget.NewButton(a.t("settings.audit.button"), nil)
	btn.Disable()
	check := func() {
		if !a.admin {
			return
		}
		a.run(func() {
			on, err := a.be.auditEnabled()
			fyne.Do(func() {
				inUse := a.rec.running || a.setup.cancel != nil
				setEnabled(btn, err == nil && on && !inUse)
			})
		})
	}
	btn.OnTapped = func() {
		btn.Disable()
		a.run(func() {
			err := a.be.auditReset()
			fyne.Do(func() {
				if err != nil {
					a.showError(a.t("error.audit"), err)
				} else {
					a.setStatus(a.t("status.auditReset"))
				}
				check()
			})
		})
	}
	check()
	return container.NewVBox(
		container.NewHBox(btn),
		smallText(a.t("settings.audit.explain"), fyne.TextWrapWord),
	)
}

func (a *App) clearSettings() fyne.CanvasObject {
	btn := widget.NewButton(a.t("settings.clear.button"), func() {
		a.confirm(a.t("settings.clear.button"), a.t("settings.clear.confirm"), a.t("settings.clear.button"), a.clearRules)
	})
	btn.Importance = widget.DangerImportance
	setEnabled(btn, a.admin)
	return container.NewVBox(
		container.NewHBox(btn),
		smallText(a.t("settings.clear.explain"), fyne.TextWrapWord),
		widget.NewSeparator(),
	)
}

func (a *App) about() fyne.CanvasObject {
	return container.NewVBox(
		boldLabel(a.t("about.title")),
		widget.NewLabel(appName+" - "+a.t("about.version", a.version)),
		wrapLabel(a.t("about.description")),
		widget.NewLabel(a.t("about.license")),
		container.NewHBox(
			hyperlink(a.t("about.repo"), repoURL),
			hyperlink(a.t("about.releases"), update.ReleasesURL),
		),
	)
}

func hyperlink(text, target string) fyne.CanvasObject {
	u, err := url.Parse(target)
	if err != nil {
		return widget.NewLabel(text)
	}
	return widget.NewHyperlink(text, u)
}
