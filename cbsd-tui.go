package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gcla/gowid"
	"github.com/gcla/gowid/vim"
	"github.com/gcla/gowid/widgets/button"
	"github.com/gcla/gowid/widgets/cellmod"
	"github.com/gcla/gowid/widgets/columns"
	"github.com/gcla/gowid/widgets/dialog"
	"github.com/gcla/gowid/widgets/fill"
	"github.com/gcla/gowid/widgets/grid"
	"github.com/gcla/gowid/widgets/holder"
	"github.com/gcla/gowid/widgets/hpadding"
	"github.com/gcla/gowid/widgets/keypress"
	"github.com/gcla/gowid/widgets/list"
	"github.com/gcla/gowid/widgets/menu"
	"github.com/gcla/gowid/widgets/pile"
	"github.com/gcla/gowid/widgets/styled"
	"github.com/gcla/gowid/widgets/terminal"
	"github.com/gcla/gowid/widgets/text"
	"github.com/gcla/gowid/widgets/vpadding"

	"bhyve"
	"host"
	"jail"
	"tui"

	tcell "github.com/gdamore/tcell/v2"
	log "github.com/sirupsen/logrus"
)

type PairString struct {
	Key   string
	Value string
}

var doas bool = true

const CTYPE_JAIL string = "jail"
const CTYPE_BHYVEVM string = "bhyvevm"

var ctype string = CTYPE_JAIL

//var ctype string = "bhyvevm"

var txtProgramName = "CBSD-TUI"
var txtHelp = `- To navigate in jails/VMs list use 'Up' and 'Down' keys or mouse
- To open 'Actions' menu for the selected jail/VM use 'F2' key
- To switch to jails management use 'Ctrl-J'
- To switch to Bhyve VMs management use 'Ctrl-B'
- To login into the selected jail/VM use 'Enter' key or mouse double-click on jail/VM name
- To switch to terminal from jails/VMs list use 'Tab' key
- To switch to jails/VMs list from terminal use 'Ctrl-Z'+'Tab' keys sequence
- Use bottom menu ('Fx' keys or mouse clicks) to start actions on the selected jail/VM`

var logFileName = "/var/log/cbsd-tui.log"

var cbsdListLines [][]gowid.IWidget
var cbsdListGrid []gowid.IWidget
var cbsdListWalker *list.SimpleListWalker

// var cbsdBottomMenu []gowid.IContainerWidget
var cbsdJailConsole *terminal.Widget
var cbsdWidgets *ResizeablePileWidget
var cbsdJailConsoleActive string
var WIDTH = 18
var HPAD = 2
var VPAD = 1

var Containers []Container
var mainTui *tui.Tui
var gHeader *grid.Widget
var gBmenu *columns.Widget

var lastFocusPosition int

//var topPanel *ResizeablePileWidget

// Temporary declaration - will be replaced with interface
//var cbsdJailsFromDb []*jail.Jail
//var cbsdVMsFromDb []*bhyve.BhyveVm

var shellProgram = "/bin/sh"
var stdbufProgram = "/usr/bin/stdbuf"
var logJstart = "/var/log/jstart.log"
var logText string = ""

var cbsdListJails *list.Widget

var app *gowid.App
var menu2 *menu.Widget
var viewHolder *holder.Widget

type handler struct{}

var HALIGN_MIDDLE text.Options = text.Options{Align: gowid.HAlignMiddle{}}
var HALIGN_LEFT text.Options = text.Options{Align: gowid.HAlignLeft{}}

func OpenHelpDialog() {
	var HelpDialog *dialog.Widget
	HelpDialog = tui.MakeDialogForJail(
		"",
		txtProgramName,
		[]string{txtHelp},
		nil, nil, nil, nil,
		nil,
	)
	HelpDialog.Open(viewHolder, gowid.RenderWithRatio{R: 0.5}, app)
}

func RunMenuAction(action string) {
	log.Infof("Menu Action: " + action)

	if len(Containers) > 0 {
		switch action {
		// "[F1]Help ",      "[F2]Actions Menu ",    "[F3]View ",     "[F4]Edit ",     "[F5]Clone ",
		// "[F6]Export ",    "[F7]Create Snapshot ", "[F8]Destroy ",  "[F10]Exit ",    "[F11]List Snapshots ", "[F12]Start/Stop"
		case Containers[0].GetCommandHelp(): // Help
			OpenHelpDialog()
			return
		case Containers[0].GetCommandExit(): // Exit
			app.Quit()
		}
	}

	curjail := GetSelectedJail()
	if curjail == nil {
		return
	}
	log.Infof("JailName: " + curjail.GetName())
	curjail.ExecuteActionOnCommand(action)
}

