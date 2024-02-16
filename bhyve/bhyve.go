package bhyve

import (
	"bytes"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/gcla/gowid"
	"github.com/gcla/gowid/widgets/dialog"
	"github.com/gcla/gowid/widgets/edit"
	"github.com/gcla/gowid/widgets/holder"
	"github.com/gdamore/tcell"
	_ "github.com/mattn/go-sqlite3"

	"host"
	"tui"
)

type BhyveVm struct {
	Bname      string
	Ip4_addr   string
	Status     int
	Astart     int
	OsType     string
	VncConsole string
	params     map[string]string
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
	DESTROY    = "Destroy"
	ACTIONS    = "Actions..."
	EXIT       = "Exit"
)

var strStatus = []string{"Off", "On", "Slave", "Unknown(3)", "Unknown(4)", "Unknown(5)"}
var strAutoStart = []string{"Off", "On"}
var strHeaderTitles = []string{"NAME", "IP4_ADDRESS", "STATUS", "AUTOSTART", "OS_TYPE", "VNC_CONSOLE"}
var strActionsMenuItems = []string{STARTSTOP, CREATESNAP, DELSNAP, VIEW, EDIT, CLONE, EXPORT, DESTROY}

var strStartedActionsMenuItems = []string{STOP, CREATESNAP, DELSNAP, VIEW, EDIT, CLONE, EXPORT, DESTROY}
var strStoppedActionsMenuItems = []string{START, CREATESNAP, DELSNAP, VIEW, EDIT, CLONE, EXPORT, DESTROY}
var strNonRunnableActionsMenuItems = []string{"---", CREATESNAP, DELSNAP, VIEW, EDIT, CLONE, EXPORT, DESTROY}

var strBottomMenuText1 = []string{" 1", " 2", " 3", " 4", " 5", " 6", " 7", " 8", " 10", " 11", " 12"}
var strBottomMenuText2 = []string{HELP, ACTIONS, VIEW, EDIT, CLONE, EXPORT, CREATESNAP, DESTROY, EXIT, DELSNAP, STARTSTOP}
var keysBottomMenu = []tcell.Key{tcell.KeyF1, tcell.KeyF2, tcell.KeyF3, tcell.KeyF4, tcell.KeyF5, tcell.KeyF6, tcell.KeyF7, tcell.KeyF8, tcell.KeyF10, tcell.KeyF11, tcell.KeyF12}

var commandJailLogin string = "blogin"
var commandJailStart string = "bstart"
var commandJailStop string = "bstop"
var commandJailSnap string = "jsnapshot"
var argJailName = "jname"
var argSnapName = "snapname"

func (jail *BhyveVm) GetCommandHelp() string {
	return HELP
}

func (jail *BhyveVm) GetCommandExit() string {
	return EXIT
}

func (jail *BhyveVm) GetBottomMenuText1() []string {
	return strBottomMenuText1
}

func (jail *BhyveVm) GetBottomMenuText2() []string {
	return strBottomMenuText2
}

func (jail *BhyveVm) GetHeaderTitles() []string {
	return strHeaderTitles
}

func (jail *BhyveVm) GetActionsMenuItems() []string {
	return strActionsMenuItems
}

func (jail *BhyveVm) GetStartedActionsMenuItems() []string {
	return strStartedActionsMenuItems
}

func (jail *BhyveVm) GetStoppedActionsMenuItems() []string {
	return strStoppedActionsMenuItems
}

func (jail *BhyveVm) GetNonRunnableActionsMenuItems() []string {
	return strNonRunnableActionsMenuItems
}

func (jail *BhyveVm) GetName() string {
	return jail.Bname
}

func (jail *BhyveVm) GetStatus() int {
	return jail.Status
}

func (jail *BhyveVm) GetAddr() string {
	return jail.Ip4_addr
}

func (jail *BhyveVm) SetAddr(addr string) {
	jail.Ip4_addr = addr
}

func (jail *BhyveVm) GetAstart() int {
	return jail.Astart
}

func (jail *BhyveVm) SetAstart(as int) {
	jail.Astart = as
}

func (jail *BhyveVm) GetVer() string {
	return "N/A"
}

func (jail *BhyveVm) SetVer(ver string) {
}

