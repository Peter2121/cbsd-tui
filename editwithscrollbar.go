// Copyright 2019-2022 Graham Clark. All rights reserved.  Use of this source code is governed by the MIT license
// that can be found in the LICENSE file of https://github.com/gcla/gowid repository.

package main

import (
	"github.com/gcla/gowid"
	"github.com/gcla/gowid/gwutil"
	"github.com/gcla/gowid/widgets/columns"
	"github.com/gcla/gowid/widgets/edit"
	"github.com/gcla/gowid/widgets/text"
	"github.com/gcla/gowid/widgets/vscroll"
	tcell "github.com/gdamore/tcell/v2"
)

type EditWithScrollbar struct {
	*columns.Widget
	e        *edit.Widget
	sb       *vscroll.Widget
	goUpDown int // positive means down
	pgUpDown int // positive means down
}

func NewEditWithScrollbar(e *edit.Widget) *EditWithScrollbar {
	sb := vscroll.NewExt(vscroll.VerticalScrollbarUnicodeRunes)
	res := &EditWithScrollbar{
		columns.New([]gowid.IContainerWidget{
			&gowid.ContainerWidget{IWidget: e, D: gowid.RenderWithWeight{W: 1}},
			&gowid.ContainerWidget{IWidget: sb, D: gowid.RenderWithUnits{U: 1}},
		}),
		e, sb, 0, 0,
	}
	sb.OnClickAbove(gowid.WidgetCallback{Name: "cb", WidgetChangedFunction: res.clickUp})
	sb.OnClickBelow(gowid.WidgetCallback{Name: "cb", WidgetChangedFunction: res.clickDown})
	sb.OnClickUpArrow(gowid.WidgetCallback{Name: "cb", WidgetChangedFunction: res.clickUpArrow})
	sb.OnClickDownArrow(gowid.WidgetCallback{Name: "cb", WidgetChangedFunction: res.clickDownArrow})
	return res
}

func (e *EditWithScrollbar) clickUp(app gowid.IApp, w gowid.IWidget) {
	e.pgUpDown -= 1
}

func (e *EditWithScrollbar) clickDown(app gowid.IApp, w gowid.IWidget) {
	e.pgUpDown += 1
}

func (e *EditWithScrollbar) clickUpArrow(app gowid.IApp, w gowid.IWidget) {
	e.goUpDown -= 1
}

func (e *EditWithScrollbar) clickDownArrow(app gowid.IApp, w gowid.IWidget) {
	e.goUpDown += 1
}

// gcdoc - do this so columns navigation e.g. ctrl-f doesn't get passed to columns
func (w *EditWithScrollbar) UserInput(ev interface{}, size gowid.IRenderSize, focus gowid.Selector, app gowid.IApp) bool {
	// Stop these keys moving focus in the columns used by this widget. C-f is used to
	// open a file.
	if evk, ok := ev.(*tcell.EventKey); ok {
		switch evk.Key() {
		case tcell.KeyCtrlF, tcell.KeyCtrlB:
			return false
		}
	}

	box, _ := size.(gowid.IRenderBox)
	w.sb.Top, w.sb.Middle, w.sb.Bottom = w.e.CalculateTopMiddleBottom(gowid.MakeRenderBox(box.BoxColumns()-1, box.BoxRows()))

	res := w.Widget.UserInput(ev, size, focus, app)
	if res {
		w.Widget.SetFocus(app, 0)
	}
	return res
}

func (w *EditWithScrollbar) Render(size gowid.IRenderSize, focus gowid.Selector, app gowid.IApp) gowid.ICanvas {
	box, err := size.(gowid.IRenderBox)
	if !err {
		return nil
	}
	ecols := box.BoxColumns() - 1
	ebox := gowid.MakeRenderBox(ecols, box.BoxRows())
	if w.goUpDown != 0 || w.pgUpDown != 0 {
		w.e.SetLinesFromTop(gwutil.Max(0, w.e.LinesFromTop()+w.goUpDown+(w.pgUpDown*box.BoxRows())), app)
		txt := w.e.MakeText()
		layout := text.MakeTextLayout(txt.Content(), ecols, txt.Wrap(), gowid.HAlignLeft{})
		_, y := text.GetCoordsFromCursorPos(w.e.CursorPos(), ecols, layout, w.e)
		if y < w.e.LinesFromTop() {
			for i := y; i < w.e.LinesFromTop(); i++ {
				w.e.DownLines(ebox, false, app)
			}
		} else if y >= w.e.LinesFromTop()+box.BoxRows() {
			for i := w.e.LinesFromTop() + box.BoxRows(); i <= y; i++ {
				w.e.UpLines(ebox, false, app)
			}
		}

	}
	w.goUpDown = 0
	w.pgUpDown = 0
	w.sb.Top, w.sb.Middle, w.sb.Bottom = w.e.CalculateTopMiddleBottom(ebox)

	canvas := w.Widget.Render(size, focus, app)

	return canvas
}