func GetSelectedJail() Container {
	curpos := GetSelectedPosition()
	if curpos < 0 {
		return nil
	}
	if len(Containers) < curpos {
		return nil
	}
	return Containers[curpos]
}

func GetSelectedPosition() int {
	ifocus := cbsdListJails.Walker().Focus()
	return int(ifocus.(list.ListPos)) - 1
}

func RefreshJailList() {
	var err error
	Containers, err = GetContainersFromDb(ctype, host.GetCbsdDbConnString(false))
	if err != nil {
		panic(err)
	}
	cbsdListLines = MakeJailsLines()
	cbsdListGrid = make([]gowid.IWidget, 0)
	gHeader = grid.New(GetJailsListHeader(), WIDTH, HPAD, VPAD, gowid.HAlignMiddle{})
	cbsdListGrid = append(cbsdListGrid, gHeader)
	for _, line := range cbsdListLines {
		gline := grid.New(line, WIDTH, HPAD, VPAD, gowid.HAlignMiddle{},
			grid.Options{
				DownKeys: []vim.KeyPress{},
				UpKeys:   []vim.KeyPress{},
			})
		cbsdListGrid = append(cbsdListGrid, gline)
	}
	cbsdListWalker = list.NewSimpleListWalker(cbsdListGrid)
	cbsdListJails.SetWalker(cbsdListWalker, app)
	for i := range Containers {
		Containers[i].SetTui(mainTui)
	}
	for i := range Containers {
		Containers[i].GetSignalRefresh().Connect(nil, func(a any) { RefreshJailList() })
		Containers[i].GetSignalUpdated().Connect(nil, func(jname string) { UpdateJailLine(GetJailByName(jname)) })
	}
	/*
		// TODO: correctly rewrite bottom menu
		gBmenu = columns.New(MakeBottomMenu(), columns.Options{DoNotSetSelected: true, LeftKeys: make([]vim.KeyPress, 0), RightKeys: make([]vim.KeyPress, 0)})
		listjails := vpadding.New(cbsdListJails, gowid.VAlignTop{}, gowid.RenderFlow{})
		top_panel := NewResizeablePile([]gowid.IContainerWidget{
			&gowid.ContainerWidget{IWidget: listjails, D: gowid.RenderWithWeight{W: 1}},
			&gowid.ContainerWidget{IWidget: gBmenu, D: gowid.RenderWithUnits{U: 1}},
		})
		cbsdWidgets.SubWidgets()[0] = &gowid.ContainerWidget{IWidget: top_panel, D: gowid.RenderWithWeight{W: 1}}
	*/
	SetJailListFocus()
}

func UpdateJailLine(jail Container) {
	for _, line := range cbsdListLines {
		btn := line[0].(*keypress.Widget).SubWidget().(*cellmod.Widget).SubWidget().(*button.Widget)
		txt := btn.SubWidget().(*styled.Widget).SubWidget().(*text.Widget)
		str := txt.Content().String()
		if str != jail.GetName() {
			continue
		}
		style := GetJailStyle(jail.GetStatus(), jail.GetAstart())
		//	var cbsdJlsHeader = []string{"NAME", "IP4_ADDRESS", "STATUS", "AUTOSTART", "VERSION"}

		line[0] = GetMenuButton(jail, "")
		jail_params := jail.GetAllParams()
		for i, param := range jail_params {
			line[i+1] = GetStyledWidget(text.New(param, HALIGN_MIDDLE), style)
		}
	}
}

func ChangeJailBtnColor(color string, position int) {
	line := cbsdListLines[position]
	jail := Containers[position]
	line[0] = GetMenuButton(jail, color)
}

