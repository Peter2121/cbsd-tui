package tui

import (
	//"bufio"
	//"bytes"
	//"fmt"
	//"os"
	//"os/exec"
	//"os/user"
	//"strings"
	//"sync"
	//"time"
	//"unicode/utf8"

	"github.com/gcla/gowid"
	//"github.com/gcla/gowid/vim"
	//"github.com/gcla/gowid/widgets/boxadapter"
	//"github.com/gcla/gowid/widgets/button"
	//"github.com/gcla/gowid/widgets/cellmod"
	"github.com/gcla/gowid/widgets/checkbox"
	"github.com/gcla/gowid/widgets/columns"
	"github.com/gcla/gowid/widgets/dialog"
	"github.com/gcla/gowid/widgets/divider"
	"github.com/gcla/gowid/widgets/edit"

	//"github.com/gcla/gowid/widgets/fill"
	"github.com/gcla/gowid/widgets/framed"
	//"github.com/gcla/gowid/widgets/grid"
	//"github.com/gcla/gowid/widgets/holder"
	"github.com/gcla/gowid/widgets/hpadding"
	//"github.com/gcla/gowid/widgets/keypress"
	//"github.com/gcla/gowid/widgets/list"
	//"github.com/gcla/gowid/widgets/menu"
	"github.com/gcla/gowid/widgets/pile"
	"github.com/gcla/gowid/widgets/styled"

	//"github.com/gcla/gowid/widgets/terminal"
	"github.com/gcla/gowid/widgets/text"
	//"github.com/gcla/gowid/widgets/vpadding"
	//tcell "github.com/gdamore/tcell/v2"
	//log "github.com/sirupsen/logrus"
)

var HALIGN_MIDDLE text.Options = text.Options{Align: gowid.HAlignMiddle{}}
var HALIGN_LEFT text.Options = text.Options{Align: gowid.HAlignLeft{}}

func MakeDialogForJail(jname string, title string, txt []string,
	boolparnames []string, boolpardefaults []bool,
	strparnames []string, strpardefaults []string,
	okfunc func(jname string, boolparams []bool, strparams []string)) *dialog.Widget {

	var lines *pile.Widget
	var containers []gowid.IContainerWidget

	var ntxt int = 0
	var nboolparams = 0
	var nstrparams = 0

	if txt != nil {
		ntxt = len(txt)
	}
	if boolparnames != nil {
		nboolparams = len(boolparnames)
	}
	if strparnames != nil {
		nstrparams = len(strparnames)
	}

	var widtxt []*text.Widget
	var widtxtst []*styled.Widget

	var widchecktxt []*text.Widget
	var widchecktxtst []*styled.Widget
	var widcheck []*checkbox.Widget
	var widcheckgrp []*hpadding.Widget

	var wideditparams []*edit.Widget
	var wideditstparams []*styled.Widget

	var strparams []string
	var boolparams []bool

	var btncancel dialog.Button
	var btnok dialog.Button
	var buttons []dialog.Button

	htxt := text.New(title, HALIGN_MIDDLE)
	htxtst := styled.New(htxt, gowid.MakePaletteRef("magenta"))
	containers = append(containers, &gowid.ContainerWidget{IWidget: htxtst, D: gowid.RenderFlow{}})
	containers = append(containers, &gowid.ContainerWidget{IWidget: divider.NewUnicode(), D: gowid.RenderFlow{}})

	for i := 0; i < ntxt; i++ {
		widtxt = append(widtxt, text.New(txt[i], HALIGN_LEFT))
		widtxtst = append(widtxtst, styled.New(widtxt[i], gowid.MakePaletteRef("green")))
		containers = append(containers, &gowid.ContainerWidget{IWidget: widtxtst[i], D: gowid.RenderFlow{}})
	}

	for i := 0; i < nboolparams; i++ {
		widchecktxt = append(widchecktxt, text.New(boolparnames[i], HALIGN_LEFT))
		widchecktxtst = append(widchecktxtst, styled.New(widchecktxt[i], gowid.MakePaletteRef("green")))
		widcheck = append(widcheck, checkbox.New(boolpardefaults[i]))
		widcheckgrp = append(widcheckgrp, hpadding.New(columns.NewFixed(widchecktxtst[i], widcheck[i]), gowid.HAlignLeft{}, gowid.RenderFixed{}))
		containers = append(containers, &gowid.ContainerWidget{IWidget: widcheckgrp[i], D: gowid.RenderFlow{}})
	}

	for i := 0; i < nstrparams; i++ {
		wideditparams = append(wideditparams, edit.New(edit.Options{Caption: strparnames[i], Text: strpardefaults[i]}))
		wideditstparams = append(wideditstparams, styled.New(wideditparams[i], gowid.MakePaletteRef("green")))
		containers = append(containers, &gowid.ContainerWidget{IWidget: wideditstparams[i], D: gowid.RenderFlow{}})
	}

	lines = pile.New(containers)

	btnok = dialog.Button{
		Msg: "OK",
		Action: gowid.MakeWidgetCallback("execclonejail", gowid.WidgetChangedFunction(func(app gowid.IApp, w gowid.IWidget) {
			for i := 0; i < nboolparams; i++ {
				boolparams = append(boolparams, widcheck[i].IsChecked())
			}
			for i := 0; i < nstrparams; i++ {
				strparams = append(strparams, wideditparams[i].Text())
			}
			okfunc(jname, boolparams, strparams)
		})),
	}

	if nboolparams < 1 && nstrparams < 1 && okfunc == nil {
		btncancel = dialog.Button{Msg: "Close"}
		buttons = append(buttons, btncancel)
	} else {
		btncancel = dialog.Button{Msg: "Cancel"}
		buttons = append(buttons, btnok)
		buttons = append(buttons, btncancel)
	}

	retdialog := dialog.New(
		framed.NewSpace(
			lines,
		),
		dialog.Options{
			Buttons:         buttons,
			NoShadow:        true,
			BackgroundStyle: gowid.MakePaletteRef("bluebg"),
			BorderStyle:     gowid.MakePaletteRef("dialog"),
			ButtonStyle:     gowid.MakePaletteRef("white-focus"),
			Modal:           true,
			FocusOnWidget:   true,
		},
	)
	return retdialog
}