func (jail *BhyveVm) IsRunning() bool {
	if jail.Status == 1 {
		return true
	} else {
		return false
	}
}

func (jail *BhyveVm) IsRunnable() bool {
	if jail.Status == 0 {
		return true
	} else {
		return false
	}
}

func (jail *BhyveVm) GetStatusString() string {
	return strStatus[jail.Status]
}

func (jail *BhyveVm) GetAutoStartString() string {
	return strAutoStart[jail.Astart]
}

func (jail *BhyveVm) GetAutoStartBool() bool {
	if jail.Astart == 1 {
		return true
	} else {
		return false
	}
}

func (jail *BhyveVm) GetAutoStartCode(astart string) int {
	for i, m := range strAutoStart {
		if m == astart {
			return i
		}
	}
	return -1
}

func (jail *BhyveVm) GetStatusCode(status string) int {
	for i, m := range strStatus {
		if m == status {
			return i
		}
	}
	return -1
}

func (jail *BhyveVm) GetParam(pn string) string {
	return jail.params[pn]
}

func (jail *BhyveVm) SetParam(pn string, pv string) bool {
	jail.params[pn] = pv
	if v, found := jail.params[pn]; found {
		if v == pv {
			return true
		}
	}
	return false
}

func New() *BhyveVm {
	res := &BhyveVm{
		Bname:      "",
		Ip4_addr:   "",
		Status:     0,
		Astart:     0,
		OsType:     "",
		VncConsole: "",
		params:     make(map[string]string),
	}
	return res
}

func NewBhyveVm(jname string, ip4_addr string, status int, astart int, os_type string, vnc_console string) *BhyveVm {
	res := &BhyveVm{
		Bname:      jname,
		Ip4_addr:   ip4_addr,
		Status:     status,
		Astart:     astart,
		OsType:     os_type,
		VncConsole: vnc_console,
		params:     make(map[string]string),
	}
	return res
}

func GetJailsFromDb(dbname string) ([]*BhyveVm, error) {
	jails := make([]*BhyveVm, 0)
	var vnc_port int = 0
	var vnc_ip_addr string = ""

	db, err := sql.Open("sqlite3", dbname)
	if err != nil {
		return jails, err
	}
	defer db.Close()

	//jails_list_query := "SELECT jname,ip4_addr,status,astart FROM jails WHERE emulator='bhyve'"
	jails_list_query := "SELECT jails.jname,jails.ip4_addr,jails.status,jails.astart,bhyve.vm_os_type,bhyve.vm_vnc_port,bhyve.bhyve_vnc_tcp_bind FROM jails LEFT JOIN bhyve ON jails.jname=bhyve.jname WHERE jails.emulator='bhyve'"
	rows, err := db.Query(jails_list_query)
	if err != nil {
		return jails, err
	}

	for rows.Next() {
		jail := New()
		err = rows.Scan(&jail.Bname, &jail.Ip4_addr, &jail.Status, &jail.Astart, &jail.OsType, &vnc_port, &vnc_ip_addr)
		if err != nil {
			return jails, err
		}
		jail.VncConsole = fmt.Sprintf("%s:%d", vnc_ip_addr, vnc_port)
		jails = append(jails, jail)
	}
	rows.Close()

	return jails, nil
}

func (jail *BhyveVm) PutJailToDb(dbname string) (bool, error) {
	return false, nil
	/*
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
	*/
}

func (jail *BhyveVm) GetJailFromDb(dbname string, jname string) (bool, error) {
	db, err := sql.Open("sqlite3", dbname)
	if err != nil {
		return false, err
	}
	defer db.Close()

	var vnc_port int = 0
	var vnc_ip_addr string = ""

	//row := db.QueryRow("SELECT jname,ip4_addr,status,astart,ver FROM jails WHERE jname = ?", jname)
	row := db.QueryRow("SELECT jails.jname,jails.ip4_addr,jails.status,jails.astart,bhyve.vm_os_type,bhyve.vm_vnc_port,bhyve.bhyve_vnc_tcp_bind FROM jails LEFT JOIN bhyve ON jails.jname=bhyve.jname WHERE jails.emulator='bhyve' AND jails.jname = ?", jname)

	if err := row.Scan(&jail.Bname, &jail.Ip4_addr, &jail.Status, &jail.Astart, &jail.OsType, &vnc_port, &vnc_ip_addr); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	jail.VncConsole = fmt.Sprintf("%s:%d", vnc_ip_addr, vnc_port)
	return true, nil
}

