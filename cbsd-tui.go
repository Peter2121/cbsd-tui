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

var doas = true

var txtProgramName = "CBSD-TUI"
var txtHelp = `- To navigate in jails list use 'Up' and 'Down' keys or mouse
- To open 'Actions' menu for the selected jail use 'F2' key
- To login into the selected jail (as root) use 'Enter' key or mouse double-click on jail name
- To switch to terminal from jails list use 'Tab' key
- To switch to jails list from terminal use 'Ctrl-Z'+'Tab' keys sequence
- Use bottom menu ('Fx' keys or mouse clicks) to start actions on the selected jail`

var logFileName = "/var/log/cbsd-tui.log"

var cbsdListHeader []gowid.IWidget
var cbsdListLines [][]gowid.IWidget
var cbsdListGrid []gowid.IWidget
var cbsdListWalker *list.SimpleListWalker
var cbsdBottomMenu []gowid.IContainerWidget
var cbsdJailConsole *terminal.Widget
var cbsdWidgets *ResizeablePileWidget
var cbsdJailConsoleActive string
var WIDTH = 18
var HPAD = 2
var VPAD = 1

var Containers []Container

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
	HelpDialog.Open(viewHolder, gowid.RenderWithRatio{R: 0.3}, app)
}

func RunMenuAction(action string) {
	log.Infof("Menu Action: " + action)

	switch action {
	// "[F1]Help ",      "[F2]Actions Menu ",    "[F3]View ",     "[F4]Edit ",     "[F5]Clone ",
	// "[F6]Export ",    "[F7]Create Snapshot ", "[F8]Destroy ",  "[F10]Exit ",    "[F11]List Snapshots ", "[F12]Start/Stop"
	case (&(jail.Jail{})).GetCommandHelp(): // Help
		OpenHelpDialog()
		return
	case (&(jail.Jail{})).GetCommandExit(): // Exit
		app.Quit()
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
	//cbsdJailsFromDb, err = jail.GetJailsFromDb(host.GetCbsdDbConnString(false))
	Containers, err = GetContainersFromDb("jail", host.GetCbsdDbConnString(false))
	if err != nil {
		panic(err)
	}
	cbsdListLines = MakeJailsLines()
	cbsdListGrid = make([]gowid.IWidget, 0)
	gheader := grid.New(cbsdListHeader, WIDTH, HPAD, VPAD, gowid.HAlignMiddle{})
	cbsdListGrid = append(cbsdListGrid, gheader)
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

		line[0] = GetMenuButton(jail)
		jail_params := jail.GetAllParams()
		for i, param := range jail_params {
			line[i+1] = GetStyledWidget(text.New(param, HALIGN_MIDDLE), style)
		}
	}
}

