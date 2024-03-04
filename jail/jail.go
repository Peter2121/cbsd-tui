package jail

import (
	"bytes"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/gcla/gowid"
	"github.com/gcla/gowid/widgets/dialog"
	"github.com/gcla/gowid/widgets/edit"
	"github.com/gdamore/tcell"
	_ "github.com/mattn/go-sqlite3"

	//"github.com/prometheus/common/log"
	"github.com/quasilyte/gsignal"

	"host"
	"tui"
)

type Jail struct {
	Jname      string
	Ip4_addr   string
	Status     int
	Astart     int
	Ver        string
	params     map[string]string
	jtui       *tui.Tui
	evtUpdated gsignal.Event[string]
	evtRefresh gsignal.Event[any]
}

const (
	HELP       = "Help"
	START      = "Start"
	STOP       = "Stop"
	STARTSTOP  = "Start/Stop"
	CREATESNAP = "Create Snap."
	LISTSNAP   = "List Snap."
	DELSNAP    = "Destroy Snap."
	VIEW       = "View"
	EDIT       = "Edit"
	CLONE      = "Clone"
	EXPORT     = "Export"
	DESTROY    = "Destroy Jail"
	ACTIONS    = "Actions..."
	EXIT       = "Exit"
)

var strStatus = []string{"Off", "On", "Slave", "Unknown(3)", "Unknown(4)", "Unknown(5)"}
var strAutoStart = []string{"Off", "On"}
var strHeaderTitles = []string{"NAME", "IP4_ADDRESS", "STATUS", "AUTOSTART", "VERSION"}
var strActionsMenuItems = []string{STARTSTOP, CREATESNAP, DELSNAP, VIEW, EDIT, CLONE, EXPORT, DESTROY}

var strStartedActionsMenuItems = []string{STOP, CREATESNAP, DELSNAP, VIEW, EDIT, CLONE, EXPORT, DESTROY}
var strStoppedActionsMenuItems = []string{START, CREATESNAP, DELSNAP, VIEW, EDIT, CLONE, EXPORT, DESTROY}
var strNonRunnableActionsMenuItems = []string{"---", CREATESNAP, DELSNAP, VIEW, EDIT, CLONE, EXPORT, DESTROY}

var strBottomMenuText1 = []string{" 1", " 2", " 3", " 4", " 5", " 6", " 7", " 8", " 10", " 11", " 12"}
var strBottomMenuText2 = []string{HELP, ACTIONS, VIEW, EDIT, CLONE, EXPORT, CREATESNAP, DESTROY, EXIT, DELSNAP, STARTSTOP}
var keysBottomMenu = []tcell.Key{tcell.KeyF1, tcell.KeyF2, tcell.KeyF3, tcell.KeyF4, tcell.KeyF5, tcell.KeyF6, tcell.KeyF7, tcell.KeyF8, tcell.KeyF10, tcell.KeyF11, tcell.KeyF12}

var commandJailLogin string = "jlogin"
var commandJailStart string = "jstart"
var commandJailStop string = "jstop"
var commandJailSnap string = "jsnapshot"
var commandJailClone string = "jclone"
var commandJailExport string = "jexport"
var commandJailDestroy string = "jdestroy"
var commandJailStatus string = "jstatus"
var argJailName = "jname"
var argSnapName = "snapname"

func (jail *Jail) GetType() string {
	return "jail"
}

func (jail *Jail) GetSignalUpdated() *gsignal.Event[string] {
	return &jail.evtUpdated
}

func (jail *Jail) GetSignalRefresh() *gsignal.Event[any] {
	return &jail.evtRefresh
}

func (jail *Jail) SetTui(t *tui.Tui) {
	jail.jtui = t
}

func (jail *Jail) GetCommandHelp() string {
	return HELP
}

func (jail *Jail) GetCommandExit() string {
	return EXIT
}

func (jail *Jail) GetBottomMenuText1() []string {
	return strBottomMenuText1
}

func (jail *Jail) GetBottomMenuText2() []string {
	return strBottomMenuText2
}

func (jail *Jail) GetHeaderTitles() []string {
	return strHeaderTitles
}

func (jail *Jail) GetActionsMenuItems() []string {
	return strActionsMenuItems
}

func (jail *Jail) GetStartedActionsMenuItems() []string {
	return strStartedActionsMenuItems
}

