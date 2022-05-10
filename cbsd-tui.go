package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"sync"
	"time"

	"github.com/gcla/gowid"
	"github.com/gcla/gowid/vim"
	"github.com/gcla/gowid/widgets/button"
	"github.com/gcla/gowid/widgets/cellmod"
	"github.com/gcla/gowid/widgets/checkbox"
	"github.com/gcla/gowid/widgets/columns"
	"github.com/gcla/gowid/widgets/dialog"
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

var USE_DOAS = true

var txtProgramName = "CBSD-TUI"
var txtHelp = `- To navigate in jails list use 'Up' and 'Down' keys or mouse
- To open 'Actions' menu for the selected jail use 'F2' key
- To login into the selected jail (as root) use 'Enter' key or mouse double-click on jail name
- To switch to terminal from jails list use 'Tab' key
- To switch to jails list from terminal use 'Ctrl-Z'+'Tab' keys sequence
- Use bottom menu ('Fx' keys or mouse clicks) to start actions on the selected jail`

var doasProgram = "/usr/local/bin/doas"

var cbsdProgram = "/usr/local/bin/cbsd"
var cbsdCommandJailLogin = "jlogin"
var cbsdArgJailName = "jname"
var cbsdUserName = "cbsd"
var cbsdJlsHeader = []string{"NAME", "IP4_ADDRESS", "STATUS", "AUTOSTART", "VERSION"}

var pwProgram = "/usr/sbin/pw"

var cbsdUser *user.User = nil

//var cbsdDatabaseName = "file:/usr/local/jails/cbsd/var/db/local.sqlite?mode=ro"
var cbsdDatabaseName = "/var/db/local.sqlite"

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

//var cbsdActionsMenuText = []string{"Start/Stop", "Create Snapshot", "List Snapshots", "Clone", "Export", "Migrate", "Destroy", "Makeresolv", "Show Config"}
var cbsdActionsMenuText = []string{"Start/Stop", "Create Snapshot", "List Snapshots", "Edit", "Clone", "Export"}

//var cbsdBottomMenuText = []string{"[F1]Help ", "[F2]Actions Menu ", "[F4]Edit ", "[F5]Clone ", "[F6]Export ", "[F7]Create Snapshot ", "[F10]Exit ", "[F11]List Snapshots ", "[F12]Start/Stop"}
var cbsdBottomMenuText1 = []string{" 1", " 2", " 4", " 5", " 6", " 7", " 10", " 11", " 12"}
var cbsdBottomMenuText2 = []string{"Help ", "Actions Menu ", "Edit ", "Clone ", "Export ", "Create Snapshot ", "Exit ", "List Snapshots ", "Start/Stop"}
var cbsdActionsMenu map[string][]gowid.IWidget
var cbsdActionsDialog *dialog.Widget
var cbsdCloneJailDialog *dialog.Widget
var cbsdSnapshotJailDialog *dialog.Widget
var cbsdEditJailDialog *dialog.Widget
var cbsdListJails *list.Widget

var app *gowid.App
var menu2 *menu.Widget
var viewHolder *holder.Widget

type handler struct{}

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
		&gowid.ContainerWidget{IWidget: txtheadst, D: gowid.RenderFlow{}},
		&gowid.ContainerWidget{IWidget: txthelpst, D: gowid.RenderFlow{}},
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
		&gowid.ContainerWidget{IWidget: htxtst, D: gowid.RenderFlow{}},
		&gowid.ContainerWidget{IWidget: edsnapnamest, D: gowid.RenderFlow{}},
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
		&gowid.ContainerWidget{IWidget: htxtst, D: gowid.RenderFlow{}},
		&gowid.ContainerWidget{IWidget: ednewjnamest, D: gowid.RenderFlow{}},
		&gowid.ContainerWidget{IWidget: ednewhnamest, D: gowid.RenderFlow{}},
		&gowid.ContainerWidget{IWidget: ednewipst, D: gowid.RenderFlow{}},
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
}

