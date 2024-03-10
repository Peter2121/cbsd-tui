package qemu

import (
	"bytes"
	"database/sql"
	"errors"
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
	"github.com/quasilyte/gsignal"

	"host"
	"tui"
)

type CBase struct {
	Name       string
	Ip4_addr   string
	Status     int
	Astart     int
	params     map[string]string
	jtui       *tui.Tui
	evtUpdated gsignal.Event[string]
	evtRefresh gsignal.Event[any]
}

type QemuVm struct {
	CBase
	OsType     string
	VncConsole string
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
	DESTROY    = "Destroy VM"
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

var commandJailLogin string = "qlogin"
var commandJailStart string = "qstart"
var commandJailStop string = "qstop"
var commandJailSnap string = "jsnapshot"
var commandJailClone string = "qclone"
var commandJailExport string = "qexport"
var commandJailDestroy string = "qdestroy"
var commandJailStatus string = "jstatus"
var commandJailGetParam string = "qget"
var commandJailSetParam string = "qset"
var argJailIpv4Addr string = "ip4_addr"
var argJailName = "jname"
var argSnapName = "snapname"

func (jail *QemuVm) GetType() string {
	return "qemuvm"
}

func (jail *CBase) GetSignalUpdated() *gsignal.Event[string] {
	return &jail.evtUpdated
}

func (jail *CBase) GetSignalRefresh() *gsignal.Event[any] {
	return &jail.evtRefresh
}

func (jail *CBase) SetTui(t *tui.Tui) {
	jail.jtui = t
}

func (jail *QemuVm) GetCommandHelp() string {
	return HELP
}

func (jail *QemuVm) GetCommandExit() string {
	return EXIT
}

func (jail *QemuVm) GetBottomMenuText1() []string {
	return strBottomMenuText1
}

func (jail *QemuVm) GetBottomMenuText2() []string {
	return strBottomMenuText2
}

func (jail *QemuVm) GetHeaderTitles() []string {
	return strHeaderTitles
}

func (jail *QemuVm) GetActionsMenuItems() []string {
	return strActionsMenuItems
}

func (jail *QemuVm) GetStartedActionsMenuItems() []string {
	return strStartedActionsMenuItems
}

func (jail *QemuVm) GetStoppedActionsMenuItems() []string {
	return strStoppedActionsMenuItems
}

func (jail *QemuVm) GetNonRunnableActionsMenuItems() []string {
	return strNonRunnableActionsMenuItems
}

func (jail *CBase) GetName() string {
	return jail.Name
}

func (jail *CBase) GetStatus() int {
	return jail.Status
}

func (jail *CBase) GetCommandResult(command string, args []string) (string, error) {
	var result string = ""
	var err error = nil
	var stdout, stderr bytes.Buffer

	cmd := exec.Command(command, args...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "NOCOLOR=1")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		//log.Errorf("cmd.Run() failed with %s\n", err)
		return result, err
	}
	str_out := string(stdout.Bytes())
	str_out = strings.TrimSuffix(str_out, "\n")
	return str_out, err
}

func (jail *QemuVm) GetCurrentStatus() int {
	retstatus := -1
	var command string = ""
	args := make([]string, 0)
	if host.USE_DOAS {
		args = append(args, host.CBSD_PROGRAM)
	}
	args = append(args, commandJailStatus)
	args = append(args, "invert=true")
	args = append(args, fmt.Sprintf("%s=%s", argJailName, jail.Name))
	if host.USE_DOAS {
		command = host.DOAS_PROGRAM
	} else {
		command = host.CBSD_PROGRAM
	}
	str_out, err := jail.GetCommandResult(command, args)
	if (err != nil) || (len(str_out) == 0) {
		return retstatus
	}
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
	return retstatus
}

func (jail *CBase) GetAddr() string {
	return jail.Ip4_addr
}

func (jail *CBase) SetAddr(addr string) {
	jail.Ip4_addr = addr
}