func (jail *Jail) GetStoppedActionsMenuItems() []string {
	return strStoppedActionsMenuItems
}

func (jail *Jail) GetNonRunnableActionsMenuItems() []string {
	return strNonRunnableActionsMenuItems
}

func (jail *Jail) GetName() string {
	return jail.Jname
}

func (jail *Jail) GetStatus() int {
	return jail.Status
}

func (jail *Jail) GetCurrentStatus() int {
	var stdout, stderr bytes.Buffer
	retstatus := -1
	var command string = ""
	args := make([]string, 0)
	if host.USE_DOAS {
		args = append(args, host.CBSD_PROGRAM)
	}
	args = append(args, commandJailStatus)
	args = append(args, "invert=true")
	args = append(args, fmt.Sprintf("%s=%s", argJailName, jail.Jname))
	if host.USE_DOAS {
		command = host.DOAS_PROGRAM
	} else {
		command = host.CBSD_PROGRAM
	}
	cmd := exec.Command(command, args...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "NOCOLOR=1")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		//log.Errorf("cmd.Run() failed with %s\n", err)
		return retstatus
	}
	str_out := string(stdout.Bytes())
	str_out = strings.TrimSuffix(str_out, "\n")
	if str_out != "" {
		jid, err := strconv.Atoi(str_out)
		if err != nil {
			//log.Errorf("cbsd jstatus incorrect return %s\n", err)
			return retstatus
		}
		if jid > 0 {
			retstatus = 1
		} else if jid == 0 {
			retstatus = 0
		}
	}
	return retstatus
}

func (jail *Jail) GetAddr() string {
	return jail.Ip4_addr
}

func (jail *Jail) SetAddr(addr string) {
	jail.Ip4_addr = addr
}

func (jail *Jail) GetAstart() int {
	return jail.Astart
}

func (jail *Jail) SetAstart(as int) {
	jail.Astart = as
}

func (jail *Jail) GetVer() string {
	return jail.Ver
}

func (jail *Jail) SetVer(ver string) {
	jail.Ver = ver
}

func (jail *Jail) IsRunning() bool {
	if jail.Status == 1 {
		return true
	} else {
		return false
	}
}

func (jail *Jail) IsRunnable() bool {
	if jail.Status == 0 {
		return true
	} else {
		return false
	}
}

func (jail *Jail) GetStatusString() string {
	return strStatus[jail.Status]
}

func (jail *Jail) GetAutoStartString() string {
	return strAutoStart[jail.Astart]
}

func (jail *Jail) GetAutoStartBool() bool {
	if jail.Astart == 1 {
		return true
	} else {
		return false
	}
}

func (jail *Jail) GetAutoStartCode(astart string) int {
	for i, m := range strAutoStart {
		if m == astart {
			return i
		}
	}
	return -1
}

func (jail *Jail) GetStatusCode(status string) int {
	for i, m := range strStatus {
		if m == status {
			return i
		}
	}
	return -1
}

func (jail *Jail) GetParam(pn string) string {
	return jail.params[pn]
}

func (jail *Jail) SetParam(pn string, pv string) bool {
	jail.params[pn] = pv
	if v, found := jail.params[pn]; found {
		if v == pv {
			return true
		}
	}
	return false
}

func New() Jail {
	res := Jail{
		Jname:    "",
		Ip4_addr: "",
		Status:   0,
		Astart:   0,
		Ver:      "",
		params:   make(map[string]string),
	}
	return res
}

func NewJail(jname string, ip4_addr string, status int, astart int, ver string) Jail {
	res := Jail{
		Jname:    jname,
		Ip4_addr: ip4_addr,
		Status:   status,
		Astart:   astart,
		Ver:      ver,
		params:   make(map[string]string),
	}
	return res
}

func GetJailsFromDb(dbname string) ([]*Jail, error) {
	jails := make([]*Jail, 0)

	db, err := sql.Open("sqlite3", dbname)
	if err != nil {
		return jails, err
	}
	defer db.Close()

	jails_list_query := "SELECT jname,ip4_addr,status,astart,ver FROM jails WHERE emulator='jail'"
	rows, err := db.Query(jails_list_query)
	if err != nil {
		return jails, err
	}

	for rows.Next() {
		jail := New()
		err = rows.Scan(&jail.Jname, &jail.Ip4_addr, &jail.Status, &jail.Astart, &jail.Ver)
		if err != nil {
			return jails, err
		}
		if (jail.Status == 0) || (jail.Status == 1) {
			cur_status := jail.GetCurrentStatus()
			if cur_status >= 0 {
				jail.Status = cur_status
			}
		}
		jails = append(jails, &jail)
	}
	rows.Close()

	return jails, nil
}