func (jail *BhyveVm) GetJailFromDbFull(dbname string, jname string) (bool, error) {
	return false, nil
	/*
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
				   //if raw == nil {
				   //    result[i] = "\\N"
				   //} else {
				   //    result[i] = string(raw)
				   //}
				if raw != nil {
					jail.params[cols[i]] = string(raw)
					result = true
				}
			}
			//fmt.Printf("%#v\n", result)
		}
		return result, nil
	*/
}

func (jail *BhyveVm) GetJailViewString() string {
	var strview string
	strview += "Name: " + jail.Bname + "\n"
	strview += "IP address: " + jail.Ip4_addr + "\n"
	strview += "Status: " + jail.GetStatusString() + "\n"
	strview += "Auto Start: " + jail.GetAutoStartString() + "\n"
	strview += "OS Type: " + jail.OsType + "\n"
	strview += "VNC Console: " + jail.VncConsole + "\n\n"
	for key, value := range jail.params {
		strview += key + ": " + value + "\n"
	}
	strview += "\n"
	return strview
}

func (jail *BhyveVm) UpdateJailFromDb(dbname string) (bool, error) {
	db, err := sql.Open("sqlite3", dbname)
	if err != nil {
		return false, err
	}
	defer db.Close()

	var vnc_port int = 0
	var vnc_ip_addr string = ""

	row := db.QueryRow("SELECT jails.jname,jails.ip4_addr,jails.status,jails.astart,bhyve.vm_os_type,bhyve.vm_vnc_port,bhyve.bhyve_vnc_tcp_bind FROM jails LEFT JOIN bhyve ON jails.jname=bhyve.jname WHERE jails.emulator='bhyve' AND jails.jname = ?", jail.Bname)

	if err := row.Scan(&jail.Bname, &jail.Ip4_addr, &jail.Status, &jail.Astart, &jail.OsType, &vnc_port, &vnc_ip_addr); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	jail.VncConsole = fmt.Sprintf("%s:%d", vnc_ip_addr, vnc_port)
	return true, nil
}

func (jail *Jail) Export(*holder.Widget, *gowid.App) {
	// cbsd jexport jname=nim1
	var command string
	txtheader := "Exporting jail...\n"

	args := make([]string, 0)
	if doas {
		args = append(args, cbsdProgram)
	}
	args = append(args, "jexport")
	args = append(args, "jname="+jail.Jname)

	if doas {
		command = doasProgram
	} else {
		command = cbsdProgram
	}
	ExecCommand(txtheader, command, args)
}

func (jail *Jail) Destroy() {
	// cbsd jdestroy jname=nim1
	var command string
	txtheader := "Destroying jail " + jail.Jname + "...\n"
	args := make([]string, 0)
	if doas {
		args = append(args, cbsdProgram)
	}
	args = append(args, "jdestroy")
	args = append(args, "jname="+jail.Jname)
	if doas {
		command = doasProgram
	} else {
		command = cbsdProgram
	}
	ExecCommand(txtheader, command, args)
	RefreshJailList()
}

func (jail *Jail) OpenDestroyDialog(viewHolder *holder.Widget, app *gowid.App) {
	var cbsdDestroyJailDialog *dialog.Widget
	cbsdDestroyJailDialog = tui.MakeDialogForJail(
		jail.Jname,
		"Destroy jail "+jail.Jname,
		[]string{"Really destroy jail " + jail.Jname + "??"},
		nil, nil, nil, nil,
		func(jname string, boolparams []bool, strparams []string) {
			cbsdDestroyJailDialog.Close(app)
			jail.Destroy()
		},
	)
	cbsdDestroyJailDialog.Open(viewHolder, gowid.RenderWithRatio{R: 0.3}, app)
}

