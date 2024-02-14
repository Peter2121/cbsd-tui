package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gcla/gowid"
	"github.com/gcla/gowid/vim"
	"github.com/gcla/gowid/widgets/boxadapter"
	"github.com/gcla/gowid/widgets/button"
	"github.com/gcla/gowid/widgets/cellmod"
	"github.com/gcla/gowid/widgets/checkbox"
	"github.com/gcla/gowid/widgets/columns"
	"github.com/gcla/gowid/widgets/dialog"
	"github.com/gcla/gowid/widgets/divider"
	"github.com/gcla/gowid/widgets/edit"
	"github.com/gcla/gowid/widgets/fill"
	"github.com/gcla/gowid/widgets/framed"
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

var doasProgram = "/usr/local/bin/doas"

var cbsdProgram = "/usr/local/bin/cbsd"
var cbsdUserName = "cbsd"

var pwProgram = "/usr/sbin/pw"

var cbsdUser *user.User = nil

// var cbsdDatabaseName = "file:/usr/local/jails/cbsd/var/db/local.sqlite?mode=ro"
var cbsdDatabaseName = "/var/db/local.sqlite"
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
var cbsdJailsFromDb []*Jail
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

func GetCbsdDbConnString(readwrite bool) string {
	var err error
	if cbsdUser == nil {
		cbsdUser, err = user.Lookup(cbsdUserName)
		if err != nil {
			panic(err)
		}
	}
	if readwrite {
		return "file:" + cbsdUser.HomeDir + cbsdDatabaseName + "?mode=rw"
	} else {
		return "file:" + cbsdUser.HomeDir + cbsdDatabaseName + "?mode=ro"
	}
}

func OpenHelpDialog() {
	var HelpDialog *dialog.Widget
	HelpDialog = MakeDialogForJail(
		"",
		txtProgramName,
		[]string{txtHelp},
		nil, nil, nil, nil,
		nil,
	)
	HelpDialog.Open(viewHolder, gowid.RenderWithRatio{R: 0.3}, app)
}

func CreateActionsLogDialog(editWidget *edit.Widget) *dialog.Widget {
	baheight := cbsdJailConsole.Height()
	ba := boxadapter.New(
		styled.New(
			NewEditWithScrollbar(editWidget),
			gowid.MakePaletteRef("white"),
		),
		baheight,
	)
	actionlogdialog := dialog.New(
		framed.NewUnicode(ba),
		dialog.Options{
			Buttons:         []dialog.Button{dialog.CloseD},
			Modal:           true,
			NoShadow:        true,
			TabToButtons:    true,
			BackgroundStyle: gowid.MakePaletteRef("bluebg"),
			BorderStyle:     gowid.MakePaletteRef("dialog"),
			ButtonStyle:     gowid.MakePaletteRef("white-focus"),
			FocusOnWidget:   true,
		},
	)
	return actionlogdialog
}

func MakeActionDialogForJail(jname string, title string, actions []string, actionfunc []func(jname string)) *dialog.Widget {
	MakeWidgetChangedFunction := func(actionfunc []func(jname string), ind int, jname string) gowid.WidgetChangedFunction {
		return func(app gowid.IApp, w gowid.IWidget) { actionfunc[ind](jname) }
	}
	var containers []gowid.IContainerWidget
	var lines *pile.Widget
	var cb *gowid.WidgetCallback
	menu := make([]gowid.IWidget, 0)

	var nact int = 0
	if actions != nil {
		nact = len(actions)
	}
	for i := 0; i < nact; i++ {
		//	for _, m := range (&Jail{}).GetActionsMenuItems() {
		mtext := text.New(actions[i], HALIGN_LEFT)
		mtexts := GetStyledWidget(mtext, "white")
		mbtn := button.New(mtexts, button.Options{Decoration: button.BareDecoration})
		cb = &gowid.WidgetCallback{
			Name:                  "cb_" + mtext.Content().String(),
			WidgetChangedFunction: MakeWidgetChangedFunction(actionfunc, i, jname),
		}
		mbtn.OnClick(cb)
		menu = append(menu, mbtn)
	}

	actionlist := list.NewSimpleListWalker(menu)
	actionlistst := styled.New(list.New(actionlist), gowid.MakePaletteRef("green"))
	htxt := text.New(title, HALIGN_MIDDLE)
	htxtst := styled.New(htxt, gowid.MakePaletteRef("magenta"))
	containers = append(containers, &gowid.ContainerWidget{IWidget: htxtst, D: gowid.RenderFlow{}})
	containers = append(containers, &gowid.ContainerWidget{IWidget: divider.NewUnicode(), D: gowid.RenderFlow{}})
	containers = append(containers, &gowid.ContainerWidget{IWidget: actionlistst, D: gowid.RenderFlow{}})
	lines = pile.New(containers)
	retdialog := dialog.New(
		framed.NewSpace(
			lines,
		),
		dialog.Options{
			Buttons:         []dialog.Button{dialog.CloseD},
			Modal:           true,
			NoShadow:        true,
			TabToButtons:    true,
			BackgroundStyle: gowid.MakePaletteRef("bluebg"),
			BorderStyle:     gowid.MakePaletteRef("dialog"),
			ButtonStyle:     gowid.MakePaletteRef("white-focus"),
			FocusOnWidget:   true,
		},
	)
	return retdialog
}

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