func GetMenuButton(jail Container) *keypress.Widget {
	btxt := text.New(jail.GetName(), HALIGN_MIDDLE)
	style := GetJailStyle(jail.GetStatus(), jail.GetAstart())
	txts := GetStyledWidget(btxt, style)
	btnnew := button.New(txts, button.Options{
		Decoration: button.BareDecoration,
	})
	btnnew.OnDoubleClick(gowid.WidgetCallback{Name: "cbb_" + btxt.Content().String(), WidgetChangedFunction: func(app gowid.IApp, w gowid.IWidget) {
		app.Run(gowid.RunFunction(func(app gowid.IApp) {
			LoginToJail(btxt.Content().String())
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

func LoginToJail(jname string) {
	if jname == cbsdJailConsoleActive {
		SendTerminalCommand("\x03")
		SendTerminalCommand("exit")
		cbsdJailConsoleActive = ""
		return
	}
	jail := GetJailByName(jname)
	if jail != nil && jail.IsRunning() {
		if cbsdJailConsoleActive != "" {
			SendTerminalCommand("\x03")
			SendTerminalCommand("exit")
		}
		if host.USE_DOAS {
			SendTerminalCommand(host.DOAS_PROGRAM + " " + host.CBSD_PROGRAM + " " + jail.GetLoginCommand())
		} else {
			SendTerminalCommand(host.CBSD_PROGRAM + " " + jail.GetLoginCommand())
		}
		cbsdJailConsoleActive = jname
		if cbsdWidgets.Focus() == 0 { // TODO: check current focus more carefully
			if next, ok := cbsdWidgets.FindNextSelectable(gowid.Forwards, true); ok {
				cbsdWidgets.SetFocus(app, next)
			}
		}
	}
}

func SendTerminalCommand(cmd string) {
	cbsdJailConsole.Write([]byte(cmd + "\n"))
	time.Sleep(200 * time.Millisecond)
}

func GetJailsListHeader() []gowid.IWidget {
	header := make([]gowid.IWidget, 0)
	for _, h := range (&(jail.Jail{})).GetHeaderTitles() {
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
	newpos := list.ListPos(0)
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
		LoginToJail(jname)
	case tcell.KeyF2:
		curjail := GetJailByName(jname)
		curjail.OpenActionDialog()
	case tcell.KeyCtrlR:
		RefreshJailList()
	case tcell.KeyTab:
		if next, ok := cbsdWidgets.FindNextSelectable(gowid.Forwards, true); ok {
			cbsdWidgets.SetFocus(app, next)
		}
	}
}

func MakeGridLine(jail Container) []gowid.IWidget {
	style := "gray"
	line := make([]gowid.IWidget, 0)
	style = GetJailStyle(jail.GetStatus(), jail.GetAstart())
	line = append(line, GetMenuButton(jail))
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
			if next, ok := cbsdWidgets.FindNextSelectable(gowid.Forwards, true); ok {
				cbsdWidgets.SetFocus(app, next)
			}
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

func MakeBottomMenu() {
	cbsdBottomMenu = make([]gowid.IContainerWidget, 0)
	for i, m := range (&(jail.Jail{})).GetBottomMenuText2() {
		mtext1 := text.New((&(jail.Jail{})).GetBottomMenuText1()[i], HALIGN_LEFT)
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

func main() {
	var err error

	palette := gowid.Palette{
		"red-nofocus":   gowid.MakePaletteEntry(gowid.ColorPurple, gowid.ColorNone),
		"red-focus":     gowid.MakePaletteEntry(gowid.ColorBlack, gowid.ColorPurple),
		"green-nofocus": gowid.MakePaletteEntry(gowid.ColorGreen, gowid.ColorNone),
		"green-focus":   gowid.MakePaletteEntry(gowid.ColorBlack, gowid.ColorGreen),
		"white-nofocus": gowid.MakePaletteEntry(gowid.ColorWhite, gowid.ColorNone),
		"white-focus":   gowid.MakePaletteEntry(gowid.ColorBlack, gowid.ColorWhite),
		"gray-nofocus":  gowid.MakePaletteEntry(gowid.ColorLightGray, gowid.ColorNone),
		"gray-focus":    gowid.MakePaletteEntry(gowid.ColorBlack, gowid.ColorLightGray),
		"cyan-nofocus":  gowid.MakePaletteEntry(gowid.ColorCyan, gowid.ColorNone),
		"cyan-focus":    gowid.MakePaletteEntry(gowid.ColorBlack, gowid.ColorCyan),
		"red":           gowid.MakePaletteEntry(gowid.ColorRed, gowid.ColorNone),
		"redgray":       gowid.MakePaletteEntry(gowid.ColorRed, gowid.ColorLightGray),
		"blackgreen":    gowid.MakePaletteEntry(gowid.ColorBlack, gowid.ColorGreen),
		"graydgreen":    gowid.MakePaletteEntry(gowid.ColorLightGray, gowid.ColorDarkGreen),
		"bluebg":        gowid.MakePaletteEntry(gowid.ColorWhite, gowid.ColorCyan),
		"invred":        gowid.MakePaletteEntry(gowid.ColorBlack, gowid.ColorRed),
		"streak":        gowid.MakePaletteEntry(gowid.ColorBlack, gowid.ColorRed),
		"test1focus":    gowid.MakePaletteEntry(gowid.ColorBlue, gowid.ColorBlack),
		"test1notfocus": gowid.MakePaletteEntry(gowid.ColorGreen, gowid.ColorBlack),
		"test2focus":    gowid.MakePaletteEntry(gowid.ColorMagenta, gowid.ColorBlack),
		"test2notfocus": gowid.MakePaletteEntry(gowid.ColorCyan, gowid.ColorBlack),
		"yellow":        gowid.MakePaletteEntry(gowid.ColorYellow, gowid.ColorNone),
		"magenta":       gowid.MakePaletteEntry(gowid.ColorMagenta, gowid.ColorNone),
	}

	Containers, err = GetContainersFromDb("jail", host.GetCbsdDbConnString(false))
	if err != nil {
		panic(err)
	}

	/*
		cbsdJailsFromDb, err = jail.GetJailsFromDb(host.GetCbsdDbConnString(false))
		if err != nil {
			panic(err)
		}
	*/

	if len(Containers) < 1 {
		log.Errorf("Cannot find containers in database %s", host.CBSD_DB_NAME)
		return
	}

	f := RedirectLogger(logFileName)
	defer f.Close()

	doas, err = host.NeedDoAs()
	if err != nil {
		log.Errorf("Error from host.NeedDoAs(): %v", err)
	}

	cbsdListLines = MakeJailsLines()
	cbsdListHeader = GetJailsListHeader()

	cbsdListGrid = make([]gowid.IWidget, 0)
	gheader := grid.New(cbsdListHeader, WIDTH, HPAD, VPAD, gowid.HAlignMiddle{})
	cbsdListGrid = append(cbsdListGrid, gheader)
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

	MakeBottomMenu()
	gbmenu := columns.New(cbsdBottomMenu, columns.Options{DoNotSetSelected: true, LeftKeys: make([]vim.KeyPress, 0), RightKeys: make([]vim.KeyPress, 0)})

	toppanel := NewResizeablePile([]gowid.IContainerWidget{
		&gowid.ContainerWidget{IWidget: listjails, D: gowid.RenderWithWeight{W: 1}},
		&gowid.ContainerWidget{IWidget: gbmenu, D: gowid.RenderWithUnits{U: 1}},
	})
	hline := styled.New(fill.New('⎯'), gowid.MakePaletteRef("line"))

	cbsdWidgets = NewResizeablePile([]gowid.IContainerWidget{
		&gowid.ContainerWidget{IWidget: toppanel, D: gowid.RenderWithWeight{W: 1}},
		&gowid.ContainerWidget{IWidget: hline, D: gowid.RenderWithUnits{U: 1}},
		&gowid.ContainerWidget{IWidget: cbsdJailConsole, D: gowid.RenderWithWeight{W: 1}},
	})
	viewHolder = holder.New(cbsdWidgets)

	app, err = gowid.NewApp(gowid.AppArgs{
		View:    viewHolder,
		Palette: &palette,
		Log:     log.StandardLogger(),
	})

	main_tui := tui.NewTui(app, viewHolder, cbsdJailConsole)
	for i := range Containers {
		Containers[i].SetTui(main_tui)
	}

	for i := range Containers {
		Containers[i].GetSignalRefresh().Connect(nil, func(a any) { RefreshJailList() })
		Containers[i].GetSignalUpdated().Connect(nil, func(jname string) { UpdateJailLine(GetJailByName(jname)) })
	}

	ExitOnErr(err)
	SetJailListFocus()
	app.MainLoop(handler{})
}