func (jail *Jail) PutJailToDb(dbname string) (bool, error) {
	db, err := sql.Open("sqlite3", dbname)
	if err != nil {
		return false, err
	}
	defer db.Close()

	result, err := db.Exec("UPDATE jails SET ip4_addr=?, status=?, astart=?, ver=? WHERE jname=?", jail.Ip4_addr, jail.Status, jail.Astart, jail.Ver, jail.Jname)
	if err != nil {
		return false, err
	}
	rows_affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	if rows_affected > 0 {
		return true, nil
	} else {
		return false, nil
	}
}

func (jail *Jail) GetJailFromDb(dbname string, jname string) (bool, error) {
	db, err := sql.Open("sqlite3", dbname)
	if err != nil {
		return false, err
	}
	defer db.Close()

	row := db.QueryRow("SELECT jname,ip4_addr,status,astart,ver FROM jails WHERE jname = ?", jname)
	if err := row.Scan(&jail.Jname, &jail.Ip4_addr, &jail.Status, &jail.Astart, &jail.Ver); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	if (jail.Status == 0) || (jail.Status == 1) {
		cur_status := jail.GetCurrentStatus()
		if cur_status >= 0 {
			jail.Status = cur_status
		}
	}
	return true, nil
}

func (jail *Jail) GetJailFromDbFull(dbname string, jname string) (bool, error) {
	if jail.Jname != jname {
		result, err := jail.GetJailFromDb(dbname, jname)
		if err != nil {
			return false, err
		}
		if !result {
			return result, nil
		}
	}
	result := false
	db, err := sql.Open("sqlite3", dbname)
	if err != nil {
		return false, err
	}
	defer db.Close()

	rows, err := db.Query("SELECT * FROM jails WHERE jname = ?", jname)
	if err != nil {
		return false, err
	}

	cols, err := rows.Columns()
	if err != nil {
		return false, err
	}

	rawResult := make([][]byte, len(cols))
	//result := make([]string, len(cols))

	dest := make([]interface{}, len(cols))
	for i := range rawResult {
		dest[i] = &rawResult[i]
	}

	for rows.Next() {
		err = rows.Scan(dest...)
		if err != nil {
			return false, err
		}

		for i, raw := range rawResult {
			/*
			   if raw == nil {
			       result[i] = "\\N"
			   } else {
			       result[i] = string(raw)
			   }
			*/
			if raw != nil {
				jail.params[cols[i]] = string(raw)
				result = true
			}
		}
		//fmt.Printf("%#v\n", result)
	}
	return result, nil
}

func (jail *Jail) GetJailViewString() string {
	var strview string
	_, _ = jail.GetJailFromDbFull(host.GetCbsdDbConnString(false), jail.Jname)
	strview += "Name: " + jail.Jname + "\n"
	strview += "IP address: " + jail.Ip4_addr + "\n"
	strview += "Status: " + jail.GetStatusString() + "\n"
	strview += "Auto Start: " + jail.GetAutoStartString() + "\n"
	strview += "Version: " + jail.Ver + "\n\n"
	for key, value := range jail.params {
		strview += key + ": " + value + "\n"
	}
	strview += "\n"
	return strview
}

func (jail *Jail) UpdateJailFromDb(dbname string) (bool, error) {
	db, err := sql.Open("sqlite3", dbname)
	if err != nil {
		return false, err
	}
	defer db.Close()

	row := db.QueryRow("SELECT ip4_addr,status,astart,ver FROM jails WHERE jname = ?", jail.Jname)
	if err := row.Scan(&jail.Ip4_addr, &jail.Status, &jail.Astart, &jail.Ver); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	if (jail.Status == 0) || (jail.Status == 1) {
		cur_status := jail.GetCurrentStatus()
		if cur_status >= 0 {
			jail.Status = cur_status
		}
	}
	return true, nil
}