func GetMenuButton(jail Container, style string) *keypress.Widget {
	btxt := text.New(jail.GetName(), HALIGN_MIDDLE)
	if len(style) == 0 {
		style = GetJailStyle(jail.GetStatus(), jail.GetAstart())
	}
	txts := GetStyledWidget(btxt, style)
	btnnew := button.New(txts, button.Options{
		Decoration: button.BareDecoration,
	})
	btnnew.OnDoubleClick(gowid.WidgetCallback{Name: "cbb_" + btxt.Content().String(), WidgetChangedFunction: func(app gowid.IApp, w gowid.IWidget) {
		app.Run(gowid.RunFunction(func(app gowid.IApp) {
			LoginToJail(btxt.Content().String(), mainTui)
		}))
	}})
	kpbtn := keypress.New(
		cellmod.Opaque(btnnew),
		keypress.Options{
			Keys: []gowid.IKey{
				gowid.MakeKeyExt(tcell.KeyEnter),
				gowid.MakeKeyExt(tcell.KeyF2),
				gowid.MakeKeyExt(tcell.KeyTab),
				gowid.MakeKeyExt(tcell.KeyCtrlR),
				gowid.MakeKeyExt(tcell.KeyCtrlJ),
				gowid.MakeKeyExt(tcell.KeyCtrlB),
			},
		},
	)
	kpbtn.OnKeyPress(keypress.MakeCallback("kpbtn_"+btxt.Content().String(), func(app gowid.IApp, w gowid.IWidget, k gowid.IKey) {
		JailListButtonCallBack(btxt.Content().String(), k)
	}))
	return kpbtn
}

func GetJailByName(jname string) Container {
	var jail Container = nil
	for i, j := range Containers {
		if j.GetName() == jname {
			jail = Containers[i]
			break
		}
	}
	return jail
}

func LoginToJail(jname string, t *tui.Tui) {
	if jname == cbsdJailConsoleActive {
		t.SendTerminalCommand("\x03")
		t.SendTerminalCommand("exit")
		cbsdJailConsoleActive = ""
		t.ResetTerminal()
		RestoreFocus()
		return
	}
	jail := GetJailByName(jname)
	if jail != nil && jail.IsRunning() {
		if cbsdJailConsoleActive != "" {
			t.SendTerminalCommand("\x03")
			t.SendTerminalCommand("exit")
			t.ResetTerminal()
			RestoreFocus()
		}
		if host.USE_DOAS {
			t.SendTerminalCommand(host.DOAS_PROGRAM + " " + host.CBSD_PROGRAM + " " + jail.GetLoginCommand())
		} else {
			t.SendTerminalCommand(host.CBSD_PROGRAM + " " + jail.GetLoginCommand())
		}
		cbsdJailConsoleActive = jname
		ReleaseFocus()
		if cbsdWidgets.Focus() == 0 { // TODO: check current focus more carefully
			if next, ok := cbsdWidgets.FindNextSelectable(gowid.Forwards, true); ok {
				cbsdWidgets.SetFocus(app, next)
			}
		}
	}
}

/*
func SendTerminalCommand(cmd string) {
	cbsdJailConsole.Write([]byte(cmd + "\n"))
	time.Sleep(100 * time.Millisecond)
}
*/

func GetJailsListHeader() []gowid.IWidget {
	header := make([]gowid.IWidget, 0)
	titles := make([]string, 0)
	if len(Containers) > 0 {
		titles = Containers[0].GetHeaderTitles()
	}
	for _, h := range titles {
		htext := text.New(h, HALIGN_MIDDLE)
		header = append(header, GetStyledWidget(htext, "white"))
	}
	return header
}

func GetJailStyle(jailstatus int, jailastart int) string {
	style := "gray"
	if jailstatus == 1 {
		style = "green"
	} else if jailstatus == 0 {
		switch jailastart {
		case 1:
			style = "red"
		default:
			style = "white"
		}
	}
	return style
}

func SetJailListFocus() {
	var newpos list.ListPos
	if len(Containers) > 0 {
		newpos = list.ListPos(1)
	} else {
		newpos = list.ListPos(0)
	}
	for i, jail := range Containers {
		if jail.IsRunning() {
			newpos = list.ListPos(i + 1)
			break
		}
	}
	cbsdListJails.Walker().SetFocus(newpos, app)
}

func JailListButtonCallBack(jname string, key gowid.IKey) {
	switch key.Key() {
	case tcell.KeyEnter:
		LoginToJail(jname, mainTui)
	case tcell.KeyF2:
		curjail := GetJailByName(jname)
		curjail.OpenActionDialog()
	case tcell.KeyCtrlR:
		RefreshJailList()
	case tcell.KeyCtrlJ:
		if ctype != CTYPE_JAIL {
			ctype = CTYPE_JAIL
			RefreshJailList()
		}
	case tcell.KeyCtrlB:
		if ctype != CTYPE_BHYVEVM {
			ctype = CTYPE_BHYVEVM
			RefreshJailList()
		}
	case tcell.KeyTab:
		// Tab from jails list
		if next, ok := cbsdWidgets.FindNextSelectable(gowid.Forwards, true); ok {
			cbsdWidgets.SetFocus(app, next)
		}
		ReleaseFocus()
	}
}