func (jail *Jail) Snapshot(snapname string) {
	// cbsd jsnapshot mode=create snapname=gettimeofday jname=nim1
	var command string
	txtheader := "Creating jail snapshot...\n"
	args := make([]string, 0)
	if doas {
		args = append(args, cbsdProgram)
	}
	args = append(args, "jsnapshot")
	args = append(args, "mode=create")
	args = append(args, "snapname="+snapname)
	args = append(args, "jname="+jail.Jname)
	if doas {
		command = doasProgram
	} else {
		command = cbsdProgram
	}
	ExecCommand(txtheader, command, args)
}

func (jail *Jail) OpenSnapshotDialog(viewHolder *holder.Widget, app *gowid.App) {
	var cbsdSnapshotJailDialog *dialog.Widget
	cbsdSnapshotJailDialog = tui.MakeDialogForJail(
		jail.Jname,
		"Snapshot jail "+jail.Jname,
		nil, nil, nil,
		[]string{"Snapshot name: "}, []string{"gettimeofday"},
		func(jname string, boolparams []bool, strparams []string) {
			cbsdSnapshotJailDialog.Close(app)
			jail.Snapshot(strparams[0])
		},
	)
	cbsdSnapshotJailDialog.Open(viewHolder, gowid.RenderWithRatio{R: 0.3}, app)
}

func (jail *Jail) Clone(jnewjname string, jnewhname string, newip string) {
	//log.Infof("Clone %s to %s (%s) IP %s", jname, jnewjname, jnewhname, newip)
	// cbsd jclone old=jail1 new=jail1clone host_hostname=jail1clone.domain.local ip4_addr=DHCP checkstate=0
	var command string
	txtheader := "Cloning jail...\n"

	args := make([]string, 0)
	if doas {
		args = append(args, cbsdProgram)
	}
	args = append(args, "jclone")
	args = append(args, "old="+jail.Jname)
	args = append(args, "new="+jnewjname)
	args = append(args, "host_hostname="+jnewhname)
	args = append(args, "ip4_addr="+newip)
	args = append(args, "checkstate=0")

	if doas {
		command = doasProgram
	} else {
		command = cbsdProgram
	}
	ExecCommand(txtheader, command, args)
	RefreshJailList()
}

func (jail *Jail) OpenCloneDialog(viewHolder *holder.Widget, app *gowid.App) {
	var cbsdCloneJailDialog *dialog.Widget
	cbsdCloneJailDialog = tui.MakeDialogForJail(
		jail.Jname,
		"Clone jail "+jail.Jname,
		nil, nil, nil,
		[]string{"New jail name: ", "New host name: ", "New IP address: "},
		[]string{jail.Jname + "clone", jail.Jname, "DHCP"},
		func(jname string, boolparams []bool, strparams []string) {
			cbsdCloneJailDialog.Close(app)
			jail.Clone(strparams[0], strparams[1], strparams[2])
		},
	)
	cbsdCloneJailDialog.Open(viewHolder, gowid.RenderWithRatio{R: 0.3}, app)
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
	_, err := jail.PutJailToDb(GetCbsdDbConnString(true))
	if err != nil {
		panic(err)
	}
	UpdateJailLine(jail)
}

func (jail *Jail) OpenEditDialog(viewHolder *holder.Widget, app *gowid.App) {
	var cbsdEditJailDialog *dialog.Widget
	if !jail.IsRunning() {
		cbsdEditJailDialog = tui.MakeDialogForJail(
			jail.Jname,
			"Edit jail "+jail.Jname,
			nil,
			[]string{"Autostart "}, []bool{jail.GetAutoStartBool()},
			[]string{"Version: ", "IP address: "},
			[]string{jail.GetVer(), jail.GetAddr()},
			func(jname string, boolparams []bool, strparams []string) {
				cbsdEditJailDialog.Close(app)
				jail.Edit(boolparams[0], strparams[0], strparams[1])
			},
		)
	} else {
		cbsdEditJailDialog = tui.MakeDialogForJail(
			jail.Jname,
			"Edit jail "+jail.Jname,
			nil,
			[]string{"Autostart "}, []bool{jail.GetAutoStartBool()},
			[]string{"Version: "},
			[]string{jail.GetVer()},
			func(jname string, boolparams []bool, strparams []string) {
				cbsdEditJailDialog.Close(app)
				jail.Edit(boolparams[0], strparams[0], "")
			},
		)
	}
	cbsdEditJailDialog.Open(viewHolder, gowid.RenderWithRatio{R: 0.3}, app)
}