func (jail *Jail) Export() {
	// cbsd jexport jname=nim1
	var command string
	txtheader := "Exporting jail...\n"

	args := make([]string, 0)
	if host.USE_DOAS {
		args = append(args, host.CBSD_PROGRAM)
	}
	args = append(args, commandJailExport)
	args = append(args, fmt.Sprintf("%s=%s", argJailName, jail.Jname))

	if host.USE_DOAS {
		command = host.DOAS_PROGRAM
	} else {
		command = host.CBSD_PROGRAM
	}
	jail.jtui.ExecCommand(txtheader, command, args)
}

func (jail *Jail) Destroy() {
	// cbsd jdestroy jname=nim1
	var command string
	txtheader := "Destroying jail " + jail.Jname + "...\n"
	args := make([]string, 0)
	if host.USE_DOAS {
		args = append(args, host.CBSD_PROGRAM)
	}
	args = append(args, commandJailDestroy)
	args = append(args, fmt.Sprintf("%s=%s", argJailName, jail.Jname))
	if host.USE_DOAS {
		command = host.DOAS_PROGRAM
	} else {
		command = host.CBSD_PROGRAM
	}
	jail.jtui.ExecCommand(txtheader, command, args)
	jail.evtRefresh.Emit(nil)
}

func (jail *Jail) OpenDestroyDialog() {
	var cbsdDestroyJailDialog *dialog.Widget
	cbsdDestroyJailDialog = jail.jtui.MakeDialogForJail(
		jail.Jname,
		"Destroy jail "+jail.Jname,
		[]string{"Really destroy jail " + jail.Jname + "??"},
		nil, nil, nil, nil,
		func(jname string, boolparams []bool, strparams []string) {
			cbsdDestroyJailDialog.Close(jail.jtui.App)
			jail.Destroy()
		},
	)
	cbsdDestroyJailDialog.Open(jail.jtui.ViewHolder, gowid.RenderWithRatio{R: 0.3}, jail.jtui.App)
}

func (jail *Jail) Snapshot(snapname string) {
	// cbsd jsnapshot mode=create snapname=gettimeofday jname=nim1
	var command string
	txtheader := "Creating jail snapshot...\n"
	args := make([]string, 0)
	if host.USE_DOAS {
		args = append(args, host.CBSD_PROGRAM)
	}
	args = append(args, commandJailSnap)
	args = append(args, "mode=create")
	args = append(args, fmt.Sprintf("%s=%s", argSnapName, snapname))
	args = append(args, fmt.Sprintf("%s=%s", argJailName, jail.Jname))
	if host.USE_DOAS {
		command = host.DOAS_PROGRAM
	} else {
		command = host.CBSD_PROGRAM
	}
	jail.jtui.ExecCommand(txtheader, command, args)
}

func (jail *Jail) OpenSnapshotDialog() {
	var cbsdSnapshotJailDialog *dialog.Widget
	cbsdSnapshotJailDialog = jail.jtui.MakeDialogForJail(
		jail.Jname,
		"Snapshot jail "+jail.Jname,
		nil, nil, nil,
		[]string{"Snapshot name: "}, []string{"gettimeofday"},
		func(jname string, boolparams []bool, strparams []string) {
			cbsdSnapshotJailDialog.Close(jail.jtui.App)
			jail.Snapshot(strparams[0])
		},
	)
	cbsdSnapshotJailDialog.Open(jail.jtui.ViewHolder, gowid.RenderWithRatio{R: 0.3}, jail.jtui.App)
}

func (jail *Jail) Clone(jnewjname string, jnewhname string, newip string) {
	//log.Infof("Clone %s to %s (%s) IP %s", jname, jnewjname, jnewhname, newip)
	// cbsd jclone old=jail1 new=jail1clone host_hostname=jail1clone.domain.local ip4_addr=DHCP checkstate=0
	var command string
	txtheader := "Cloning jail...\n"

	args := make([]string, 0)
	if host.USE_DOAS {
		args = append(args, host.CBSD_PROGRAM)
	}
	args = append(args, commandJailClone)
	args = append(args, fmt.Sprintf("old=%s", jail.Jname))
	args = append(args, fmt.Sprintf("new=%s", jnewjname))
	args = append(args, fmt.Sprintf("host_hostname=%s", jnewhname))
	args = append(args, fmt.Sprintf("ip4_addr=%s", newip))
	args = append(args, "checkstate=0")

	if host.USE_DOAS {
		command = host.DOAS_PROGRAM
	} else {
		command = host.CBSD_PROGRAM
	}
	jail.jtui.ExecCommand(txtheader, command, args)
	jail.evtRefresh.Emit(nil)
}