func (jail *CBase) GetAstart() int {
	return jail.Astart
}

func (jail *CBase) SetAstart(as int) {
	jail.Astart = as
}

func (jail *QemuVm) GetVncConsoleAddress() string {
	return jail.VncConsole
}

func (jail *QemuVm) SetVncConsoleAddress(va string) {
	jail.VncConsole = va
}

func (jail *CBase) IsRunning() bool {
	if jail.Status == 1 {
		return true
	} else {
		return false
	}
}

func (jail *CBase) IsRunnable() bool {
	if jail.Status == 0 {
		return true
	} else {
		return false
	}
}

func (jail *QemuVm) GetStatusString() string {
	return strStatus[jail.Status]
}

func (jail *QemuVm) GetAutoStartString() string {
	return strAutoStart[jail.Astart]
}

func (jail *CBase) GetAutoStartBool() bool {
	if jail.Astart == 1 {
		return true
	} else {
		return false
	}
}

func (jail *QemuVm) GetAutoStartCode(astart string) int {
	for i, m := range strAutoStart {
		if m == astart {
			return i
		}
	}
	return -1
}

func (jail *QemuVm) GetStatusCode(status string) int {
	for i, m := range strStatus {
		if m == status {
			return i
		}
	}
	return -1
}

func (jail *CBase) GetParam(pn string) string {
	return jail.params[pn]
}

func (jail *CBase) SetParam(pn string, pv string) bool {
	jail.params[pn] = pv
	if v, found := jail.params[pn]; found {
		if v == pv {
			return true
		}
	}
	return false
}

func New() QemuVm {
	res := QemuVm{
		CBase: CBase{
			Name:     "",
			Ip4_addr: "",
			Status:   0,
			Astart:   0,
			params:   make(map[string]string),
			jtui:     nil,
		},
		OsType:     "",
		VncConsole: "",
	}
	return res
}

func NewQemuVm(jname string, ip4_addr string, status int, astart int, os_type string, vnc_console string) QemuVm {
	res := QemuVm{
		CBase: CBase{
			Name:     jname,
			Ip4_addr: ip4_addr,
			Status:   status,
			Astart:   astart,
			params:   make(map[string]string),
			jtui:     nil,
		},
		OsType:     os_type,
		VncConsole: vnc_console,
	}
	return res
}

func GetQemuVmsFromDb(dbname string) ([]*QemuVm, error) {
	jails := make([]*QemuVm, 0)
	var vnc_port int = 0
	var vnc_ip_addr string = ""

	db, err := sql.Open("sqlite3", dbname)
	if err != nil {
		return jails, err
	}

	jails_list_query := "SELECT jname FROM jails WHERE jails.emulator='qemu'"
	rows, err := db.Query(jails_list_query)
	if err != nil {
		return jails, err
	}

	jnames := make([]string, 0)
	var jname string
	for rows.Next() {
		err = rows.Scan(&jname)
		if err != nil {
			return jails, err
		}
		if len(jname) != 0 {
			jnames = append(jnames, jname)
		}
	}
	rows.Close()
	db.Close()

	for _, jn := range jnames {
		dbj, err := sql.Open("sqlite3", host.GetJailDbConnString(jn, false))
		if err != nil {
			continue
		}
		jail := New()
		jail_params_query := "SELECT astart, vm_os_type, vm_vnc_port, qemu_vnc_tcp_bind FROM settings"
		row := dbj.QueryRow(jail_params_query)
		if err := row.Scan(&jail.Astart, &jail.OsType, &vnc_port, &vnc_ip_addr); err != nil {
			continue
		}
		dbj.Close()
		jail.Name = jn
		jail.VncConsole = fmt.Sprintf("%s:%d", vnc_ip_addr, vnc_port)
		jail.Ip4_addr, err = jail.GetJailParam(argJailIpv4Addr)
		if err != nil {
			continue
		}
		cur_status := jail.GetCurrentStatus()
		if cur_status >= 0 {
			jail.Status = cur_status
		}
		jails = append(jails, &jail)
	}
	return jails, nil
}