func RunMenuAction(action string) {
	log.Infof("Menu Action: " + action)

	switch action {
	// "[F1]Help ",      "[F2]Actions Menu ",    "[F3]View ",     "[F4]Edit ",     "[F5]Clone ",
	// "[F6]Export ",    "[F7]Create Snapshot ", "[F8]Destroy ",  "[F10]Exit ",    "[F11]List Snapshots ", "[F12]Start/Stop"
	case (&Jail{}).GetCommandHelp(): // Help
		OpenHelpDialog()
		return
	case (&Jail{}).GetCommandExit(): // Exit
		app.Quit()
	}

	curjail := GetSelectedJail()
	if curjail == nil {
		return
	}
	log.Infof("JailName: " + curjail.GetName())
	curjail.ExecuteActionOnCommand(action, viewHolder, app)
}

func GetSelectedJail() *Jail {
	curpos := GetSelectedPosition()
	if curpos < 0 {
		return nil
	}
	if len(cbsdJailsFromDb) < curpos {
		return nil
	}
	return cbsdJailsFromDb[curpos]
}

func GetSelectedPosition() int {
	ifocus := cbsdListJails.Walker().Focus()
	return int(ifocus.(list.ListPos)) - 1
}

func RefreshJailList() {
	var err error
	cbsdJailsFromDb, err = GetJailsFromDb(GetCbsdDbConnString(false))
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

func UpdateJailStatus(jail *Jail) {
	_, _ = jail.UpdateJailFromDb(GetCbsdDbConnString(false))
	UpdateJailLine(jail)
}

func UpdateJailLine(jail *Jail) {
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

func GetMenuButton(jail *Jail) *keypress.Widget {
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

func ExecCommand(title string, command string, args []string) {
	var cmd *exec.Cmd
	logspace := edit.New(edit.Options{ReadOnly: true})
	outdlg := CreateActionsLogDialog(logspace)
	/*
		if cbsdActionsDialog != nil {
			if cbsdActionsDialog.IsOpen() {
				cbsdActionsDialog.Close(app)
			}
		}
	*/
	outdlg.Open(viewHolder, gowid.RenderWithRatio{R: 0.7}, app)
	app.RedrawTerminal()
	cmd = exec.Command(command, args...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "NOCOLOR=1")
	cmd.Stderr = cmd.Stdout
	cmdout, err := cmd.StdoutPipe()
	defer cmdout.Close()
	if err != nil {
		log.Errorf("cmdout creation failed with %s\n", err)
	}
	scanner := bufio.NewScanner(cmdout)
	//scanner.Buffer(make([]byte, MAXBUF), MAXBUF)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for scanner.Scan() {
			logText = logspace.Text() + scanner.Text() + "\n"
			app.RunThenRenderEvent(gowid.RunFunction(func(app gowid.IApp) {
				logspace.SetText(logText, app)
				logspace.SetCursorPos(utf8.RuneCountInString(logspace.Text()), app)
			}))
			//app.RedrawTerminal()
		}
		wg.Done()
	}()
	err = cmd.Start()
	if err != nil {
		log.Errorf("cmd.Start() failed with %s\n", err)
	}
	wg.Wait()
	err = cmd.Wait()
	if err != nil {
		log.Errorf("cmd.Wait() failed with %s\n", err)
	}
}

func ExecShellCommand(title string, command string, args []string, logfile string) {
	var cmd *exec.Cmd
	var file *os.File
	var err error
	MAXBUF := 1000000
	buf := make([]byte, MAXBUF)
	log.Infof("Trying to start %s command with %v arguments", command, args)
	logspace := edit.New(edit.Options{ReadOnly: true})
	outdlg := CreateActionsLogDialog(logspace)
	/*
		if cbsdActionsDialog != nil {
			if cbsdActionsDialog.IsOpen() {
				cbsdActionsDialog.Close(app)
			}
		}
	*/
	outdlg.Open(viewHolder, gowid.RenderWithRatio{R: 0.7}, app)
	app.RedrawTerminal()
	cmd = exec.Command(command, args...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "NOCOLOR=1")
	file, err = os.OpenFile(logfile, os.O_TRUNC|os.O_RDWR, 0644)
	if os.IsNotExist(err) {
		file, err = os.OpenFile(logfile, os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		if err != nil {
			log.Fatal(err)
		}
	}
	file.Close()
	chanfread := make(chan int)

	go func() {
		var rbytes int
		file, err = os.OpenFile(logfile, os.O_RDONLY|os.O_SYNC, 0644)
		if err != nil {
			log.Fatal(err)
		}
		fsize, err := file.Stat()
		if err != nil {
			log.Fatal(err)
		}
		oldfsize := fsize.Size()
		for true {
			fsize, err = file.Stat()
			if err != nil {
				log.Fatal(err)
			}
			if fsize.Size() > oldfsize {
				oldfsize = fsize.Size()
				if fsize.Size() > int64(MAXBUF) {
					log.Errorf(command + " produced output is too long, it will be truncated\n")
					break
				}
				rbytes, err = file.Read(buf)
				if rbytes > 0 {
					logText = logspace.Text() + string(buf[:rbytes]) + "\n"
					app.RunThenRenderEvent(gowid.RunFunction(func(app gowid.IApp) {
						logspace.SetText(logText, app)
						logspace.SetCursorPos(utf8.RuneCountInString(logspace.Text()), app)
						app.Sync()
					}))
					//app.RedrawTerminal()
				}
			}
			select {
			case <-chanfread:
				break
			default:
				time.Sleep(300 * time.Millisecond)
			}
		}
		file.Close()
	}()

	err = cmd.Start()
	if err != nil {
		log.Errorf("cmd.Start() failed with %s\n", err)
	}
	err = cmd.Wait()
	if err != nil {
		log.Errorf("cmd.Wait() failed with %s\n", err)
	}
	chanfread <- 1
}

func LogError(strerr string, err error) {
	log.Errorf(strerr+": %w", err)
}

func GetJailByName(jname string) *Jail {
	var jail *Jail = nil
	for _, j := range cbsdJailsFromDb {
		if j.GetName() == jname {
			jail = j
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
		if doas {
			SendTerminalCommand(doasProgram + " " + cbsdProgram + " " + jail.GetLoginCommand())
		} else {
			SendTerminalCommand(cbsdProgram + " " + jail.GetLoginCommand())
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
	for _, h := range (&Jail{}).GetHeaderTitles() {
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
	for i, jail := range cbsdJailsFromDb {
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
		curjail.OpenActionDialog(viewHolder, app)
	case tcell.KeyCtrlR:
		RefreshJailList()
	case tcell.KeyTab:
		if next, ok := cbsdWidgets.FindNextSelectable(gowid.Forwards, true); ok {
			cbsdWidgets.SetFocus(app, next)
		}
	}
}

func MakeGridLine(jail *Jail) []gowid.IWidget {
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
	for _, jail := range cbsdJailsFromDb {
		line := MakeGridLine(jail)
		lines = append(lines, line)
	}
	return lines
}

func GetNodeName() string {
	nname := ""
	cmd := exec.Command(pwProgram, "user", "show", cbsdUserName)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		log.Fatalf("cmd.Run() failed with %s\n", err)
	}
	cbsd_user_conf := string(stdout.Bytes())
	cbsd_home_dir := strings.Split(cbsd_user_conf, ":")[8]
	cbsd_nodename_file := cbsd_home_dir + "/nodename"
	nnfile, err := os.Open(cbsd_nodename_file)
	if err != nil {
		log.Fatal(err)
	}
	defer nnfile.Close()
	scanner := bufio.NewScanner(nnfile)
	for scanner.Scan() {
		nname = scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	return nname
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
		curjail.ExecuteActionOnKey(int16(ekey), viewHolder, app.(*gowid.App))
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
	for i, m := range (&Jail{}).GetBottomMenuText2() {
		mtext1 := text.New((&Jail{}).GetBottomMenuText1()[i], HALIGN_LEFT)
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

	cbsdJailsFromDb, err = GetJailsFromDb(GetCbsdDbConnString(false))
	if err != nil {
		panic(err)
	}

	if len(cbsdJailsFromDb) < 1 {
		log.Errorf("Cannot find jails in database %s", cbsdDatabaseName)
		return
	}

	f := RedirectLogger(logFileName)
	defer f.Close()

	curuser, err := user.Current()
	if err == nil {
		if curuser.Username == "root" {
			doas = false
		}
	} else {
		log.Errorf("Error from user.Current(): %s", err)
	}

	//cbsdJlsHeader = cbsdJailsFromDb[0].GetHeaderTitles()
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

	ExitOnErr(err)
	SetJailListFocus()
	app.MainLoop(handler{})
}
