package ui

import (
	"context"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/1ukah/cod-server-picker/internal/monitor"
)

// recorder is the state of the server recording.
type recorder struct {
	running  bool
	stopping bool
	cancel   context.CancelFunc
	note     string
	servers  []monitor.Server
	path     string
}

func (a *App) buildAdvanced(mv *mainView) fyne.CanvasObject {
	mv.recNote = widget.NewEntry()
	mv.recNote.SetPlaceHolder(a.t("record.note"))
	mv.recNote.SetText(a.rec.note)
	mv.recNote.OnChanged = func(s string) { a.rec.note = s }

	mv.recButton = widget.NewButton("", a.toggleRecording)
	mv.recList = widget.NewList(
		func() int { return len(a.rec.servers) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i widget.ListItemID, obj fyne.CanvasObject) {
			if i < len(a.rec.servers) {
				obj.(*widget.Label).SetText(a.serverLine(a.rec.servers[i]))
			}
		},
	)
	mv.recResult = container.NewVBox()

	top := container.NewVBox(
		boldLabel(a.t("record.title")),
		wrapLabel(a.t("record.explain")),
		container.NewBorder(nil, nil, nil, mv.recButton, mv.recNote),
	)
	return container.NewBorder(top, mv.recResult, nil, nil, mv.recList)
}

// serverLine shows the IP, the protocol and the location of one recorded server.
func (a *App) serverLine(s monitor.Server) string {
	location := a.t("record.unknown")
	if l := a.file.Location(s.Location); l != nil {
		location = l.Name
	}
	return s.IP + "   " + s.Proto + "   " + location
}

func (a *App) refreshAdvanced() {
	mv := a.main
	if mv == nil || mv.recButton == nil {
		return
	}
	if a.rec.running {
		mv.recButton.SetText(a.t("record.stop"))
		mv.recNote.Disable()
	} else {
		mv.recButton.SetText(a.t("record.start"))
		mv.recNote.Enable()
	}
	setEnabled(mv.recButton, a.admin && !a.rec.stopping && a.setup.cancel == nil)
	mv.recList.Refresh()

	mv.recResult.RemoveAll()
	if a.rec.path != "" {
		dir := filepath.Dir(a.rec.path)
		open := widget.NewButton(a.t("record.openFolder"), func() { a.be.openFolder(dir) })
		mv.recResult.Add(smallText(a.t("record.saved", a.rec.path), fyne.TextWrapBreak))
		mv.recResult.Add(wrapLabel(a.t("record.send")))
		mv.recResult.Add(container.NewHBox(open))
	}
	mv.recResult.Refresh()
}

func (a *App) toggleRecording() {
	if a.rec.running {
		a.rec.stopping = true
		a.rec.cancel()
		a.refreshAdvanced()
		return
	}
	if !a.admin {
		return
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.rec = recorder{running: true, cancel: cancel, note: a.rec.note}
	a.setStatus(a.t("status.recording"))
	a.refreshAdvanced()

	note, known := a.rec.note, a.file
	dir := filepath.Join(a.dataDir, "recordings")
	a.run(func() {
		defer cancel()
		rec, err := a.be.record(ctx, note, known, func(s monitor.Server) {
			fyne.Do(func() {
				a.rec.servers = append(a.rec.servers, s)
				if a.main != nil {
					a.main.recList.Refresh()
				}
			})
		})
		path := ""
		if rec != nil {
			var saveErr error
			if path, saveErr = rec.Save(dir); saveErr != nil {
				path = ""
				if err == nil {
					err = saveErr
				}
			}
		}
		fyne.Do(func() {
			a.rec.running = false
			a.rec.stopping = false
			a.rec.cancel = nil
			a.rec.path = path
			if err != nil {
				a.showError(a.t("error.record"), err)
			}
			if path != "" {
				a.setStatus(a.t("status.recordSaved"))
			}
			a.refreshAdvanced()
		})
	})
}
