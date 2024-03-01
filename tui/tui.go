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

	"bufio"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/gcla/gowid"

	"github.com/gcla/gowid/widgets/boxadapter"
	"github.com/gcla/gowid/widgets/button"
	"github.com/gcla/gowid/widgets/holder"
	"github.com/gcla/gowid/widgets/list"
	"github.com/gcla/gowid/widgets/terminal"

	"github.com/gcla/gowid/widgets/checkbox"
	"github.com/gcla/gowid/widgets/columns"
	"github.com/gcla/gowid/widgets/dialog"
	"github.com/gcla/gowid/widgets/divider"
	"github.com/gcla/gowid/widgets/edit"

	"github.com/gcla/gowid/widgets/framed"
	"github.com/gcla/gowid/widgets/hpadding"
	"github.com/gcla/gowid/widgets/pile"
	"github.com/gcla/gowid/widgets/styled"

	"github.com/gcla/gowid/widgets/text"
	log "github.com/sirupsen/logrus"

	"editwithscrollbar"
)

var HALIGN_MIDDLE text.Options = text.Options{Align: gowid.HAlignMiddle{}}
var HALIGN_LEFT text.Options = text.Options{Align: gowid.HAlignLeft{}}

var CbsdJailConsoleActive = ""

type Tui struct {
	App        *gowid.App
	ViewHolder *holder.Widget
	Console    *terminal.Widget
	LogText    string
}

func NewTui(app *gowid.App, view_holder *holder.Widget, console *terminal.Widget) *Tui {
	res := &Tui{
		App:        app,
		ViewHolder: view_holder,
		Console:    console,
		LogText:    "",
	}
	return res
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

func CreateActionsLogDialog(editWidget *edit.Widget, height int) *dialog.Widget {
	ba := boxadapter.New(
		styled.New(
			editwithscrollbar.NewEditWithScrollbar(editWidget),
			gowid.MakePaletteRef("white"),
		),
		height,
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

func (tui *Tui) ExecCommand(title string, command string, args []string) {
	var cmd *exec.Cmd
	logspace := edit.New(edit.Options{ReadOnly: true})
	outdlg := CreateActionsLogDialog(logspace, tui.Console.Height())
	/*
		if cbsdActionsDialog != nil {
			if cbsdActionsDialog.IsOpen() {
				cbsdActionsDialog.Close(app)
			}
		}
	*/
	//outdlg.Callbacks.AddCallback("Close", jail.evtRestoreFocus.Emit(nil))
	outdlg.Open(tui.ViewHolder, gowid.RenderWithRatio{R: 0.7}, tui.App)
	tui.App.RedrawTerminal()
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
			tui.LogText = logspace.Text() + scanner.Text() + "\n"
			tui.App.RunThenRenderEvent(gowid.RunFunction(func(app gowid.IApp) {
				logspace.SetText(tui.LogText, app)
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

func GetStyledWidget(w gowid.IWidget, color string) *styled.Widget {
	cfocus := color + "-focus"
	cnofocus := color + "-nofocus"
	return styled.NewWithRanges(w,
		[]styled.AttributeRange{{Start: 0, End: -1, Styler: gowid.MakePaletteRef(cnofocus)}},
		[]styled.AttributeRange{{Start: 0, End: -1, Styler: gowid.MakePaletteRef(cfocus)}},
	)
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

func (tui *Tui) ExecShellCommand(title string, command string, args []string, logfile string) {
	var cmd *exec.Cmd
	var file *os.File
	var err error
	MAXBUF := 1000000
	buf := make([]byte, MAXBUF)
	log.Infof("Trying to start %s command with %v arguments", command, args)
	logspace := edit.New(edit.Options{ReadOnly: true})
	outdlg := CreateActionsLogDialog(logspace, tui.Console.Height())
	/*
		if cbsdActionsDialog != nil {
			if cbsdActionsDialog.IsOpen() {
				cbsdActionsDialog.Close(app)
			}
		}
	*/
	outdlg.Open(tui.ViewHolder, gowid.RenderWithRatio{R: 0.7}, tui.App)
	tui.App.RedrawTerminal()
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
					tui.LogText = logspace.Text() + string(buf[:rbytes]) + "\n"
					tui.App.RunThenRenderEvent(gowid.RunFunction(func(app gowid.IApp) {
						logspace.SetText(tui.LogText, app)
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

func (tui *Tui) SendTerminalCommand(cmd string) {
	tui.Console.Write([]byte(cmd + "\n"))
	time.Sleep(200 * time.Millisecond)
}

func (tui *Tui) ResetTerminal() {
	sig := syscall.Signal(9)
	tui.Console.Signal(sig)
	tui.Console.StartCommand(tui.App, tui.Console.Width(), tui.Console.Height())
}