func (jail *QemuVm) GetVncParams() (string, int) {
	data := strings.Split(jail.VncConsole, ":")
	if len(data) < 2 {
		return "", 0
	}
	port, err := strconv.Atoi(data[1])
	if err != nil {
		return "", 0
	}
	// TODO: validate IP address and TCP port
	return data[0], port
}

func (jail *QemuVm) PutJailToDb(dbname string) (bool, error) {

	var rows_affected int64
	var err error
	dbj, errdb := sql.Open("sqlite3", host.GetJailDbConnString(jail.Name, true))
	if errdb != nil {
		return false, errdb
	}

	vnc_addr, vnc_port := jail.GetVncParams()
	if (len(vnc_addr) > 0) && (vnc_port > 0) {
		result, err := dbj.Exec("UPDATE settings SET astart=?, vm_os_type=?, vm_vnc_port=?, qemu_vnc_tcp_bind=?", jail.Astart, jail.OsType, vnc_port, vnc_addr)
		if err != nil {
			return false, err
		}
		rows_affected, err = result.RowsAffected()
		if err != nil {
			return false, err
		}
		if rows_affected == 0 {
			return false, errors.New("Cannot write VM params to database")
		}
	} else {
		return false, errors.New(fmt.Sprintf("Cannot get valid VNC params from %s", jail.VncConsole))
	}
	dbj.Close()

	err = jail.SetJailParam(argJailIpv4Addr, jail.Ip4_addr)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (jail *QemuVm) GetJailFromDb(dbname string, jname string) (bool, error) {
	/*
		db, err := sql.Open("sqlite3", dbname)
		if err != nil {
			return false, err
		}
		defer db.Close()
	*/
	var err error
	var vnc_port int = 0
	var vnc_ip_addr string = ""

	dbj, errdb := sql.Open("sqlite3", host.GetJailDbConnString(jail.Name, false))
	if errdb != nil {
		return false, errdb
	}

	//row := db.QueryRow("SELECT jname,ip4_addr,status,astart,ver FROM jails WHERE jname = ?", jname)
	row := dbj.QueryRow("SELECT astart, vm_os_type, vm_vnc_port, qemu_vnc_tcp_bind FROM settings")

	if err := row.Scan(&jail.Astart, &jail.OsType, &vnc_port, &vnc_ip_addr); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	jail.VncConsole = fmt.Sprintf("%s:%d", vnc_ip_addr, vnc_port)
	jail.Ip4_addr, err = jail.GetJailParam(argJailIpv4Addr)
	if err != nil {
		return false, err
	}
	cur_status := jail.GetCurrentStatus()
	if cur_status >= 0 {
		jail.Status = cur_status
	}
	return true, nil
}

func (jail *QemuVm) GetJailFromDbFull(dbname string, jname string) (bool, error) {

	if jail.Name != jname {
		result, err := jail.GetJailFromDb(dbname, jname)
		if err != nil {
			return false, err
		}
		if !result {
			return result, nil
		}
	}
	result := false
	db, err := sql.Open("sqlite3", host.GetJailDbConnString(jail.Name, false))
	if err != nil {
		return false, err
	}
	defer db.Close()

	rows, err := db.Query("SELECT * FROM settings")
	if err != nil {
		return false, err
	}

	cols, err := rows.Columns()
	if err != nil {
		return false, err
	}

	rawResult := make([][]byte, len(cols))

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
			if raw != nil {
				jail.params[cols[i]] = string(raw)
				result = true
			}
		}
		//fmt.Printf("%#v\n", result)
	}
	return result, nil
}