func (jail *Jail) OpenCloneDialog() {
	var cbsdCloneJailDialog *dialog.Widget
	cbsdCloneJailDialog = jail.jtui.MakeDialogForJail(
		jail.Jname,
		"Clone jail "+jail.Jname,
		nil, nil, nil,
		[]string{"New jail name: ", "New host name: ", "New IP address: "},
		[]string{jail.Jname + "clone", jail.Jname, "DHCP"},
		func(jname string, boolparams []bool, strparams []string) {
			cbsdCloneJailDialog.Close(jail.jtui.App)
			jail.Clone(strparams[0], strparams[1], strparams[2])
		},
	)
	cbsdCloneJailDialog.Open(jail.jtui.ViewHolder, gowid.RenderWithRatio{R: 0.3}, jail.jtui.App)
}

func (jail *Jail) Edit(astart bool, version string, ip string) {
	if astart != jail.GetAutoStartBool() {
		if astart {
			jail.SetAstart(1)
		} else {
			jail.SetAstart(0)
		}
	}
	if version != jail.GetVer() {
		jail.SetVer(version)
	}
	if ip != "" {
		if ip != jail.GetAddr() {
			jail.SetAddr(ip)
		}
	}
	_, err := jail.PutJailToDb(host.GetCbsdDbConnString(true))
	if err != nil {
		panic(err)
	}
	jail.evtUpdated.Emit(jail.Jname)
}

func (jail *Jail) OpenEditDialog() {
	var cbsdEditJailDialog *dialog.Widget
	if !jail.IsRunning() {
		cbsdEditJailDialog = jail.jtui.MakeDialogForJail(
			jail.Jname,
			"Edit jail "+jail.Jname,
			nil,
			[]string{"Autostart "}, []bool{jail.GetAutoStartBool()},
			[]string{"Version: ", "IP address: "},
			[]string{jail.GetVer(), jail.GetAddr()},
			func(jname string, boolparams []bool, strparams []string) {
				cbsdEditJailDialog.Close(jail.jtui.App)
				jail.Edit(boolparams[0], strparams[0], strparams[1])
			},
		)
	} else {
		cbsdEditJailDialog = jail.jtui.MakeDialogForJail(
			jail.Jname,
			"Edit jail "+jail.Jname,
			nil,
			[]string{"Autostart "}, []bool{jail.GetAutoStartBool()},
			[]string{"Version: "},
			[]string{jail.GetVer()},
			func(jname string, boolparams []bool, strparams []string) {
				cbsdEditJailDialog.Close(jail.jtui.App)
				jail.Edit(boolparams[0], strparams[0], "")
			},
		)
	}
	cbsdEditJailDialog.Open(jail.jtui.ViewHolder, gowid.RenderWithRatio{R: 0.3}, jail.jtui.App)
}

func (jail *Jail) View() {
	viewspace := edit.New(edit.Options{ReadOnly: true})
	outdlg := jail.jtui.CreateActionsLogDialog(viewspace, jail.jtui.Console.Height())
	outdlg.Open(jail.jtui.ViewHolder, gowid.RenderWithRatio{R: 0.7}, jail.jtui.App)
	viewspace.SetText(jail.GetJailViewString(), jail.jtui.App)
	jail.jtui.App.RedrawTerminal()
}

func (jail *Jail) StartStop() {
	txtheader := ""
	var args []string
	var command string

	if jail.IsRunning() {
		if tui.CbsdJailConsoleActive == jail.Jname {
			jail.jtui.SendTerminalCommand("exit")
			tui.CbsdJailConsoleActive = ""
		}
		txtheader = "Stopping jail...\n"
		if host.USE_DOAS {
			args = append(args, host.CBSD_PROGRAM)
		}
		args = append(args, commandJailStop)
		args = append(args, "inter=1")
		args = append(args, fmt.Sprintf("%s=%s", argJailName, jail.Jname))
		if host.USE_DOAS {
			command = host.DOAS_PROGRAM
		} else {
			command = host.CBSD_PROGRAM
		}
		jail.jtui.ExecCommand(txtheader, command, args)
	} else if jail.IsRunnable() {
		txtheader = "Starting jail...\n"
		command = host.SHELL_PROGRAM
		script, err := jail.CreateScriptStartJail()
		if err != nil {
			host.LogError("Cannot create jstart script", err)
			if script != "" {
				os.Remove(script)
			}
			return
		}
		defer os.Remove(script)
		args = append(args, script)
		jail.jtui.ExecShellCommand(txtheader, command, args, host.LOGFILE_JSTART)
	}
	_, _ = jail.UpdateJailFromDb(host.GetCbsdDbConnString(false))
	jail.evtUpdated.Emit(jail.Jname)
}