func MakeGridLine(jail Container) []gowid.IWidget {
	style := "gray"
	line := make([]gowid.IWidget, 0)
	style = GetJailStyle(jail.GetStatus(), jail.GetAstart())
	line = append(line, GetMenuButton(jail, ""))
	jail_params := jail.GetAllParams()
	for _, param := range jail_params {
		line = append(line, GetStyledWidget(text.New(param, HALIGN_MIDDLE), style))
	}
	return line
}

func MakeJailsLines() [][]gowid.IWidget {
	lines := make([][]gowid.IWidget, 0)
	for i := range Containers {
		line := MakeGridLine(Containers[i])
		lines = append(lines, line)
	}
	return lines
}

func RedirectLogger(path string) *os.File {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Error opening log file: %v", err)
	}
	log.SetOutput(f)
	return f
}

func ExitOnErr(err error) {
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func (h handler) UnhandledInput(app gowid.IApp, ev interface{}) bool {
	log.Infof("Handler " + fmt.Sprintf("%T", ev))
	handled := false
	evk, ok := ev.(*tcell.EventKey)
	if ok {
		handled = true
		//log.Infof(string(evk.Key()))
		/*
			if evk.Key() == tcell.KeyCtrlC || evk.Key() == tcell.KeyEsc || evk.Key() == tcell.KeyF10 {
				app.Quit()
			} else if evk.Key() == tcell.KeyTab {
				if next, ok := cbsdWidgets.FindNextSelectable(gowid.Forwards, true); ok {
					cbsdWidgets.SetFocus(app, next)
				}
			} else {
				handled = false
			}
		*/
		// "[F1]Help ",            "[F2]Actions Menu ", "[F4]Edit ",   "[F5]Clone ",           "[F6]Export ",
		// "[F7]Create Snapshot ", "[F8]Destroy ",      "[F10]Exit ",  "[F11]List Snapshots ", "[F12]Start/Stop"
		ekey := evk.Key()
		switch ekey {
		case tcell.KeyCtrlC, tcell.KeyEsc, tcell.KeyF10:
			app.Quit()
		case tcell.KeyTab:
			// CtrlZ-Tab from terminal
			if next, ok := cbsdWidgets.FindNextSelectable(gowid.Forwards, true); ok {
				cbsdWidgets.SetFocus(app, next)
			}
			RestoreFocus()
			return handled
		case tcell.KeyF1:
			OpenHelpDialog()
			return handled
		}
		curjail := GetSelectedJail()
		if curjail == nil {
			return handled
		}
		curjail.ExecuteActionOnKey(int16(ekey))
	}
	return handled
}

type ResizeablePileWidget struct {
	*pile.Widget
	offset int
}

func NewResizeablePile(widgets []gowid.IContainerWidget) *ResizeablePileWidget {
	res := &ResizeablePileWidget{}
	res.Widget = pile.New(widgets)
	return res
}

func GetStyledWidget(w gowid.IWidget, color string) *styled.Widget {
	cfocus := color + "-focus"
	cnofocus := color + "-nofocus"
	return styled.NewWithRanges(w,
		[]styled.AttributeRange{{Start: 0, End: -1, Styler: gowid.MakePaletteRef(cnofocus)}},
		[]styled.AttributeRange{{Start: 0, End: -1, Styler: gowid.MakePaletteRef(cfocus)}},
	)
}

func MakeBottomMenu() []gowid.IContainerWidget {
	cbsdBottomMenu := make([]gowid.IContainerWidget, 0)
	menu_text := make([]string, 0)
	menu_text2 := make([]string, 0)
	if len(Containers) > 0 {
		menu_text = Containers[0].GetBottomMenuText1()
		menu_text2 = Containers[0].GetBottomMenuText2()
	}
	for i, m := range menu_text2 {
		mtext1 := text.New(menu_text[i], HALIGN_LEFT)
		mtext1st := styled.New(mtext1, gowid.MakePaletteRef("blackgreen"))
		mtext2 := text.New(m+" ", HALIGN_LEFT)
		mtext2st := styled.New(mtext2, gowid.MakePaletteRef("graydgreen"))
		mtextgrp := hpadding.New(
			columns.NewFixed(mtext1st, mtext2st),
			gowid.HAlignLeft{},
			gowid.RenderFixed{},
		)
		mbtn := button.New(mtextgrp, button.Options{Decoration: button.BareDecoration})
		mbtn.OnClick(gowid.WidgetCallback{Name: "cbb_" + mtext2.Content().String(), WidgetChangedFunction: func(app gowid.IApp, w gowid.IWidget) {
			app.Run(gowid.RunFunction(func(app gowid.IApp) {
				RunMenuAction(strings.TrimSpace(mtext2.Content().String()))
			}))
		}})
		cbsdBottomMenu = append(cbsdBottomMenu, &gowid.ContainerWidget{IWidget: mbtn, D: gowid.RenderFixed{}})
	}
	return cbsdBottomMenu
}

func GetContainersFromDb(c_type string, db string) ([]Container, error) {
	switch c_type {
	case "jail":
		jails, err := jail.GetJailsFromDb(db)
		if err != nil {
			return make([]Container, 0), err
		}
		jlen := len(jails)
		cont := make([]Container, jlen)
		for i := range jails {
			cont[i] = jails[i]
		}
		return cont, nil
	case "bhyvevm":
		vms, err := bhyve.GetBhyveVmsFromDb(db)
		if err != nil {
			return make([]Container, 0), err
		}
		jlen := len(vms)
		cont := make([]Container, jlen)
		for i := range vms {
			cont[i] = vms[i]
		}
		return cont, nil
	}
	return make([]Container, 0), nil
}

func ReleaseFocus() {
	lastFocusPosition = GetSelectedPosition()
	ChangeJailBtnColor("inactive", lastFocusPosition)
}

func RestoreFocus() {
	ChangeJailBtnColor("", GetSelectedPosition())
	if (lastFocusPosition >= 0) && (lastFocusPosition < len(cbsdListLines)) {
		ChangeJailBtnColor("", lastFocusPosition)
	}
}

func main() {
	var err error
	g23color, _ := gowid.MakeColorSafe("g23")
	//g7color, _ := gowid.MakeColorSafe("g7")
	palette := gowid.Palette{
		"red-nofocus":      gowid.MakePaletteEntry(gowid.ColorPurple, gowid.ColorNone),
		"red-focus":        gowid.MakePaletteEntry(gowid.ColorBlack, gowid.ColorPurple),
		"green-nofocus":    gowid.MakePaletteEntry(gowid.ColorGreen, gowid.ColorNone),
		"green-focus":      gowid.MakePaletteEntry(gowid.ColorBlack, gowid.ColorGreen),
		"white-nofocus":    gowid.MakePaletteEntry(gowid.ColorWhite, gowid.ColorNone),
		"white-focus":      gowid.MakePaletteEntry(gowid.ColorBlack, gowid.ColorWhite),
		"gray-nofocus":     gowid.MakePaletteEntry(gowid.ColorLightGray, gowid.ColorNone),
		"gray-focus":       gowid.MakePaletteEntry(gowid.ColorBlack, gowid.ColorLightGray),
		"cyan-nofocus":     gowid.MakePaletteEntry(gowid.ColorCyan, gowid.ColorNone),
		"cyan-focus":       gowid.MakePaletteEntry(gowid.ColorBlack, gowid.ColorCyan),
		"red":              gowid.MakePaletteEntry(gowid.ColorRed, gowid.ColorNone),
		"redgray":          gowid.MakePaletteEntry(gowid.ColorRed, gowid.ColorLightGray),
		"blackgreen":       gowid.MakePaletteEntry(gowid.ColorBlack, gowid.ColorGreen),
		"graydgreen":       gowid.MakePaletteEntry(gowid.ColorLightGray, gowid.ColorDarkGreen),
		"bluebg":           gowid.MakePaletteEntry(gowid.ColorWhite, gowid.ColorCyan),
		"invred":           gowid.MakePaletteEntry(gowid.ColorBlack, gowid.ColorRed),
		"streak":           gowid.MakePaletteEntry(gowid.ColorBlack, gowid.ColorRed),
		"darkblue-focus":   gowid.MakePaletteEntry(gowid.ColorWhite, gowid.ColorDarkBlue),
		"darkblue-nofocus": gowid.MakePaletteEntry(gowid.ColorWhite, gowid.ColorDarkBlue),
		"inactive-focus":   gowid.MakePaletteEntry(gowid.ColorWhite, g23color),
		"inactive-nofocus": gowid.MakePaletteEntry(gowid.ColorWhite, g23color),
		"test2focus":       gowid.MakePaletteEntry(gowid.ColorMagenta, gowid.ColorBlack),
		"test2notfocus":    gowid.MakePaletteEntry(gowid.ColorCyan, gowid.ColorBlack),
		"yellow-focus":     gowid.MakePaletteEntry(gowid.ColorYellow, gowid.ColorYellow),
		"yellow-nofocus":   gowid.MakePaletteEntry(gowid.ColorYellow, gowid.ColorYellow),
		"magenta":          gowid.MakePaletteEntry(gowid.ColorMagenta, gowid.ColorNone),
	}

	f := RedirectLogger(logFileName)
	defer f.Close()

	doas, err = host.NeedDoAs()
	if err != nil {
		log.Errorf("Error from host.NeedDoAs(): %v", err)
	}

	Containers, err = GetContainersFromDb(ctype, host.GetCbsdDbConnString(false))
	if err != nil {
		panic(err)
	}

	if len(Containers) < 1 {
		log.Errorf("Cannot find containers in database %s", host.CBSD_DB_NAME)
		return
	}

	cbsdListLines = MakeJailsLines()

	cbsdListGrid = make([]gowid.IWidget, 0)
	gHeader = grid.New(GetJailsListHeader(), WIDTH, HPAD, VPAD, gowid.HAlignMiddle{})
	cbsdListGrid = append(cbsdListGrid, gHeader)
	for _, line := range cbsdListLines {
		gline := grid.New(line, WIDTH, HPAD, VPAD, gowid.HAlignMiddle{},
			grid.Options{
				DownKeys: []vim.KeyPress{},
				UpKeys:   []vim.KeyPress{},
			})
		cbsdListGrid = append(cbsdListGrid, gline)
	}

	cbsdJailConsole, err = terminal.NewExt(terminal.Options{
		Command:           strings.Split(os.Getenv("SHELL"), " "),
		HotKey:            terminal.HotKey{K: tcell.KeyCtrlZ},
		HotKeyPersistence: &terminal.HotKeyDuration{D: time.Second * 2},
		Scrollbar:         true,
		Scrollback:        1000,
	})
	if err != nil {
		panic(err)
	}

	cbsdListWalker = list.NewSimpleListWalker(cbsdListGrid)
	cbsdListJails = list.New(cbsdListWalker)
	listjails := vpadding.New(cbsdListJails, gowid.VAlignTop{}, gowid.RenderFlow{})

	gBmenu = columns.New(MakeBottomMenu(), columns.Options{DoNotSetSelected: true, LeftKeys: make([]vim.KeyPress, 0), RightKeys: make([]vim.KeyPress, 0)})

	top_panel := NewResizeablePile([]gowid.IContainerWidget{
		&gowid.ContainerWidget{IWidget: listjails, D: gowid.RenderWithWeight{W: 1}},
		&gowid.ContainerWidget{IWidget: gBmenu, D: gowid.RenderWithUnits{U: 1}},
	})
	top_panel.OnFocusChanged(
		gowid.WidgetCallback{
			Name: "onfocuscbtp",
			WidgetChangedFunction: func(app gowid.IApp, w gowid.IWidget) {
				pw := w.(*pile.Widget)
				focus := pw.Focus()
				if focus == 1 {
					ReleaseFocus()
				} else {
					RestoreFocus()
				}
			},
		},
	)
	hline := styled.New(fill.New('⎯'), gowid.MakePaletteRef("line"))

	cbsdWidgets = NewResizeablePile([]gowid.IContainerWidget{
		&gowid.ContainerWidget{IWidget: top_panel, D: gowid.RenderWithWeight{W: 1}},
		&gowid.ContainerWidget{IWidget: hline, D: gowid.RenderWithUnits{U: 1}},
		&gowid.ContainerWidget{IWidget: cbsdJailConsole, D: gowid.RenderWithWeight{W: 1}},
	})
	viewHolder = holder.New(cbsdWidgets)

	app, err = gowid.NewApp(gowid.AppArgs{
		View:    viewHolder,
		Palette: &palette,
		Log:     log.StandardLogger(),
	})

	mainTui = tui.NewTui(app, viewHolder, cbsdJailConsole, cbsdWidgets.Widget)
	for i := range Containers {
		Containers[i].SetTui(mainTui)
	}
	for i := range Containers {
		Containers[i].GetSignalRefresh().Connect(nil, func(a any) { RefreshJailList() })
		Containers[i].GetSignalUpdated().Connect(nil, func(jname string) { UpdateJailLine(GetJailByName(jname)) })
	}

	ExitOnErr(err)
	SetJailListFocus()
	app.MainLoop(handler{})
}