func (jail *QemuVm) GetJailViewString() string {
	var strview string
	_, _ = jail.GetJailFromDbFull(host.GetCbsdDbConnString(false), jail.Name)
	strview += "Name: " + jail.Name + "\n"
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

func (jail *QemuVm) UpdateJailFromDb(dbname string) (bool, error) {
	db, err := sql.Open("sqlite3", host.GetJailDbConnString(jail.Name, false))
	if err != nil {
		return false, err
	}
	defer db.Close()

	var vnc_port int = 0
	var vnc_ip_addr string = ""

	row := db.QueryRow("SELECT astart, vm_os_type, vm_vnc_port, qemu_vnc_tcp_bind FROM settings")
	//row := db.QueryRow("SELECT jails.jname,jails.status,jails.astart,bhyve.vm_os_type,bhyve.vm_vnc_port,bhyve.bhyve_vnc_tcp_bind FROM jails LEFT JOIN bhyve ON jails.jname=bhyve.jname WHERE jails.emulator='bhyve' AND jails.jname = ?", jail.Name)

	if err := row.Scan(&jail.Astart, &jail.OsType, &vnc_port, &vnc_ip_addr); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	jail.VncConsole = fmt.Sprintf("%s:%d", vnc_ip_addr, vnc_port)
	jail.Ip4_addr, err = jail.GetJailParam(argJailIpv4Addr)
	cur_status := jail.GetCurrentStatus()
	if cur_status >= 0 {
		jail.Status = cur_status
	}
	return true, nil
}

func (jail *QemuVm) Export() {
	// cbsd jexport jname=nim1
	var command string
	txtheader := "Exporting VM...\n"

	args := make([]string, 0)
	if host.USE_DOAS {
		args = append(args, host.CBSD_PROGRAM)
	}
	args = append(args, commandJailExport)
	args = append(args, fmt.Sprintf("%s=%s", argJailName, jail.Name))

	if host.USE_DOAS {
		command = host.DOAS_PROGRAM
	} else {
		command = host.CBSD_PROGRAM
	}
	jail.jtui.ExecCommand(txtheader, command, args)
}

func (jail *QemuVm) Destroy() {
	// cbsd jdestroy jname=nim1
	var command string
	txtheader := "Destroying VM " + jail.Name + "...\n"
	args := make([]string, 0)
	if host.USE_DOAS {
		args = append(args, host.CBSD_PROGRAM)
	}
	args = append(args, commandJailDestroy)
	args = append(args, fmt.Sprintf("%s=%s", argJailName, jail.Name))
	if host.USE_DOAS {
		command = host.DOAS_PROGRAM
	} else {
		command = host.CBSD_PROGRAM
	}
	jail.jtui.ExecCommand(txtheader, command, args)
	jail.evtRefresh.Emit(nil)
}

func (jail *QemuVm) OpenDestroyDialog() {
	var cbsdDestroyJailDialog *dialog.Widget
	cbsdDestroyJailDialog = jail.jtui.MakeDialogForJail(
		jail.Name,
		"Destroy VM "+jail.Name,
		[]string{"Really destroy VM " + jail.Name + "??"},
		nil, nil, nil, nil,
		func(jname string, boolparams []bool, strparams []string) {
			cbsdDestroyJailDialog.Close(jail.jtui.App)
			jail.Destroy()
		},
	)
	cbsdDestroyJailDialog.Open(jail.jtui.ViewHolder, gowid.RenderWithRatio{R: 0.3}, jail.jtui.App)
}

func (jail *QemuVm) Snapshot(snapname string) {
	// cbsd jsnapshot mode=create snapname=gettimeofday jname=nim1
	var command string
	txtheader := "Creating VM snapshot...\n"
	args := make([]string, 0)
	if host.USE_DOAS {
		args = append(args, host.CBSD_PROGRAM)
	}
	args = append(args, commandJailSnap)
	args = append(args, "mode=create")
	args = append(args, fmt.Sprintf("%s=%s", argSnapName, snapname))
	args = append(args, fmt.Sprintf("%s=%s", argJailName, jail.Name))
	if host.USE_DOAS {
		command = host.DOAS_PROGRAM
	} else {
		command = host.CBSD_PROGRAM
	}
	jail.jtui.ExecCommand(txtheader, command, args)
}

func (jail *QemuVm) OpenSnapshotDialog() {
	var cbsdSnapshotJailDialog *dialog.Widget
	cbsdSnapshotJailDialog = jail.jtui.MakeDialogForJail(
		jail.Name,
		"Snapshot VM "+jail.Name,
		nil, nil, nil,
		[]string{"Snapshot name: "}, []string{"gettimeofday"},
		func(jname string, boolparams []bool, strparams []string) {
			cbsdSnapshotJailDialog.Close(jail.jtui.App)
			jail.Snapshot(strparams[0])
		},
	)
	cbsdSnapshotJailDialog.Open(jail.jtui.ViewHolder, gowid.RenderWithRatio{R: 0.3}, jail.jtui.App)
}

func (jail *QemuVm) Clone(jnewjname string, jnewhname string, newip string) {
	//log.Infof("Clone %s to %s (%s) IP %s", jname, jnewjname, jnewhname, newip)
	// cbsd jclone old=jail1 new=jail1clone host_hostname=jail1clone.domain.local ip4_addr=DHCP checkstate=0
	var command string
	txtheader := "Cloning VM...\n"

	args := make([]string, 0)
	if host.USE_DOAS {
		args = append(args, host.CBSD_PROGRAM)
	}
	args = append(args, commandJailClone)
	args = append(args, fmt.Sprintf("old=%s", jail.Name))
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

func (jail *QemuVm) OpenCloneDialog() {
	var cbsdCloneJailDialog *dialog.Widget
	cbsdCloneJailDialog = jail.jtui.MakeDialogForJail(
		jail.Name,
		"Clone VM "+jail.Name,
		nil, nil, nil,
		[]string{"New VM name: ", "New host name: ", "New IP address: "},
		[]string{jail.Name + "clone", jail.Name, "DHCP"},
		func(jname string, boolparams []bool, strparams []string) {
			cbsdCloneJailDialog.Close(jail.jtui.App)
			jail.Clone(strparams[0], strparams[1], strparams[2])
		},
	)
	cbsdCloneJailDialog.Open(jail.jtui.ViewHolder, gowid.RenderWithRatio{R: 0.3}, jail.jtui.App)
}

func (jail *QemuVm) Edit(astart bool, vnc_console string, ip string) {
	if astart != jail.GetAutoStartBool() {
		if astart {
			jail.SetAstart(1)
		} else {
			jail.SetAstart(0)
		}
	}
	if ip != "" {
		if ip != jail.GetAddr() {
			jail.SetAddr(ip)
		}
	}
	if vnc_console != "" {
		if vnc_console != jail.GetVncConsoleAddress() {
			jail.SetVncConsoleAddress(vnc_console)
		}
	}
	_, err := jail.PutJailToDb(host.GetCbsdDbConnString(true))
	if err != nil {
		panic(err)
	}
	jail.evtUpdated.Emit(jail.Name)
}

func (jail *QemuVm) OpenEditDialog() {
	var cbsdEditJailDialog *dialog.Widget
	if !jail.IsRunning() {
		cbsdEditJailDialog = jail.jtui.MakeDialogForJail(
			jail.Name,
			"Edit VM "+jail.Name,
			nil,
			[]string{"Autostart "}, []bool{jail.GetAutoStartBool()},
			[]string{"VNC Console: ", "IP address: "},
			[]string{jail.GetVncConsoleAddress(), jail.GetAddr()},
			func(jname string, boolparams []bool, strparams []string) {
				cbsdEditJailDialog.Close(jail.jtui.App)
				jail.Edit(boolparams[0], strparams[0], strparams[1])
			},
		)
	} else {
		cbsdEditJailDialog = jail.jtui.MakeDialogForJail(
			jail.Name,
			"Edit VM "+jail.Name,
			nil,
			[]string{"Autostart "}, []bool{jail.GetAutoStartBool()},
			nil,
			nil,
			func(jname string, boolparams []bool, strparams []string) {
				cbsdEditJailDialog.Close(jail.jtui.App)
				jail.Edit(boolparams[0], strparams[0], "")
			},
		)
	}
	cbsdEditJailDialog.Open(jail.jtui.ViewHolder, gowid.RenderWithRatio{R: 0.3}, jail.jtui.App)
}

func (jail *QemuVm) View() {
	viewspace := edit.New(edit.Options{ReadOnly: true})
	outdlg := jail.jtui.CreateActionsLogDialog(viewspace, jail.jtui.Console.Height())
	outdlg.Open(jail.jtui.ViewHolder, gowid.RenderWithRatio{R: 0.7}, jail.jtui.App)
	viewspace.SetText(jail.GetJailViewString(), jail.jtui.App)
	jail.jtui.App.RedrawTerminal()
}

func (jail *QemuVm) StartStop() {
	txtheader := ""
	var args []string
	var command string

	if jail.IsRunning() {
		if tui.CbsdJailConsoleActive == jail.Name {
			jail.jtui.SendTerminalCommand("exit")
			tui.CbsdJailConsoleActive = ""
		}
		txtheader = "Stopping VM...\n"
		if host.USE_DOAS {
			args = append(args, host.CBSD_PROGRAM)
		}
		args = append(args, commandJailStop)
		args = append(args, "inter=1")
		args = append(args, fmt.Sprintf("%s=%s", argJailName, jail.Name))
		if host.USE_DOAS {
			command = host.DOAS_PROGRAM
		} else {
			command = host.CBSD_PROGRAM
		}
		jail.jtui.ExecCommand(txtheader, command, args)
	} else if jail.IsRunnable() {
		txtheader = "Starting VM...\n"
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
	jail.evtUpdated.Emit(jail.Name)
}

func (jail *QemuVm) GetStartCommand() string {
	cmd := fmt.Sprintf("%s inter=1 %s=%s", commandJailStart, argJailName, jail.Name)
	return cmd
}

func (jail *QemuVm) GetLoginCommand() string {
	cmd := fmt.Sprintf("%s %s=%s", commandJailLogin, argJailName, jail.Name)
	return cmd
}

func (jail *QemuVm) CreateScriptStartJail() (string, error) {
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

func (jail *QemuVm) OpenActionDialog() {
	var cbsdActionsDialog *dialog.Widget
	var MenuLines []string
	if jail.IsRunning() {
		MenuLines = jail.GetStartedActionsMenuItems()
	} else if jail.IsRunnable() {
		MenuLines = jail.GetStoppedActionsMenuItems()
	} else {
		MenuLines = jail.GetNonRunnableActionsMenuItems()
	}
	cbsdActionsDialog = jail.jtui.MakeActionDialogForJail(jail.Name, "Actions for "+jail.Name, MenuLines,
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

func (jail *QemuVm) ExecuteActionOnCommand(command string) {
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

func (jail *QemuVm) ExecuteActionOnKey(tkey int16) {
	for i, k := range keysBottomMenu {
		if int16(k) == tkey {
			jail.ExecuteActionOnCommand(strBottomMenuText2[i])
		}
	}
}

func (jail *QemuVm) GetSnapshots() [][2]string {
	var snap = [2]string{"", ""}
	retsnap := make([][2]string, 0)
	var command string

	// cbsd jsnapshot jname=jinja1 mode=list header=0 display=snapname,creation
	args := make([]string, 0)
	if host.USE_DOAS {
		args = append(args, host.CBSD_PROGRAM)
	}
	args = append(args, commandJailSnap)
	args = append(args, "mode=list")
	args = append(args, "header=0")
	args = append(args, "display=snapname,creation")
	args = append(args, fmt.Sprintf("%s=%s", argJailName, jail.Name))
	if host.USE_DOAS {
		command = host.DOAS_PROGRAM
	} else {
		command = host.CBSD_PROGRAM
	}
	str_out, err := jail.GetCommandResult(command, args)
	if err != nil {
		host.LogError("GetCommandResult() failed", err)
		return retsnap
	}
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

func (jail *QemuVm) OpenSnapActionsDialog() {
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
	cbsdSnapActionsDialog = jail.jtui.MakeActionDialogForJail(jail.Name, "Snapshots for "+jail.Name, menulines, cbfunc)
	cbsdSnapActionsDialog.Open(jail.jtui.ViewHolder, gowid.RenderWithRatio{R: 0.3}, jail.jtui.App)
}

func (jail *QemuVm) DestroySnapshot(snapname string) {
	// cbsd jsnapshot mode=destroy jname=nim1 snapname=20220319193339
	var command string
	txtheader := "Destroy VM snapshot...\n"
	args := make([]string, 0)
	if host.USE_DOAS {
		args = append(args, host.CBSD_PROGRAM)
	}
	args = append(args, commandJailSnap)
	args = append(args, "mode=destroy")
	args = append(args, fmt.Sprintf("%s=%s", argJailName, jail.Name))
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

func (jail *QemuVm) OpenDestroySnapshotDialog(snapname string) {
	var cbsdDestroySnapDialog *dialog.Widget
	cbsdDestroySnapDialog = jail.jtui.MakeDialogForJail(
		jail.Name,
		"Destroy snapshot "+snapname+"\nof jail "+jail.Name,
		[]string{"Really destroy snapshot " + snapname + "\nof jail " + jail.Name + "??"},
		nil, nil, nil, nil,
		func(jname string, boolparams []bool, strparams []string) {
			cbsdDestroySnapDialog.Close(jail.jtui.App)
			jail.DestroySnapshot(snapname)
		},
	)
	cbsdDestroySnapDialog.Open(jail.jtui.ViewHolder, gowid.RenderWithRatio{R: 0.3}, jail.jtui.App)
}

func (jail *QemuVm) GetAllParams() []string {
	params := make([]string, 5)
	params[0] = jail.GetAddr()
	params[1] = jail.GetStatusString()
	params[2] = jail.GetAutoStartString()
	params[3] = jail.OsType
	params[4] = jail.VncConsole
	return params
}

func (jail *QemuVm) GetJailParam(param string) (string, error) {
	var command string = ""
	args := make([]string, 0)
	if host.USE_DOAS {
		args = append(args, host.CBSD_PROGRAM)
	}
	args = append(args, commandJailGetParam)
	args = append(args, "mode=quiet")
	args = append(args, param)
	args = append(args, fmt.Sprintf("%s=%s", argJailName, jail.Name))
	if host.USE_DOAS {
		command = host.DOAS_PROGRAM
	} else {
		command = host.CBSD_PROGRAM
	}
	str_out, err := jail.GetCommandResult(command, args)
	if err != nil {
		host.LogError("GetCommandResult() failed", err)
		return "", err
	}
	str_out = strings.TrimSuffix(str_out, "\n")
	return str_out, nil
}

func (jail *QemuVm) SetJailParam(param string, value string) error {
	var command string
	args := make([]string, 0)
	if host.USE_DOAS {
		args = append(args, host.CBSD_PROGRAM)
	}
	args = append(args, commandJailSetParam)
	args = append(args, fmt.Sprintf("%s=%s", param, value))
	args = append(args, fmt.Sprintf("%s=%s", argJailName, jail.Name))
	if host.USE_DOAS {
		command = host.DOAS_PROGRAM
	} else {
		command = host.CBSD_PROGRAM
	}
	cmd := exec.Command(command, args...)
	err := cmd.Run()
	if err != nil {
		//log.Errorf("cmd.Run() failed with %s\n", err)
		return err
	}
	return nil
}