func (jail *Jail) GetStartCommand() string {
	cmd := fmt.Sprintf("%s inter=1 %s=%s", commandJailStart, argJailName, jail.Jname)
	return cmd
}

func (jail *Jail) GetLoginCommand() string {
	cmd := fmt.Sprintf("%s %s=%s", commandJailLogin, argJailName, jail.Jname)
	return cmd
}

func (jail *Jail) CreateScriptStartJail() (string, error) {
	cmd := ""
	file, err := ioutil.TempFile("", "jail_start_")
	if err != nil {
		return "", err
	}
	file.WriteString("#!" + host.SHELL_PROGRAM + "\n")
	cmd += host.STDBUF_PROGRAM
	cmd += " -o"
	//cmd += " 0 "
	cmd += " L "
	if host.USE_DOAS {
		cmd += host.DOAS_PROGRAM
		cmd += " "
		cmd += host.CBSD_PROGRAM
	} else {
		cmd += host.CBSD_PROGRAM
	}
	cmd += " "
	cmd += jail.GetStartCommand()
	cmd += " > "
	cmd += host.LOGFILE_JSTART
	_, err = file.WriteString(cmd + "\n")
	if err != nil {
		return file.Name(), err
	}
	return file.Name(), nil
}

func (jail *Jail) OpenActionDialog() {
	var cbsdActionsDialog *dialog.Widget
	var MenuLines []string
	if jail.IsRunning() {
		MenuLines = jail.GetStartedActionsMenuItems()
	} else if jail.IsRunnable() {
		MenuLines = jail.GetStoppedActionsMenuItems()
	} else {
		MenuLines = jail.GetNonRunnableActionsMenuItems()
	}
	cbsdActionsDialog = jail.jtui.MakeActionDialogForJail(jail.Jname, "Actions for "+jail.Jname, MenuLines,
		[]func(jname string){
			func(jname string) {
				cbsdActionsDialog.Close(jail.jtui.App)
				jail.StartStop()
			},
			func(jname string) {
				cbsdActionsDialog.Close(jail.jtui.App)
				jail.OpenSnapshotDialog()
			},
			func(jname string) {
				cbsdActionsDialog.Close(jail.jtui.App)
				jail.OpenSnapActionsDialog()
			},
			func(jname string) {
				cbsdActionsDialog.Close(jail.jtui.App)
				jail.View()
			},
			func(jname string) {
				cbsdActionsDialog.Close(jail.jtui.App)
				jail.OpenEditDialog()
			},
			func(jname string) {
				cbsdActionsDialog.Close(jail.jtui.App)
				jail.OpenCloneDialog()
			},
			func(jname string) {
				cbsdActionsDialog.Close(jail.jtui.App)
				jail.Export()
			},
			func(jname string) {
				cbsdActionsDialog.Close(jail.jtui.App)
				jail.OpenDestroyDialog()
			},
			func(jname string) {},
		},
	)
	cbsdActionsDialog.Open(jail.jtui.ViewHolder, gowid.RenderWithRatio{R: 0.3}, jail.jtui.App)
}

func (jail *Jail) ExecuteActionOnCommand(command string) {
	switch command {
	case ACTIONS: // Actions Menu
		jail.OpenActionDialog()
	case VIEW: // View
		jail.View()
	case EDIT: // Edit
		jail.OpenEditDialog()
	case CLONE: // Clone
		jail.OpenCloneDialog()
	case EXPORT: // Export
		jail.Export()
	case CREATESNAP: // Create Snapshot
		jail.OpenSnapshotDialog()
	case DESTROY: // Destroy
		jail.OpenDestroyDialog()
	case DELSNAP: // Destroy Snapshots
		jail.OpenSnapActionsDialog()
	case STARTSTOP: // Start/Stop
		jail.StartStop()
	}
}

func (jail *Jail) ExecuteActionOnKey(tkey int16) {
	for i, k := range keysBottomMenu {
		if int16(k) == tkey {
			jail.ExecuteActionOnCommand(strBottomMenuText2[i])
		}
	}
}

