package main

import (
	"bufio"
	"bytes"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/gcla/gowid"
	"github.com/gcla/gowid/vim"
	"github.com/gcla/gowid/widgets/button"
	"github.com/gcla/gowid/widgets/cellmod"
	"github.com/gcla/gowid/widgets/columns"
	"github.com/gcla/gowid/widgets/dialog"
	"github.com/gcla/gowid/widgets/edit"
	"github.com/gcla/gowid/widgets/fill"
	"github.com/gcla/gowid/widgets/framed"
	"github.com/gcla/gowid/widgets/grid"
	"github.com/gcla/gowid/widgets/holder"
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

type CbsdJail struct {
	Name       string
	IsRunning  bool
	IsRunnable bool
	Parameters []PairString
}

var USE_DOAS = false

var txtProgramName = "CBSD-TUI"
var txtHelp = `- To navigate in jails list use 'Up' and 'Down' keys or mouse
- To open 'Actions' menu for the selected jail use 'F2' key
- To login into the selected jail (as root) use 'Enter' key
- To switch to terminal from jails list use 'Tab' key
- To switch to jails list from terminal use 'Ctrl-Z'+'Tab' keys sequence
- Use bottom menu ('Fx' keys or mouse clicks) to start actions on the selected jail`

var doasProgram = "/usr/local/bin/doas"
var cbsdJlsDisplay = []string{"jname", "ip4_addr", "host_hostname", "status", "astart", "ver", "path", "interface", "baserw", "vnet"}
var cbsdProgram = "/usr/local/bin/cbsd"
var cbsdCommandJailLogin = "jlogin"
var cbsdArgJailName = "jname"
var cbsdUser = "cbsd"
var cbsdJlsHeader = []string{"NAME", "IP4_ADDRESS", "HOSTNAME", "STATUS", "AUTOSTART", "VERSION", "PATH", "INTERFACE", "BASERW", "VNET"}
var cbsdJlsBlackList = []string{"PATH", "INTERFACE", "HOSTNAME", "BASERW", "VNET"}
var pwProgram = "/usr/sbin/pw"
var cbsdJails []*CbsdJail
var cbsdJailHeader []gowid.IWidget
var cbsdJailsLines [][]gowid.IWidget
var cbsdJailsGrid []gowid.IWidget
var cbsdBottomMenu []gowid.IContainerWidget
var cbsdListWalker *list.SimpleListWalker
var cbsdJailConsole *terminal.Widget
var cbsdWidgets *ResizeablePileWidget
var cbsdJailConsoleActive string
var WIDTH = 18
var HPAD = 2
var VPAD = 1

//var cbsdActionsMenuText = []string{"Start/Stop", "Create Snapshot", "List Snapshots", "Clone", "Export", "Migrate", "Destroy", "Makeresolv", "Show Config"}
var cbsdActionsMenuText = []string{"Start/Stop", "Create Snapshot", "List Snapshots", "Clone", "Export"}
var cbsdBottomMenuText = []string{"[F1]Help ", "[F2]Actions Menu ", "[F5]Clone ", "[F6]Export ", "[F7]Create Snapshot ", "[F10]Exit ", "[F11]List Snapshots ", "[F12]Start/Stop"}
var cbsdActionsMenu map[string][]gowid.IWidget
var cbsdActionsDialog *dialog.Widget
var cbsdCloneJailDialog *dialog.Widget
var cbsdSnapshotJailDialog *dialog.Widget
var cbsdListJails *list.Widget

var app *gowid.App
var menu2 *menu.Widget
var viewHolder *holder.Widget

type handler struct{}

func CreateCbsdJailActionsDialog(jname string) *dialog.Widget {
	actionlist := list.NewSimpleListWalker(cbsdActionsMenu[jname])
	actionlistst := styled.New(list.New(actionlist), gowid.MakePaletteRef("green"))
	actiondialog := dialog.New(
		framed.NewSpace(
			actionlistst,
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
	return actiondialog
}

func CreateHelpDialog() *dialog.Widget {
	txthead := text.New(txtProgramName, text.Options{Align: gowid.HAlignMiddle{}})
	txtheadst := styled.New(txthead, gowid.MakePaletteRef("magenta"))
	txthelp := text.New(txtHelp, text.Options{Align: gowid.HAlignLeft{}})
	txthelpst := styled.New(txthelp, gowid.MakePaletteRef("white"))
	/*
		sb := vscroll.NewExt(vscroll.VerticalScrollbarUnicodeRunes)
		col := columns.New([]gowid.IContainerWidget{
			&gowid.ContainerWidget{txtoutst, gowid.RenderWithWeight{W: 1}},
			&gowid.ContainerWidget{sb, gowid.RenderWithUnits{U: 1}},
		})
	*/
	helplines := pile.New([]gowid.IContainerWidget{
		&gowid.ContainerWidget{txtheadst, gowid.RenderFlow{}},
		&gowid.ContainerWidget{txthelpst, gowid.RenderFlow{}},
	})
	helpdialog := dialog.New(
		framed.NewSpace(
			helplines,
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
	return helpdialog
}

func CreateActionsLogDialog(txtout *text.Widget) *dialog.Widget {
	txtoutst := styled.New(txtout, gowid.MakePaletteRef("white"))
	/*
		sb := vscroll.NewExt(vscroll.VerticalScrollbarUnicodeRunes)
		col := columns.New([]gowid.IContainerWidget{
			&gowid.ContainerWidget{txtoutst, gowid.RenderWithWeight{W: 1}},
			&gowid.ContainerWidget{sb, gowid.RenderWithUnits{U: 1}},
		})
	*/
	actionlogdialog := dialog.New(
		framed.NewSpace(
			txtoutst,
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
	return actionlogdialog
}

func MakeSnapshotJailDialog(jname string) *dialog.Widget {
	htxt := text.New("Snapshot jail "+jname, text.Options{Align: gowid.HAlignMiddle{}})
	htxtst := styled.New(htxt, gowid.MakePaletteRef("magenta"))
	edsnapname := edit.New(edit.Options{Caption: "Snapshot name: ", Text: "gettimeofday"})
	edsnapnamest := styled.New(edsnapname, gowid.MakePaletteRef("green"))
	edlines := pile.New([]gowid.IContainerWidget{
		&gowid.ContainerWidget{htxtst, gowid.RenderFlow{}},
		&gowid.ContainerWidget{edsnapnamest, gowid.RenderFlow{}},
	})
	Ok := dialog.Button{
		Msg: "OK",
		Action: gowid.MakeWidgetCallback("execsnapjail", gowid.WidgetChangedFunction(func(app gowid.IApp, w gowid.IWidget) {
			cbsdSnapshotJailDialog.Close(app)
			DoSnapshotJail(jname, edsnapname.Text())
		})),
	}
	Cancel := dialog.Button{
		Msg: "Cancel",
	}
	snapjaildialog := dialog.New(
		//edlines,
		framed.NewSpace(
			edlines,
		),
		dialog.Options{
			Buttons:         []dialog.Button{Ok, Cancel},
			NoShadow:        true,
			BackgroundStyle: gowid.MakePaletteRef("bluebg"),
			BorderStyle:     gowid.MakePaletteRef("dialog"),
			ButtonStyle:     gowid.MakePaletteRef("white-focus"),
			Modal:           true,
			FocusOnWidget:   true,
		},
	)
	return snapjaildialog
}

func MakeCloneJailDialog(jname string) *dialog.Widget {
	htxt := text.New("Clone jail "+jname, text.Options{Align: gowid.HAlignMiddle{}})
	htxtst := styled.New(htxt, gowid.MakePaletteRef("magenta"))
	ednewjname := edit.New(edit.Options{Caption: "New jail name: ", Text: jname + "clone"})
	ednewjnamest := styled.New(ednewjname, gowid.MakePaletteRef("green"))
	ednewhname := edit.New(edit.Options{Caption: "New host name: ", Text: jname})
	ednewhnamest := styled.New(ednewhname, gowid.MakePaletteRef("green"))
	ednewip := edit.New(edit.Options{Caption: "New IP address: ", Text: "DHCP"})
	ednewipst := styled.New(ednewip, gowid.MakePaletteRef("green"))
	edlines := pile.New([]gowid.IContainerWidget{
		&gowid.ContainerWidget{htxtst, gowid.RenderFlow{}},
		&gowid.ContainerWidget{ednewjnamest, gowid.RenderFlow{}},
		&gowid.ContainerWidget{ednewhnamest, gowid.RenderFlow{}},
		&gowid.ContainerWidget{ednewipst, gowid.RenderFlow{}},
	})
	Ok := dialog.Button{
		Msg: "OK",
		Action: gowid.MakeWidgetCallback("execclonejail", gowid.WidgetChangedFunction(func(app gowid.IApp, w gowid.IWidget) {
			cbsdCloneJailDialog.Close(app)
			DoCloneJail(jname, ednewjname.Text(), ednewhname.Text(), ednewip.Text())
		})),
	}
	Cancel := dialog.Button{
		Msg: "Cancel",
	}
	clonejaildialog := dialog.New(
		//edlines,
		framed.NewSpace(
			edlines,
		),
		dialog.Options{
			Buttons:         []dialog.Button{Ok, Cancel},
			NoShadow:        true,
			BackgroundStyle: gowid.MakePaletteRef("bluebg"),
			BorderStyle:     gowid.MakePaletteRef("dialog"),
			ButtonStyle:     gowid.MakePaletteRef("white-focus"),
			Modal:           true,
			FocusOnWidget:   true,
		},
	)
	return clonejaildialog
	/*
			msg := text.New("Do you want to quit?")
			yesno = dialog.New(
				framed.NewSpace(hpadding.New(msg, gowid.HAlignMiddle{}, gowid.RenderFixed{})),
				dialog.Options{
					Buttons: dialog.OkCancel,
				},
			)
			yesno.Open(viewHolder, gowid.RenderWithRatio{R: 0.5}, app)


		Yes := dialog.Button{
			Msg: "Yes",
			Action: gowid.MakeWidgetCallback("exec", gowid.WidgetChangedFunction(func(app gowid.IApp, w gowid.IWidget) {
				termshark.ShouldSwitchTerminal = true
				switchTerm.Close(app)
				RequestQuit()
			})),
		}
		No := dialog.Button{
			Msg: "No",
		}
		NoAsk := dialog.Button{
			Msg: "No, don't ask",
			Action: gowid.MakeWidgetCallback("exec", gowid.WidgetChangedFunction(func(app gowid.IApp, w gowid.IWidget) {
				termshark.SetConf("main.disable-term-helper", true)
				switchTerm.Close(app)
			})),
		}
		switchTerm = dialog.New(
			framed.NewSpace(paragraph.New(fmt.Sprintf("Termshark is running with TERM=%s. The terminal database contains %s. Would you like to switch for a more colorful experience? Termshark will need to restart.", term, term256))),
			dialog.Options{
				Buttons:         []dialog.Button{Yes, No, NoAsk},
				NoShadow:        true,
				BackgroundStyle: gowid.MakePaletteRef("dialog"),
				BorderStyle:     gowid.MakePaletteRef("dialog"),
				ButtonStyle:     gowid.MakePaletteRef("dialog-button"),
				Modal:           true,
				FocusOnWidget:   false,
			},
		)
		switchTerm.Open(appView, gowid.RenderWithRatio{R: 0.5}, app)

	*/
}

func MakeCbsdActionsMenu() map[string][]gowid.IWidget {
	actions := make(map[string][]gowid.IWidget, 0)
	for _, j := range cbsdJails {
		actions[j.Name] = MakeCbsdJailActionsMenu(j.Name)
	}
	return actions
}

func MakeCbsdJailActionsMenu(jname string) []gowid.IWidget {
	menu := make([]gowid.IWidget, 0)
	for _, m := range cbsdActionsMenuText {
		mtext := text.New(m, text.Options{Align: gowid.HAlignLeft{}})
		mtexts := GetStyledWidget(mtext, "white")
		mbtn := button.New(mtexts, button.Options{Decoration: button.BareDecoration})
		mbtn.OnClick(gowid.WidgetCallback{"cb_" + mtext.Content().String(), func(app gowid.IApp, w gowid.IWidget) {
			app.Run(gowid.RunFunction(func(app gowid.IApp) {
				RunActionOnJail(mtext.Content().String(), jname)
			}))
		}})
		menu = append(menu, mbtn)
	}
	return menu
}

func RunActionOnJail(action string, jname string) {
	log.Infof("Action: " + action + " on jail: " + jname)
	jail := GetJailByName(jname)
	if jail == nil {
		log.Errorf("Cannot find jail: " + jname)
		return
	}
	switch action {
	//	case cbsdActionsMenuText[0]: // Start/Stop
	//		jail.StartStopJail()
	case "Start":
		jail.StartStopJail()
	case "Stop":
		jail.StartStopJail()
	case "Create Snapshot":
		jail.SnapshotJail()
	case "List Snapshots":
		jail.ListSnapshotsJail()
	case "Clone":
		jail.CloneJail()
	case "Export":
		jail.ExportJail()
	}
}

func RunMenuAction(action string) {
	log.Infof("Menu Action: " + action)
	jname := GetSelectedJailName()
	log.Infof("JailName: " + jname)
	jail := GetJailByName(jname)
	if jail == nil {
		log.Errorf("Cannot find jail: " + jname)
		return
	}
	switch action {
	// "[F1]Help ",            "[F2]Actions Menu ", "[F5]Clone ",           "[F6]Export ",
	// "[F7]Create Snapshot ", "[F10]Exit ",        "[F11]List Snapshots ", "[F12]Start/Stop"
	case cbsdBottomMenuText[0]: // Help
		helpdialog := CreateHelpDialog()
		helpdialog.Open(viewHolder, gowid.RenderWithRatio{R: 0.6}, app)
	case cbsdBottomMenuText[1]: // Actions Menu
		OpenJailActionsMenu(jname)
	case cbsdBottomMenuText[2]: // Clone
		jail.CloneJail()
	case cbsdBottomMenuText[3]: // Export
		jail.ExportJail()
	case cbsdBottomMenuText[4]: // Create Snapshot
		jail.SnapshotJail()
	case cbsdBottomMenuText[5]: // Exit
		app.Close()
	case cbsdBottomMenuText[6]: // List Snapshots
		jail.ListSnapshotsJail()
	case cbsdBottomMenuText[7]: // Start/Stop
		jail.StartStopJail()
	}
}

func GetSelectedJailName() string {
	//cbsdListJails.Walker().SetFocus(newpos, app)
	ifocus := cbsdListJails.Walker().Focus()
	jname := cbsdJails[int(ifocus.(list.ListPos))-1].Name
	return jname
}

func DoSnapshotJail(jname string, snapname string) {
	// cbsd jsnapshot mode=create snapname=gettimeofday jname=nim1
	var command string
	txtheader := "Creating jail snapshot...\n"
	args := make([]string, 0)
	if USE_DOAS {
		args = append(args, "cbsd")
	}
	args = append(args, "jsnapshot")
	args = append(args, "mode=create")
	args = append(args, "snapname="+snapname)
	args = append(args, "jname="+jname)
	if USE_DOAS {
		command = doasProgram
	} else {
		command = cbsdProgram
	}
	ExecCommand(txtheader, command, args)
}

func DoCloneJail(jname string, jnewjname string, jnewhname string, newip string) {
	//log.Infof("Clone %s to %s (%s) IP %s", jname, jnewjname, jnewhname, newip)
	// cbsd jclone old=jail1 new=jail1clone host_hostname=jail1clone.domain.local ip4_addr=DHCP checkstate=0
	var command string
	txtheader := "Cloning jail...\n"

	args := make([]string, 0)
	if USE_DOAS {
		args = append(args, "cbsd")
	}
	args = append(args, "jclone")
	args = append(args, "old="+jname)
	args = append(args, "new="+jnewjname)
	args = append(args, "host_hostname="+jnewhname)
	args = append(args, "ip4_addr="+newip)
	args = append(args, "checkstate=0")

	if USE_DOAS {
		command = doasProgram
	} else {
		command = cbsdProgram
	}
	ExecCommand(txtheader, command, args)
	RefreshJailList()
}

func RefreshJailList() {
	cbsdJails = GetCbsdJails()
	cbsdJailsLines = MakeCbsdJailsLines()
	cbsdActionsMenu = MakeCbsdActionsMenu()
	cbsdJailsGrid = make([]gowid.IWidget, 0)
	gheader := grid.New(cbsdJailHeader, WIDTH, HPAD, VPAD, gowid.HAlignMiddle{})
	cbsdJailsGrid = append(cbsdJailsGrid, gheader)
	for _, line := range cbsdJailsLines {
		gline := grid.New(line, WIDTH, HPAD, VPAD, gowid.HAlignMiddle{},
			grid.Options{
				DownKeys: []vim.KeyPress{},
				UpKeys:   []vim.KeyPress{},
			})
		cbsdJailsGrid = append(cbsdJailsGrid, gline)
	}
	cbsdListWalker = list.NewSimpleListWalker(cbsdJailsGrid)
	cbsdListJails.SetWalker(cbsdListWalker, app)
	SetJailListFocus()
}

func (jail *CbsdJail) SnapshotJail() {
	cbsdSnapshotJailDialog = MakeSnapshotJailDialog(jail.Name)
	cbsdSnapshotJailDialog.Open(viewHolder, gowid.RenderWithRatio{R: 0.3}, app)
}

func (jail *CbsdJail) ListSnapshotsJail() {
	// cbsd jsnapshot mode=list jname=nim1
	var command string
	txtheader := "List jail snapshots...\n"
	args := make([]string, 0)
	if USE_DOAS {
		args = append(args, "cbsd")
	}
	args = append(args, "jsnapshot")
	args = append(args, "mode=list")
	args = append(args, "jname="+jail.Name)
	if USE_DOAS {
		command = doasProgram
	} else {
		command = cbsdProgram
	}
	ExecCommand(txtheader, command, args)
}

func (jail *CbsdJail) CloneJail() {
	cbsdCloneJailDialog = MakeCloneJailDialog(jail.Name)
	cbsdCloneJailDialog.Open(viewHolder, gowid.RenderWithRatio{R: 0.3}, app)
}

func (jail *CbsdJail) ExportJail() {
	// cbsd jexport jname=nim1
	var command string
	txtheader := "Exporting jail...\n"

	args := make([]string, 0)
	if USE_DOAS {
		args = append(args, "cbsd")
	}
	args = append(args, "jexport")
	args = append(args, "jname="+jail.Name)

	if USE_DOAS {
		command = doasProgram
	} else {
		command = cbsdProgram
	}
	ExecCommand(txtheader, command, args)
}

func GetJailStatus(jname string) string {
	var stdout, stderr bytes.Buffer
	//var jid int
	retstatus := "Unknown"
	cmd_args := make([]string, 0)
	cmd_args = append(cmd_args, "jstatus")
	cmd_args = append(cmd_args, "invert=true")
	cmd_args = append(cmd_args, "jname="+jname)
	cmd := exec.Command(cbsdProgram, cmd_args...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "NOCOLOR=1")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		log.Errorf("cmd.Run() failed with %s\n", err)
	}
	str_out := string(stdout.Bytes())
	str_out = strings.TrimSuffix(str_out, "\n")
	if str_out != "" {
		jid, err := strconv.Atoi(str_out)
		if err != nil {
			log.Errorf("cbsd jstatus incorrect return %s\n", err)
		}
		if jid > 0 {
			retstatus = "On"
		} else if jid == 0 {
			retstatus = "Off"
		}
	}
	return retstatus
}

func (jail *CbsdJail) UpdateJailStatus() {
	jstatus := GetJailStatus(jail.Name)
	status_changed := false
	switch jstatus {
	case "On":
		if (*jail).IsRunning == false {
			(*jail).IsRunning = true
			status_changed = true
		}
	case "Off":
		if (*jail).IsRunning {
			(*jail).IsRunning = false
			(*jail).IsRunnable = true
			status_changed = true
		}
	default:
		if (*jail).IsRunning {
			(*jail).IsRunning = false
			status_changed = true
		}
		if (*jail).IsRunnable {
			(*jail).IsRunnable = false
			status_changed = true
		}
	}
	if status_changed == false {
		return
	}
	for i, p := range (*jail).Parameters {
		if p.Key == "STATUS" {
			switch (*jail).IsRunning {
			case true:
				(*jail).Parameters[i].Value = "On"
				//p.Value = "On"
			case false:
				(*jail).Parameters[i].Value = "Off"
				//p.Value = "Off"
			}
		}
	}
	jail.UpdateCbsdJailLine()
}

func (jail CbsdJail) UpdateCbsdJailLine() {
	i := 1
	for _, line := range cbsdJailsLines {
		btn := line[0].(*keypress.Widget).SubWidget().(*cellmod.Widget).SubWidget().(*button.Widget)
		txt := btn.SubWidget().(*styled.Widget).SubWidget().(*text.Widget)
		str := txt.Content().String()
		if str != jail.Name {
			continue
		}
		i = 1
		style := GetJailStyle(&jail)
		for _, param := range jail.Parameters {
			if param.Key == "NAME" {
				continue
			}
			if IsBlackListed(param.Key) {
				continue
			}
			str := param.Value
			if param.Key == "AUTOSTART" {
				switch param.Value {
				case "0":
					str = "Off"
				case "1":
					str = "On"
				}
			}
			if param.Key == "BASERW" {
				switch param.Value {
				case "0":
					str = "Yes"
				case "1":
					str = "No"
				}
			}
			if param.Key == "VNET" {
				switch param.Value {
				case "0":
					str = "Yes"
				case "1":
					str = "No"
				}
			}
			ptxt := text.New(str, text.Options{Align: gowid.HAlignMiddle{}})
			ptxts := GetStyledWidget(ptxt, style)
			line[i] = ptxts
			i++
		}
		line[0] = jail.GetMenuButton()
	}
}

func (jail CbsdJail) GetMenuButton() *keypress.Widget {
	btxt := text.New(jail.Name, text.Options{Align: gowid.HAlignMiddle{}})
	style := GetJailStyle(&jail)
	txts := GetStyledWidget(btxt, style)
	btnnew := button.New(txts, button.Options{
		Decoration: button.BareDecoration,
	})
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
	txtout := text.New(title, text.Options{Align: gowid.HAlignLeft{}})
	outdlg := CreateActionsLogDialog(txtout)
	if cbsdActionsDialog != nil {
		if cbsdActionsDialog.IsOpen() {
			cbsdActionsDialog.Close(app)
		}
	}
	outdlg.Open(viewHolder, gowid.RenderWithRatio{R: 0.7}, app)
	outdlgwriter := text.Writer{txtout, app}
	cmd = exec.Command(command, args...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "NOCOLOR=1")
	cmdout, err := cmd.StdoutPipe()
	defer cmdout.Close()
	if err != nil {
		log.Errorf("cmdout creation failed with %s\n", err)
	}
	cmd.Stderr = cmd.Stdout
	cmdchan := make(chan struct{})
	scanner := bufio.NewScanner(cmdout)
	go func() {
		for scanner.Scan() {
			cr := []byte("\n")
			txtbytes := []byte(txtout.Content().String())
			txtbytes = append(txtbytes, cr...)
			txtbytes = append(txtbytes, scanner.Bytes()...)
			outdlgwriter.Write(txtbytes)
			app.RedrawTerminal()
		}
		cmdchan <- struct{}{}
	}()
	err = cmd.Start()
	if err != nil {
		log.Errorf("cmd.Start() failed with %s\n", err)
	}
	<-cmdchan
	err = cmd.Wait()
	if err != nil {
		log.Errorf("cmd.Wait() failed with %s\n", err)
	}
}

func (jail *CbsdJail) StartStopJail() {
	txtheader := ""
	var args []string
	var command string
	if jail.IsRunning {
		if cbsdJailConsoleActive == jail.Name {
			SendTerminalCommand("exit")
			cbsdJailConsoleActive = ""
		}
		txtheader = "Stopping jail...\n"
		if USE_DOAS {
			args = append(args, "cbsd")
		}
		args = append(args, "jstop")
		args = append(args, "inter=1")
		args = append(args, "jname="+jail.Name)
	} else if jail.IsRunnable {
		txtheader = "Starting jail...\n"
		if USE_DOAS {
			args = append(args, "cbsd")
		}
		args = append(args, "jstart")
		args = append(args, "inter=1")
		args = append(args, "jname="+jail.Name)
	}
	if USE_DOAS {
		command = doasProgram
	} else {
		command = cbsdProgram
	}
	if (!jail.IsRunning && jail.IsRunnable) || jail.IsRunning {
		ExecCommand(txtheader, command, args)
		jail.UpdateJailStatus()
	}
}

func GetJailByName(jname string) *CbsdJail {
	var jail *CbsdJail = nil
	for _, j := range cbsdJails {
		if j.Name == jname {
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
	if jail != nil && jail.IsRunning {
		if cbsdJailConsoleActive != "" {
			SendTerminalCommand("\x03")
			SendTerminalCommand("exit")
		}
		if USE_DOAS {
			SendTerminalCommand(doasProgram + " " + "cbsd" + " " + cbsdCommandJailLogin + " " + cbsdArgJailName + "=" + jname)
		} else {
			SendTerminalCommand(cbsdProgram + " " + cbsdCommandJailLogin + " " + cbsdArgJailName + "=" + jname)
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

func NewCbsdJail() *CbsdJail {
	jail := CbsdJail{
		Name:       "",
		IsRunning:  false,
		IsRunnable: false,
		Parameters: make([]PairString, 0),
	}
	return &jail
}

func NewCbsdJailN(jname string) *CbsdJail {
	jail := CbsdJail{
		Name:       jname,
		IsRunning:  false,
		IsRunnable: false,
		Parameters: make([]PairString, 0),
	}
	return &jail
}

func NewCbsdJailNR(jname string, ir bool) *CbsdJail {
	jail := CbsdJail{
		Name:       jname,
		IsRunning:  ir,
		Parameters: make([]PairString, 0),
	}
	return &jail
}

func GetCbsdJlsHeader() []gowid.IWidget {
	header := make([]gowid.IWidget, 0)
	found := false
	for _, h := range cbsdJlsHeader {
		found = false
		for _, bh := range cbsdJlsBlackList {
			if bh == h {
				found = true
				break
			}
		}
		if found {
			continue
		}
		htext := text.New(h, text.Options{Align: gowid.HAlignMiddle{}})
		header = append(header, GetStyledWidget(htext, "white"))
	}
	return header
}

func GetJailStyle(jail *CbsdJail) string {
	style := "gray"
	status := ""
	astart := ""
	for _, p := range jail.Parameters {
		if p.Key == "STATUS" {
			status = p.Value
		}
		if p.Key == "AUTOSTART" {
			astart = p.Value
		}
	}
	if status == "On" {
		style = "green"
	} else if status == "Off" {
		switch astart {
		case "1":
			style = "red"
		default:
			style = "white"
		}
	}
	return style
}

func SetJailListFocus() {
	newpos := list.ListPos(0)
	for i, jail := range cbsdJails {
		if jail.IsRunning {
			newpos = list.ListPos(i + 1)
			break
		}
	}
	cbsdListJails.Walker().SetFocus(newpos, app)
}

func OpenJailActionsMenu(jname string) {
	btn := cbsdActionsMenu[jname][0].(*button.Widget)
	txt := btn.SubWidget().(*styled.Widget).SubWidget().(*text.Widget)
	wr := text.Writer{txt, app}
	jail := GetJailByName(jname)
	if jail.IsRunning {
		wr.Write([]byte("Stop"))
	} else {
		if jail.IsRunnable {
			wr.Write([]byte("Start"))
		} else {
			wr.Write([]byte("--Not Runnable--"))
		}
	}
	cbsdActionsDialog = CreateCbsdJailActionsDialog(jname)
	cbsdActionsDialog.Open(viewHolder, gowid.RenderWithRatio{R: 0.3}, app)
}

func JailListButtonCallBack(jname string, key gowid.IKey) {
	switch key.Key() {
	case tcell.KeyEnter:
		LoginToJail(jname)
	case tcell.KeyF2:
		OpenJailActionsMenu(jname)
	case tcell.KeyCtrlR:
		RefreshJailList()
	case tcell.KeyTab:
		if next, ok := cbsdWidgets.FindNextSelectable(gowid.Forwards, true); ok {
			cbsdWidgets.SetFocus(app, next)
		}
	}
}
func (jail *CbsdJail) MakeGridLine() []gowid.IWidget {
	style := "gray"
	line := make([]gowid.IWidget, 0)
	style = GetJailStyle(jail)
	line = append(line, jail.GetMenuButton())
	for _, param := range jail.Parameters {
		if param.Key == "NAME" {
			continue
		}
		if IsBlackListed(param.Key) {
			continue
		}
		str := param.Value
		if param.Key == "AUTOSTART" {
			switch param.Value {
			case "0":
				str = "Off"
			case "1":
				str = "On"
			}
		}
		if param.Key == "BASERW" {
			switch param.Value {
			case "0":
				str = "Yes"
			case "1":
				str = "No"
			}
		}
		if param.Key == "VNET" {
			switch param.Value {
			case "0":
				str = "Yes"
			case "1":
				str = "No"
			}
		}
		ptxt := text.New(str, text.Options{Align: gowid.HAlignMiddle{}})
		ptxts := GetStyledWidget(ptxt, style)
		line = append(line, ptxts)
	}
	return line
}

func MakeCbsdJailsLines() [][]gowid.IWidget {
	lines := make([][]gowid.IWidget, 0)
	for _, jail := range cbsdJails {
		line := (*jail).MakeGridLine()
		lines = append(lines, line)
	}
	return lines
}

func GetCbsdJails() []*CbsdJail {
	jails := make([]*CbsdJail, 0)
	var stdout, stderr bytes.Buffer

	args := GetCbsdJlsCommandArgs()
	cmd := exec.Command(cbsdProgram, args...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "NOCOLOR=1")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		log.Fatalf("cmd.Run() failed with %s\n", err)
	}
	str_out := string(stdout.Bytes())
	str_jails := strings.Split(str_out, "\n")
	for _, s := range str_jails {
		fields := strings.Fields(s)
		if len(fields) <= 1 {
			continue
		}
		jail := NewCbsdJailN(fields[0])
		len := int(math.Min(float64(len(fields)), float64(len(cbsdJlsHeader))))
		for i := 1; i < len; i++ {
			if cbsdJlsHeader[i] == "STATUS" {
				if fields[i] == "On" {
					jail.IsRunning = true
				} else {
					jail.IsRunning = false
				}
				if fields[i] == "Off" {
					jail.IsRunnable = true
				} else {
					jail.IsRunnable = false
				}
			}
			jail.Parameters = append(jail.Parameters, PairString{cbsdJlsHeader[i], fields[i]})
		}
		jails = append(jails, jail)
	}
	return jails
}

func IsBlackListed(param string) bool {
	found := false
	for _, bh := range cbsdJlsBlackList {
		if bh == param {
			found = true
			break
		}
	}
	return found
}

func GetCbsdJlsCommandArgs() []string {
	cmd_args := make([]string, 0)
	cmd_args = append(cmd_args, "jls")
	cmd_args = append(cmd_args, "header=0")
	arg_display := "display="
	for _, f := range cbsdJlsDisplay {
		arg_display += f + ","
	}
	cmd_args = append(cmd_args, arg_display)
	return cmd_args
}

func GetNodeName() string {
	nname := ""
	cmd := exec.Command(pwProgram, "user", "show", cbsdUser)
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
		// "[F1]Help ",            "[F2]Actions Menu ", "[F5]Clone ",           "[F6]Export ",
		// "[F7]Create Snapshot ", "[F10]Exit ",        "[F11]List Snapshots ", "[F12]Start/Stop"
		switch evk.Key() {
		case tcell.KeyCtrlC, tcell.KeyEsc, tcell.KeyF10:
			app.Quit()
		case tcell.KeyTab:
			if next, ok := cbsdWidgets.FindNextSelectable(gowid.Forwards, true); ok {
				cbsdWidgets.SetFocus(app, next)
			}
		case tcell.KeyF1:
			RunMenuAction(cbsdBottomMenuText[0])
		case tcell.KeyF2:
			RunMenuAction(cbsdBottomMenuText[1])
		case tcell.KeyF5:
			RunMenuAction(cbsdBottomMenuText[2])
		case tcell.KeyF6:
			RunMenuAction(cbsdBottomMenuText[3])
		case tcell.KeyF7:
			RunMenuAction(cbsdBottomMenuText[4])
		case tcell.KeyF11:
			RunMenuAction(cbsdBottomMenuText[6])
		case tcell.KeyF12:
			RunMenuAction(cbsdBottomMenuText[7])
		default:
			handled = false
		}
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
		[]styled.AttributeRange{styled.AttributeRange{0, -1, gowid.MakePaletteRef(cnofocus)}},
		[]styled.AttributeRange{styled.AttributeRange{0, -1, gowid.MakePaletteRef(cfocus)}})
}

func MakeBottomMenu() {
	cbsdBottomMenu = make([]gowid.IContainerWidget, 0)
	for _, m := range cbsdBottomMenuText {
		mtext := text.New(m, text.Options{Align: gowid.HAlignLeft{}})
		mbtn := button.New(mtext, button.Options{Decoration: button.BareDecoration})
		mbtns := GetStyledWidget(mbtn, "red")
		mbtn.OnClick(gowid.WidgetCallback{"cbb_" + mtext.Content().String(), func(app gowid.IApp, w gowid.IWidget) {
			app.Run(gowid.RunFunction(func(app gowid.IApp) {
				RunMenuAction(mtext.Content().String())
			}))
		}})
		cbsdBottomMenu = append(cbsdBottomMenu, &gowid.ContainerWidget{IWidget: mbtns, D: gowid.RenderFixed{}})
	}
}

func main() {
	var err error

	f := RedirectLogger("/var/log/cbsd-tui.log")
	defer f.Close()

	palette := gowid.Palette{
		"red-nofocus":   gowid.MakePaletteEntry(gowid.ColorRed, gowid.ColorNone),
		"red-focus":     gowid.MakePaletteEntry(gowid.ColorBlack, gowid.ColorRed),
		"green-nofocus": gowid.MakePaletteEntry(gowid.ColorGreen, gowid.ColorNone),
		"green-focus":   gowid.MakePaletteEntry(gowid.ColorBlack, gowid.ColorGreen),
		"white-nofocus": gowid.MakePaletteEntry(gowid.ColorWhite, gowid.ColorNone),
		"white-focus":   gowid.MakePaletteEntry(gowid.ColorBlack, gowid.ColorWhite),
		"gray-nofocus":  gowid.MakePaletteEntry(gowid.ColorLightGray, gowid.ColorNone),
		"gray-focus":    gowid.MakePaletteEntry(gowid.ColorBlack, gowid.ColorLightGray),
		"cyan-nofocus":  gowid.MakePaletteEntry(gowid.ColorCyan, gowid.ColorNone),
		"cyan-focus":    gowid.MakePaletteEntry(gowid.ColorBlack, gowid.ColorCyan),
		"red":           gowid.MakePaletteEntry(gowid.ColorRed, gowid.ColorNone),
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

	cbsdJails = GetCbsdJails()
	cbsdJailsLines = MakeCbsdJailsLines()
	cbsdJailHeader = GetCbsdJlsHeader()
	cbsdActionsMenu = MakeCbsdActionsMenu()

	cbsdJailsGrid = make([]gowid.IWidget, 0)
	gheader := grid.New(cbsdJailHeader, WIDTH, HPAD, VPAD, gowid.HAlignMiddle{})
	cbsdJailsGrid = append(cbsdJailsGrid, gheader)
	for _, line := range cbsdJailsLines {
		gline := grid.New(line, WIDTH, HPAD, VPAD, gowid.HAlignMiddle{},
			grid.Options{
				DownKeys: []vim.KeyPress{},
				UpKeys:   []vim.KeyPress{},
			})
		cbsdJailsGrid = append(cbsdJailsGrid, gline)
	}

	cbsdJailConsole, err = terminal.NewExt(terminal.Options{
		Command:           strings.Split(os.Getenv("SHELL"), " "),
		HotKey:            terminal.HotKey{tcell.KeyCtrlZ},
		HotKeyPersistence: &terminal.HotKeyDuration{time.Second * 2},
		Scrollback:        1000,
	})
	if err != nil {
		panic(err)
	}

	cbsdListWalker = list.NewSimpleListWalker(cbsdJailsGrid)
	cbsdListJails = list.New(cbsdListWalker)
	listjails := vpadding.New(cbsdListJails, gowid.VAlignTop{}, gowid.RenderFlow{})
	//hlptxt := text.New("Press \"Esc\" to exit", text.Options{Align: gowid.HAlignLeft{}})
	//hlptxtst := styled.New(hlptxt, gowid.MakePaletteRef("magenta"))

	/*
		clickToOpenWidgets := make([]gowid.IContainerWidget, 0)
		for i := 0; i < 20; i++ {
			btn := button.New(text.New(fmt.Sprintf("clickety%d", i)))
			btnStyled := styled.NewExt(btn, gowid.MakePaletteRef("red"), gowid.MakePaletteRef("white"))
			btnSite := menu.NewSite(menu.SiteOptions{YOffset: 1})
			btn.OnClick(gowid.WidgetCallback{gowid.ClickCB{}, func(app gowid.IApp, target gowid.IWidget) {
				menu1.Open(btnSite, app)
			}})
			clickToOpenWidgets = append(clickToOpenWidgets, &gowid.ContainerWidget{IWidget: btnSite, D: fixed})
			clickToOpenWidgets = append(clickToOpenWidgets, &gowid.ContainerWidget{IWidget: btnStyled, D: fixed})
		}
		clickToOpenCols := columns.New(clickToOpenWidgets)
	*/

	MakeBottomMenu()
	gbmenu := columns.New(cbsdBottomMenu, columns.Options{DoNotSetSelected: true, LeftKeys: make([]vim.KeyPress, 0), RightKeys: make([]vim.KeyPress, 0)})

	toppanel := NewResizeablePile([]gowid.IContainerWidget{
		&gowid.ContainerWidget{listjails, gowid.RenderWithWeight{1}},
		&gowid.ContainerWidget{gbmenu, gowid.RenderWithUnits{U: 1}},
	})
	hline := styled.New(fill.New('⎯'), gowid.MakePaletteRef("line"))

	cbsdWidgets = NewResizeablePile([]gowid.IContainerWidget{
		&gowid.ContainerWidget{toppanel, gowid.RenderWithWeight{1}},
		&gowid.ContainerWidget{hline, gowid.RenderWithUnits{U: 1}},
		&gowid.ContainerWidget{cbsdJailConsole, gowid.RenderWithWeight{1}},
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