func (jail *Jail) View(viewHolder *holder.Widget, app *gowid.App) {
	viewspace := edit.New(edit.Options{ReadOnly: true})
	outdlg := CreateActionsLogDialog(viewspace)
	outdlg.Open(viewHolder, gowid.RenderWithRatio{R: 0.7}, app)
	viewspace.SetText(jail.GetJailViewString(), app)
	app.RedrawTerminal()
}

func (jail *Jail) StartStop(viewHolder *holder.Widget, app *gowid.App) {
	txtheader := ""
	var args []string
	var command string

	if jail.IsRunning() {
		if cbsdJailConsoleActive == jail.Jname { // TODO: don't use cbsdJailConsoleActive directly
			SendTerminalCommand("exit")
			cbsdJailConsoleActive = ""
		}
		txtheader = "Stopping jail...\n"
		if doas {
			args = append(args, cbsdProgram)
		}
		args = append(args, commandJailStop)
		args = append(args, "inter=1")
		args = append(args, "jname="+jail.Jname)
		if doas {
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
		script, err := jail.CreateScriptStartJail()
		if err != nil {
			LogError("Cannot create jstart script", err)
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

func (jail *BhyveVm) GetStartCommand() string {
	cmd := fmt.Sprintf("%s inter=1 %s=%s", commandJailStart, argJailName, jail.Bname)
	return cmd
}

func (jail *BhyveVm) GetLoginCommand() string {
	cmd := fmt.Sprintf("%s %s=%s", commandJailLogin, argJailName, jail.Bname)
	return cmd
}

func (jail *Jail) CreateScriptStartJail() (string, error) {
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
	if doas {
		cmd += doasProgram
		cmd += " "
		cmd += cbsdProgram
	} else {
		cmd += cbsdProgram
	}
	cmd += " "
	cmd += jail.GetStartCommand()
	cmd += " > "
	cmd += logJstart
	_, err = file.WriteString(cmd + "\n")
	if err != nil {
		return file.Name(), err
	}
	return file.Name(), nil
}

func (jail *Jail) OpenActionDialog(viewHolder *holder.Widget, app *gowid.App) {
	var cbsdActionsDialog *dialog.Widget
	var MenuLines []string
	if jail.IsRunning() {
		MenuLines = jail.GetStartedActionsMenuItems()
	} else if jail.IsRunnable() {
		MenuLines = jail.GetStoppedActionsMenuItems()
	} else {
		MenuLines = jail.GetNonRunnableActionsMenuItems()
	}
	cbsdActionsDialog = MakeActionDialogForJail(jail.Jname, "Actions for "+jail.Jname, MenuLines,
		[]func(jname string){
			func(jname string) {
				cbsdActionsDialog.Close(app)
				jail.StartStop(viewHolder, app)
			},
			func(jname string) {
				cbsdActionsDialog.Close(app)
				jail.OpenSnapshotDialog(viewHolder, app)
			},
			func(jname string) {
				cbsdActionsDialog.Close(app)
				jail.OpenSnapActionsDialog(viewHolder, app)
			},
			func(jname string) {
				cbsdActionsDialog.Close(app)
				jail.View(viewHolder, app)
			},
			func(jname string) {
				cbsdActionsDialog.Close(app)
				jail.OpenEditDialog(viewHolder, app)
			},
			func(jname string) {
				cbsdActionsDialog.Close(app)
				jail.OpenCloneDialog(viewHolder, app)
			},
			func(jname string) {
				cbsdActionsDialog.Close(app)
				jail.Export(viewHolder, app)
			},
			func(jname string) {
				cbsdActionsDialog.Close(app)
				jail.OpenDestroyDialog(viewHolder, app)
			},
			func(jname string) {},
		},
	)
	cbsdActionsDialog.Open(viewHolder, gowid.RenderWithRatio{R: 0.3}, app)
}

func (jail *Jail) ExecuteActionOnCommand(command string, vh *holder.Widget, app *gowid.App) {
	switch command {
	case ACTIONS: // Actions Menu
		jail.OpenActionDialog(vh, app)
	case VIEW: // View
		jail.View(vh, app)
	case EDIT: // Edit
		jail.OpenEditDialog(vh, app)
	case CLONE: // Clone
		jail.OpenCloneDialog(vh, app)
	case EXPORT: // Export
		jail.Export(vh, app)
	case CREATESNAP: // Create Snapshot
		jail.OpenSnapshotDialog(vh, app)
	case DESTROY: // Destroy
		jail.OpenDestroyDialog(vh, app)
	case DELSNAP: // Destroy Snapshots
		jail.OpenSnapActionsDialog(vh, app)
	case STARTSTOP: // Start/Stop
		jail.StartStop(vh, app)
	}
}

func (jail *Jail) ExecuteActionOnKey(tkey int16, vh *holder.Widget, app *gowid.App) {
	for i, k := range keysBottomMenu {
		if int16(k) == tkey {
			jail.ExecuteActionOnCommand(strBottomMenuText2[i], vh, app)
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
	if doas {
		args = append(args, cbsdProgram)
	}
	args = append(args, "jsnapshot")
	args = append(args, "jname="+jail.Jname)
	args = append(args, "mode=list")
	args = append(args, "header=0")
	args = append(args, "display=snapname,creation")
	if doas {
		cmd = exec.Command(doasProgram, args...)
	} else {
		cmd = exec.Command(cbsdProgram, args...)
	}
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "NOCOLOR=1")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		LogError("cmd.Run() failed", err)
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

func (jail *Jail) OpenSnapActionsDialog(viewHolder *holder.Widget, app *gowid.App) {
	var cbsdSnapActionsDialog *dialog.Widget
	MakeWidgetChangedFunction := func(snapname string) func(jname string) {
		return func(jname string) {
			cbsdSnapActionsDialog.Close(app)
			jail.OpenDestroySnapshotDialog(snapname, viewHolder, app)
		}
	}
	snaps := jail.GetSnapshots()
	var menulines []string
	var cbfunc []func(jname string)
	for _, s := range snaps {
		menulines = append(menulines, s[0]+" ("+s[1]+")")
		cbfunc = append(cbfunc, MakeWidgetChangedFunction(s[0]))
	}
	cbsdSnapActionsDialog = MakeActionDialogForJail(jail.Jname, "Snapshots for "+jail.Jname, menulines, cbfunc)
	cbsdSnapActionsDialog.Open(viewHolder, gowid.RenderWithRatio{R: 0.3}, app)
}

func (jail *BhyveVm) DestroySnapshot(snapname string) {
	// cbsd jsnapshot mode=destroy jname=nim1 snapname=20220319193339
	var command string
	txtheader := "Destroy jail snapshot...\n"
	args := make([]string, 0)
	if host.USE_DOAS {
		args = append(args, host.CBSD_PROGRAM)
	}
	args = append(args, commandJailSnap)
	args = append(args, "mode=destroy")
	args = append(args, fmt.Sprintf("%s=%s", argJailName, jail.Bname))
	args = append(args, fmt.Sprintf("%s=%s", argSnapName, snapname))
	if host.USE_DOAS {
		command = host.DOAS_PROGRAM
	} else {
		command = host.CBSD_PROGRAM
	}
	ExecCommand(txtheader, command, args)
}

func (jail *BhyveVm) OpenDestroySnapshotDialog(snapname string, viewHolder *holder.Widget, app *gowid.App) {
	var cbsdDestroySnapDialog *dialog.Widget
	cbsdDestroySnapDialog = tui.MakeDialogForJail(
		jail.Bname,
		"Destroy snapshot "+snapname+"\nof jail "+jail.Bname,
		[]string{"Really destroy snapshot " + snapname + "\nof jail " + jail.Bname + "??"},
		nil, nil, nil, nil,
		func(jname string, boolparams []bool, strparams []string) {
			cbsdDestroySnapDialog.Close(app)
			jail.DestroySnapshot(snapname)
		},
	)
	cbsdDestroySnapDialog.Open(viewHolder, gowid.RenderWithRatio{R: 0.3}, app)
}

func (jail *BhyveVm) GetAllParams() []string {
	params := make([]string, 5)
	params[0] = jail.GetAddr()
	params[1] = jail.GetStatusString()
	params[2] = jail.GetAutoStartString()
	params[3] = jail.OsType
	params[4] = jail.VncConsole
	return params
}