func (jail *Jail) GetSnapshots() [][2]string {
	var snap = [2]string{"", ""}
	retsnap := make([][2]string, 0)
	var stdout, stderr bytes.Buffer
	var cmd *exec.Cmd = nil

	// cbsd jsnapshot jname=jinja1 mode=list header=0 display=snapname,creation
	args := make([]string, 0)
	if host.USE_DOAS {
		args = append(args, host.CBSD_PROGRAM)
	}
	args = append(args, commandJailSnap)
	args = append(args, "mode=list")
	args = append(args, "header=0")
	args = append(args, "display=snapname,creation")
	args = append(args, fmt.Sprintf("%s=%s", argJailName, jail.Jname))
	if host.USE_DOAS {
		cmd = exec.Command(host.DOAS_PROGRAM, args...)
	} else {
		cmd = exec.Command(host.CBSD_PROGRAM, args...)
	}
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "NOCOLOR=1")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		host.LogError("cmd.Run() failed", err)
		return retsnap
	}
	str_out := string(stdout.Bytes())
	str_snaps := strings.Split(str_out, "\n")
	for _, s := range str_snaps {
		fields := strings.Fields(s)
		if len(fields) < 2 {
			continue
		}
		snap[0] = fields[0]
		snap[1] = fields[1]
		retsnap = append(retsnap, snap)
	}
	return retsnap
}

func (jail *Jail) OpenSnapActionsDialog() {
	var cbsdSnapActionsDialog *dialog.Widget
	MakeWidgetChangedFunction := func(snapname string) func(jname string) {
		return func(jname string) {
			cbsdSnapActionsDialog.Close(jail.jtui.App)
			jail.OpenDestroySnapshotDialog(snapname)
		}
	}
	snaps := jail.GetSnapshots()
	var menulines []string
	var cbfunc []func(jname string)
	for _, s := range snaps {
		menulines = append(menulines, s[0]+" ("+s[1]+")")
		cbfunc = append(cbfunc, MakeWidgetChangedFunction(s[0]))
	}
	cbsdSnapActionsDialog = jail.jtui.MakeActionDialogForJail(jail.Jname, "Snapshots for "+jail.Jname, menulines, cbfunc)
	cbsdSnapActionsDialog.Open(jail.jtui.ViewHolder, gowid.RenderWithRatio{R: 0.3}, jail.jtui.App)
}

func (jail *Jail) DestroySnapshot(snapname string) {
	// cbsd jsnapshot mode=destroy jname=nim1 snapname=20220319193339
	var command string
	txtheader := "Destroy jail snapshot...\n"
	args := make([]string, 0)
	if host.USE_DOAS {
		args = append(args, host.CBSD_PROGRAM)
	}
	args = append(args, commandJailSnap)
	args = append(args, "mode=destroy")
	args = append(args, fmt.Sprintf("%s=%s", argJailName, jail.Jname))
	args = append(args, fmt.Sprintf("%s=%s", argSnapName, snapname))
	if host.USE_DOAS {
		command = host.DOAS_PROGRAM
	} else {
		command = host.CBSD_PROGRAM
	}
	if jail.jtui != nil {
		jail.jtui.ExecCommand(txtheader, command, args)
	}
}

func (jail *Jail) OpenDestroySnapshotDialog(snapname string) {
	var cbsdDestroySnapDialog *dialog.Widget
	cbsdDestroySnapDialog = jail.jtui.MakeDialogForJail(
		jail.Jname,
		"Destroy snapshot "+snapname+"\nof jail "+jail.Jname,
		[]string{"Really destroy snapshot " + snapname + "\nof jail " + jail.Jname + "??"},
		nil, nil, nil, nil,
		func(jname string, boolparams []bool, strparams []string) {
			cbsdDestroySnapDialog.Close(jail.jtui.App)
			jail.DestroySnapshot(snapname)
		},
	)
	cbsdDestroySnapDialog.Open(jail.jtui.ViewHolder, gowid.RenderWithRatio{R: 0.3}, jail.jtui.App)
}

func (jail *Jail) GetAllParams() []string {
	params := make([]string, 4)
	params[0] = jail.GetAddr()
	params[1] = jail.GetStatusString()
	params[2] = jail.GetAutoStartString()
	params[3] = jail.GetVer()
	return params
}