func MakeEditJailDialog(jname string) *dialog.Widget {
	jail := GetJailByName(jname)
	if jail == nil {
		log.Errorf("Cannot find jail: " + jname)
		return nil
	}

	var edlines *pile.Widget = nil
	var ednewip *edit.Widget = nil
	var ednewipst *styled.Widget = nil

	htxt := text.New("Edit jail "+jname, text.Options{Align: gowid.HAlignMiddle{}})
	htxtst := styled.New(htxt, gowid.MakePaletteRef("magenta"))

	cbastart := checkbox.New(jail.GetAutoStartBool())
	labelastart := text.New("Autostart ")
	labelastartst := styled.New(labelastart, gowid.MakePaletteRef("green"))
	astartgrp := hpadding.New(
		columns.NewFixed(labelastartst, cbastart),
		gowid.HAlignLeft{},
		gowid.RenderFixed{},
	)

	ednewversion := edit.New(edit.Options{Caption: "Version: ", Text: jail.Ver})
	ednewversionst := styled.New(ednewversion, gowid.MakePaletteRef("green"))
	if !jail.IsRunning() {
		ednewip = edit.New(edit.Options{Caption: "IP address: ", Text: jail.Ip4_addr})
		ednewipst = styled.New(ednewip, gowid.MakePaletteRef("green"))
		edlines = pile.New([]gowid.IContainerWidget{
			&gowid.ContainerWidget{IWidget: htxtst, D: gowid.RenderFlow{}},
			&gowid.ContainerWidget{IWidget: astartgrp, D: gowid.RenderFlow{}},
			&gowid.ContainerWidget{IWidget: ednewversionst, D: gowid.RenderFlow{}},
			&gowid.ContainerWidget{IWidget: ednewipst, D: gowid.RenderFlow{}},
		})
	} else {
		edlines = pile.New([]gowid.IContainerWidget{
			&gowid.ContainerWidget{IWidget: htxtst, D: gowid.RenderFlow{}},
			&gowid.ContainerWidget{IWidget: astartgrp, D: gowid.RenderFlow{}},
			&gowid.ContainerWidget{IWidget: ednewversionst, D: gowid.RenderFlow{}},
		})
	}
	Ok := dialog.Button{
		Msg: "OK",
		Action: gowid.MakeWidgetCallback("execeditjail", gowid.WidgetChangedFunction(func(app gowid.IApp, w gowid.IWidget) {
			cbsdEditJailDialog.Close(app)
			if ednewip != nil {
				if (cbastart.IsChecked() != jail.GetAutoStartBool()) ||
					(ednewversion.Text() != jail.Ver) ||
					(ednewip.Text() != jail.Ip4_addr) {
					DoEditJail(jname, cbastart.IsChecked(), ednewversion.Text(), ednewip.Text())
				}
			} else {
				if (cbastart.IsChecked() != jail.GetAutoStartBool()) ||
					(ednewversion.Text() != jail.Ver) {
					DoEditJail(jname, cbastart.IsChecked(), ednewversion.Text(), "")
				}
			}
		})),
	}
	Cancel := dialog.Button{
		Msg: "Cancel",
	}
	editjaildialog := dialog.New(
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
	return editjaildialog
}

func MakeCbsdActionsMenu() map[string][]gowid.IWidget {
	actions := make(map[string][]gowid.IWidget, 0)
	for _, j := range cbsdJailsFromDb {
		actions[j.Jname] = MakeCbsdJailActionsMenu(j.Jname)
	}
	return actions
}

func MakeCbsdJailActionsMenu(jname string) []gowid.IWidget {
	menu := make([]gowid.IWidget, 0)
	for _, m := range cbsdActionsMenuText {
		mtext := text.New(m, text.Options{Align: gowid.HAlignLeft{}})
		mtexts := GetStyledWidget(mtext, "white")
		mbtn := button.New(mtexts, button.Options{Decoration: button.BareDecoration})
		mbtn.OnClick(gowid.WidgetCallback{Name: "cb_" + mtext.Content().String(), WidgetChangedFunction: func(app gowid.IApp, w gowid.IWidget) {
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
	//var cbsdActionsMenuText = []string{"Start/Stop", "Create Snapshot", "List Snapshots", "Edit", "Clone", "Export"}

	switch action {
	case "Start":
	case "Stop":
		StartStopJail(jname)
	case cbsdActionsMenuText[1]: // "Create Snapshot"
		SnapshotJail(jname)
	case cbsdActionsMenuText[2]: // "List Snapshots"
		ListSnapshotsJail(jname)
	case cbsdActionsMenuText[3]: // "Edit"
		EditJail(jname)
	case cbsdActionsMenuText[4]: // "Clone"
		CloneJail(jname)
	case cbsdActionsMenuText[5]: // "Export"
		ExportJail(jname)
	}
}

func RunMenuAction(action string) {
	log.Infof("Menu Action: " + action)

	switch action {
	// "[F1]Help ",            "[F2]Actions Menu ", "[F4]Edit ",            "[F5]Clone ",      "[F6]Export ",
	// "[F7]Create Snapshot ", "[F10]Exit ",        "[F11]List Snapshots ", "[F12]Start/Stop"
	case cbsdBottomMenuText2[0]: // Help
		helpdialog := CreateHelpDialog()
		helpdialog.Open(viewHolder, gowid.RenderWithRatio{R: 0.6}, app)
		return
	case cbsdBottomMenuText2[6]: // Exit
		app.Quit()
	}

	jname := GetSelectedJailName()
	log.Infof("JailName: " + jname)

	switch action {
	case cbsdBottomMenuText2[1]: // Actions Menu
		OpenJailActionsMenu(jname)
	case cbsdBottomMenuText2[2]: // Edit
		EditJail(jname)
	case cbsdBottomMenuText2[3]: // Clone
		CloneJail(jname)
	case cbsdBottomMenuText2[4]: // Export
		ExportJail(jname)
	case cbsdBottomMenuText2[5]: // Create Snapshot
		SnapshotJail(jname)
	case cbsdBottomMenuText2[7]: // List Snapshots
		ListSnapshotsJail(jname)
	case cbsdBottomMenuText2[8]: // Start/Stop
		StartStopJail(jname)
	}
}

func GetSelectedJailName() string {
	ifocus := cbsdListJails.Walker().Focus()
	jname := cbsdJailsFromDb[int(ifocus.(list.ListPos))-1].Jname
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

func DoEditJail(jname string, astart bool, version string, ip string) {
	jail := GetJailByName(jname)
	if jail == nil {
		log.Errorf("Cannot find jail: " + jname)
		return
	}
	if astart {
		jail.Astart = 1
	} else {
		jail.Astart = 0
	}
	jail.Ver = version
	if ip != "" {
		jail.Ip4_addr = ip
	}
	_, err := jail.PutJailToDb(GetCbsdDbConnString(true))
	if err != nil {
		panic(err)
	}
	UpdateJailLine(jail)
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
	var err error
	cbsdJailsFromDb, err = GetJailsFromDb(GetCbsdDbConnString(false))
	if err != nil {
		panic(err)
	}
	cbsdListLines = MakeJailsLines()
	cbsdActionsMenu = MakeCbsdActionsMenu()
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

func SnapshotJail(jname string) {
	cbsdSnapshotJailDialog = MakeSnapshotJailDialog(jname)
	cbsdSnapshotJailDialog.Open(viewHolder, gowid.RenderWithRatio{R: 0.3}, app)
}

func ListSnapshotsJail(jname string) {
	// cbsd jsnapshot mode=list jname=nim1
	var command string
	txtheader := "List jail snapshots...\n"
	args := make([]string, 0)
	if USE_DOAS {
		args = append(args, "cbsd")
	}
	args = append(args, "jsnapshot")
	args = append(args, "mode=list")
	args = append(args, "jname="+jname)
	if USE_DOAS {
		command = doasProgram
	} else {
		command = cbsdProgram
	}
	ExecCommand(txtheader, command, args)
}

func EditJail(jname string) {
	cbsdEditJailDialog = MakeEditJailDialog(jname)
	cbsdEditJailDialog.Open(viewHolder, gowid.RenderWithRatio{R: 0.3}, app)
}

func CloneJail(jname string) {
	cbsdCloneJailDialog = MakeCloneJailDialog(jname)
	cbsdCloneJailDialog.Open(viewHolder, gowid.RenderWithRatio{R: 0.3}, app)
}

func ExportJail(jname string) {
	// cbsd jexport jname=nim1
	var command string
	txtheader := "Exporting jail...\n"

	args := make([]string, 0)
	if USE_DOAS {
		args = append(args, "cbsd")
	}
	args = append(args, "jexport")
	args = append(args, "jname="+jname)

	if USE_DOAS {
		command = doasProgram
	} else {
		command = cbsdProgram
	}
	ExecCommand(txtheader, command, args)
}

func GetJailStatus(jname string) string {
	jail := GetJailByName(jname)
	return jail.GetStatusString()
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
		if str != jail.Jname {
			continue
		}
		style := GetJailStyle(jail.Status, jail.Astart)
		//	var cbsdJlsHeader = []string{"NAME", "IP4_ADDRESS", "STATUS", "AUTOSTART", "VERSION"}

		line[0] = GetMenuButton(jail)
		line[1] = GetStyledWidget(text.New(jail.Ip4_addr, text.Options{Align: gowid.HAlignMiddle{}}), style)
		line[2] = GetStyledWidget(text.New(jail.GetStatusString(), text.Options{Align: gowid.HAlignMiddle{}}), style)
		line[3] = GetStyledWidget(text.New(jail.GetAutoStartString(), text.Options{Align: gowid.HAlignMiddle{}}), style)
		line[4] = GetStyledWidget(text.New(jail.Ver, text.Options{Align: gowid.HAlignMiddle{}}), style)
	}
}

func GetMenuButton(jail *Jail) *keypress.Widget {
	btxt := text.New(jail.Jname, text.Options{Align: gowid.HAlignMiddle{}})
	style := GetJailStyle(jail.Status, jail.Astart)
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
	//MAXBUF := 10000000000
	txtout := text.New(title, text.Options{Align: gowid.HAlignLeft{}})
	outdlg := CreateActionsLogDialog(txtout)
	if cbsdActionsDialog != nil {
		if cbsdActionsDialog.IsOpen() {
			cbsdActionsDialog.Close(app)
		}
	}
	outdlg.Open(viewHolder, gowid.RenderWithRatio{R: 0.7}, app)
	outdlgwriter := text.Writer{Widget: txtout, IApp: app}
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
			logtxt := scanner.Text()
			logtxt = txtout.Content().String() + "\n" + logtxt
			outdlgwriter.Write([]byte(logtxt))
			app.RedrawTerminal()
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
	txtout := text.New(title, text.Options{Align: gowid.HAlignLeft{}})
	outdlg := CreateActionsLogDialog(txtout)
	if cbsdActionsDialog != nil {
		if cbsdActionsDialog.IsOpen() {
			cbsdActionsDialog.Close(app)
		}
	}
	outdlg.Open(viewHolder, gowid.RenderWithRatio{R: 0.7}, app)
	outdlgwriter := text.Writer{Widget: txtout, IApp: app}
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
					log.Errorf("jstart produced output is too long, it will be truncated\n")
					break
				}
				rbytes, err = file.Read(buf)
				if rbytes > 0 {
					logtxt := txtout.Content().String() + string(buf[:rbytes]) + "\n"
					outdlgwriter.Write([]byte(logtxt))
					app.RedrawTerminal()
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

func StartStopJail(jname string) {
	txtheader := ""
	//var cmd string
	//stdbuf := "/usr/bin/stdbuf"
	//shell := "/bin/sh"
	//logJstart := "/var/log/jstart.log"
	var args []string
	var command string

	jail := GetJailByName(jname)
	if jail == nil {
		log.Errorf("Cannot find jail: " + jname)
		return
	}
	if jail.IsRunning() {
		if cbsdJailConsoleActive == jname {
			SendTerminalCommand("exit")
			cbsdJailConsoleActive = ""
		}
		txtheader = "Stopping jail...\n"
		if USE_DOAS {
			args = append(args, cbsdProgram)
		}
		args = append(args, "jstop")
		args = append(args, "inter=1")
		args = append(args, "jname="+jname)
		if USE_DOAS {
			command = doasProgram
		} else {
			command = cbsdProgram
		}
		ExecCommand(txtheader, command, args)
	} else if jail.IsRunnable() {
		txtheader = "Starting jail...\n"
		/*
			if USE_DOAS {
				args = append(args, "cbsd")
			}
			args = append(args, "jstart")
			args = append(args, "inter=1")
			args = append(args, "quiet=1") // Temporary workaround for lock reading stdout when jail service use stderr
			args = append(args, "jname="+jail.Name)
		*/
		command = shellProgram
		script, err := CreateScriptStartJail(jname)
		if err != nil {
			log.Errorf("Cannot create jstart script: %w", err)
			if script != "" {
				os.Remove(script)
			}
			return
		}
		defer os.Remove(script)
		args = append(args, script)
		ExecShellCommand(txtheader, command, args, logJstart)
	}
	UpdateJailStatus(jail)
}

func CreateScriptStartJail(jname string) (string, error) {
	cmd := ""
	file, err := ioutil.TempFile("", "jail_start_")
	if err != nil {
		return "", err
	}
	file.WriteString("#!" + shellProgram + "\n")
	cmd += stdbufProgram
	cmd += " -o"
	//cmd += " 0 "
	cmd += " L "
	if USE_DOAS {
		cmd += doasProgram
		cmd += " "
		cmd += cbsdProgram
	} else {
		cmd += cbsdProgram
	}
	cmd += " jstart"
	cmd += " inter=1"
	cmd += " jname=" + jname
	cmd += " > "
	cmd += logJstart
	_, err = file.WriteString(cmd + "\n")
	if err != nil {
		return file.Name(), err
	}
	return file.Name(), nil
}

func GetJailByName(jname string) *Jail {
	var jail *Jail = nil
	for _, j := range cbsdJailsFromDb {
		if j.Jname == jname {
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

func GetJailsListHeader() []gowid.IWidget {
	header := make([]gowid.IWidget, 0)
	//found := false
	for _, h := range cbsdJlsHeader {
		htext := text.New(h, text.Options{Align: gowid.HAlignMiddle{}})
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

func OpenJailActionsMenu(jname string) {
	btn := cbsdActionsMenu[jname][0].(*button.Widget)
	txt := btn.SubWidget().(*styled.Widget).SubWidget().(*text.Widget)
	wr := text.Writer{Widget: txt, IApp: app}
	jail := GetJailByName(jname)
	if jail.IsRunning() {
		wr.Write([]byte("Stop"))
	} else {
		if jail.IsRunnable() {
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
func MakeGridLine(jail *Jail) []gowid.IWidget {
	style := "gray"
	line := make([]gowid.IWidget, 0)
	style = GetJailStyle(jail.Status, jail.Astart)
	//log.Infof("Got Style: " + fmt.Sprintf("%d %d %s", jail.Status, jail.Astart, style) + " for jail " + jail.Jname)
	line = append(line, GetMenuButton(jail))
	line = append(line, GetStyledWidget(text.New(jail.Ip4_addr, text.Options{Align: gowid.HAlignMiddle{}}), style))
	line = append(line, GetStyledWidget(text.New(jail.GetStatusString(), text.Options{Align: gowid.HAlignMiddle{}}), style))
	line = append(line, GetStyledWidget(text.New(jail.GetAutoStartString(), text.Options{Align: gowid.HAlignMiddle{}}), style))
	line = append(line, GetStyledWidget(text.New(jail.Ver, text.Options{Align: gowid.HAlignMiddle{}}), style))
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

/*
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
*/

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
		// "[F1]Help ",            "[F2]Actions Menu ", "[F4]Edit ",           "[F5]Clone ",      "[F6]Export ",
		// "[F7]Create Snapshot ", "[F10]Exit ",        "[F11]List Snapshots ", "[F12]Start/Stop"
		switch evk.Key() {
		case tcell.KeyCtrlC, tcell.KeyEsc, tcell.KeyF10:
			app.Quit()
		case tcell.KeyTab:
			if next, ok := cbsdWidgets.FindNextSelectable(gowid.Forwards, true); ok {
				cbsdWidgets.SetFocus(app, next)
			}
		case tcell.KeyF1:
			RunMenuAction(cbsdBottomMenuText2[0]) // Help
		case tcell.KeyF2:
			RunMenuAction(cbsdBottomMenuText2[1]) // Actions Menu
		case tcell.KeyF4:
			RunMenuAction(cbsdBottomMenuText2[2]) // Edit
		case tcell.KeyF5:
			RunMenuAction(cbsdBottomMenuText2[3]) // Clone
		case tcell.KeyF6:
			RunMenuAction(cbsdBottomMenuText2[4]) // Export
		case tcell.KeyF7:
			RunMenuAction(cbsdBottomMenuText2[5]) // Create Snapshot
		case tcell.KeyF11:
			RunMenuAction(cbsdBottomMenuText2[7]) // List Snapshots
		case tcell.KeyF12:
			RunMenuAction(cbsdBottomMenuText2[8]) // Start/Stop
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
		[]styled.AttributeRange{{Start: 0, End: -1, Styler: gowid.MakePaletteRef(cnofocus)}},
		[]styled.AttributeRange{{Start: 0, End: -1, Styler: gowid.MakePaletteRef(cfocus)}},
	)
}

func MakeBottomMenu() {
	cbsdBottomMenu = make([]gowid.IContainerWidget, 0)
	for i, m := range cbsdBottomMenuText2 {
		mtext1 := text.New(cbsdBottomMenuText1[i], text.Options{Align: gowid.HAlignLeft{}})
		mtext1st := styled.New(mtext1, gowid.MakePaletteRef("blackgreen"))
		mtext2 := text.New(m, text.Options{Align: gowid.HAlignLeft{}})
		mtext2st := styled.New(mtext2, gowid.MakePaletteRef("graydgreen"))
		mtextgrp := hpadding.New(
			columns.NewFixed(mtext1st, mtext2st),
			gowid.HAlignLeft{},
			gowid.RenderFixed{},
		)
		mbtn := button.New(mtextgrp, button.Options{Decoration: button.BareDecoration})
		mbtn.OnClick(gowid.WidgetCallback{Name: "cbb_" + mtext2.Content().String(), WidgetChangedFunction: func(app gowid.IApp, w gowid.IWidget) {
			app.Run(gowid.RunFunction(func(app gowid.IApp) {
				RunMenuAction(mtext2.Content().String())
			}))
		}})
		cbsdBottomMenu = append(cbsdBottomMenu, &gowid.ContainerWidget{IWidget: mbtn, D: gowid.RenderFixed{}})
	}
}

func main() {
	var err error

	f := RedirectLogger("/var/log/cbsd-tui.log")
	defer f.Close()

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

	cbsdListLines = MakeJailsLines()
	cbsdListHeader = GetJailsListHeader()
	cbsdActionsMenu = MakeCbsdActionsMenu()

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
