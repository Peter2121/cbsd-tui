package bhyve

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

type BhyveVm struct {
	Bname      string
	Ip4_addr   string
	Status     int
	Astart     int
	OsType     string
	VncConsole string
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
var commandJailClone string = "bclone"
var commandJailExport string = "bexport"
var commandJailDestroy string = "bdestroy"
var commandJailStatus string = "jstatus"
var commandJailGetParam string = "bget"
var commandJailSetParam string = "bset"
var argJailIpv4Addr string = "ip4_addr"
var argJailName = "jname"
var argSnapName = "snapname"

func (jail *BhyveVm) GetType() string {
	return "bhyvevm"
}

func (jail *BhyveVm) GetSignalUpdated() *gsignal.Event[string] {
	return &jail.evtUpdated
}

func (jail *BhyveVm) GetSignalRefresh() *gsignal.Event[any] {
	return &jail.evtRefresh
}

func (jail *BhyveVm) SetTui(t *tui.Tui) {
	jail.jtui = t
}

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

func (jail *BhyveVm) GetCurrentStatus() int {
	var stdout, stderr bytes.Buffer
	retstatus := -1
	var command string = ""
	args := make([]string, 0)
	if host.USE_DOAS {
		args = append(args, host.CBSD_PROGRAM)
	}
	args = append(args, commandJailStatus)
	args = append(args, "invert=true")
	args = append(args, fmt.Sprintf("%s=%s", argJailName, jail.Bname))
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

func (jail *BhyveVm) GetVncConsoleAddress() string {
	return jail.VncConsole
}

func (jail *BhyveVm) SetVncConsoleAddress(va string) {
	jail.VncConsole = va
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

func New() BhyveVm {
	res := BhyveVm{
		Bname:      "",
		Ip4_addr:   "",
		Status:     0,
		Astart:     0,
		OsType:     "",
		VncConsole: "",
		params:     make(map[string]string),
		jtui:       nil,
	}
	return res
}

func NewBhyveVm(jname string, ip4_addr string, status int, astart int, os_type string, vnc_console string) BhyveVm {
	res := BhyveVm{
		Bname:      jname,
		Ip4_addr:   ip4_addr,
		Status:     status,
		Astart:     astart,
		OsType:     os_type,
		VncConsole: vnc_console,
		params:     make(map[string]string),
		jtui:       nil,
	}
	return res
}

func GetBhyveVmsFromDb(dbname string) ([]*BhyveVm, error) {
	jails := make([]*BhyveVm, 0)
	var vnc_port int = 0
	var vnc_ip_addr string = ""

	db, err := sql.Open("sqlite3", dbname)
	if err != nil {
		return jails, err
	}
	defer db.Close()

	//jails_list_query := "SELECT jname,ip4_addr,status,astart FROM jails WHERE emulator='bhyve'"
	jails_list_query := "SELECT jails.jname,jails.status,jails.astart,bhyve.vm_os_type,bhyve.vm_vnc_port,bhyve.bhyve_vnc_tcp_bind FROM jails LEFT JOIN bhyve ON jails.jname=bhyve.jname WHERE jails.emulator='bhyve'"
	rows, err := db.Query(jails_list_query)
	if err != nil {
		return jails, err
	}

	for rows.Next() {
		jail := New()
		err = rows.Scan(&jail.Bname, &jail.Status, &jail.Astart, &jail.OsType, &vnc_port, &vnc_ip_addr)
		if err != nil {
			return jails, err
		}
		jail.VncConsole = fmt.Sprintf("%s:%d", vnc_ip_addr, vnc_port)
		jail.Ip4_addr, err = jail.GetJailParam(argJailIpv4Addr)
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

func (jail *BhyveVm) GetVncParams() (string, int) {
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

func (jail *BhyveVm) PutJailToDb(dbname string) (bool, error) {

	var rows_affected int64
	var err error
	db, err := sql.Open("sqlite3", dbname)
	if err != nil {
		return false, err
	}
	defer db.Close()

	result, err1 := db.Exec("UPDATE jails SET astart=? WHERE jname=?", jail.Astart, jail.Bname)
	if err1 != nil {
		return false, err1
	}
	rows_affected, err = result.RowsAffected()
	if err != nil {
		return false, err
	}
	if rows_affected == 0 {
		return false, errors.New("Cannot write VM parameters to database")
	}

	vnc_addr, vnc_port := jail.GetVncParams()
	if (len(vnc_addr) > 0) && (vnc_port > 0) {
		result, err := db.Exec("UPDATE bhyve SET bhyve_vnc_tcp_bind=?,vm_vnc_port=? WHERE jname=?", vnc_addr, vnc_port, jail.Bname)
		if err != nil {
			return false, err
		}
		rows_affected, err = result.RowsAffected()
		if err != nil {
			return false, err
		}
		if rows_affected == 0 {
			return false, errors.New("Cannot write VNC params to database")
		}
	} else {
		return false, errors.New(fmt.Sprintf("Cannot get valid VNC params from %s", jail.VncConsole))
	}

	err = jail.SetJailParam(argJailIpv4Addr, jail.Ip4_addr)
	if err != nil {
		return false, err
	}
	return true, nil
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
	row := db.QueryRow("SELECT jails.jname,jails.status,jails.astart,bhyve.vm_os_type,bhyve.vm_vnc_port,bhyve.bhyve_vnc_tcp_bind FROM jails LEFT JOIN bhyve ON jails.jname=bhyve.jname WHERE jails.emulator='bhyve' AND jails.jname = ?", jname)

	if err := row.Scan(&jail.Bname, &jail.Status, &jail.Astart, &jail.OsType, &vnc_port, &vnc_ip_addr); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	jail.VncConsole = fmt.Sprintf("%s:%d", vnc_ip_addr, vnc_port)
	jail.Ip4_addr, err = jail.GetJailParam(argJailIpv4Addr)
	if (jail.Status == 0) || (jail.Status == 1) {
		cur_status := jail.GetCurrentStatus()
		if cur_status >= 0 {
			jail.Status = cur_status
		}
	}
	return true, nil
}

func (jail *BhyveVm) GetJailFromDbFull(dbname string, jname string) (bool, error) {

	if jail.Bname != jname {
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

	rows, err := db.Query("SELECT * FROM jails LEFT JOIN bhyve ON jails.jname=bhyve.jname WHERE jails.emulator='bhyve' AND jails.jname = ?", jname)
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

func (jail *BhyveVm) GetJailViewString() string {
	var strview string
	_, _ = jail.GetJailFromDbFull(host.GetCbsdDbConnString(false), jail.Bname)
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

	row := db.QueryRow("SELECT jails.jname,jails.status,jails.astart,bhyve.vm_os_type,bhyve.vm_vnc_port,bhyve.bhyve_vnc_tcp_bind FROM jails LEFT JOIN bhyve ON jails.jname=bhyve.jname WHERE jails.emulator='bhyve' AND jails.jname = ?", jail.Bname)

	if err := row.Scan(&jail.Bname, &jail.Status, &jail.Astart, &jail.OsType, &vnc_port, &vnc_ip_addr); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	jail.VncConsole = fmt.Sprintf("%s:%d", vnc_ip_addr, vnc_port)
	jail.Ip4_addr, err = jail.GetJailParam(argJailIpv4Addr)
	if (jail.Status == 0) || (jail.Status == 1) {
		cur_status := jail.GetCurrentStatus()
		if cur_status >= 0 {
			jail.Status = cur_status
		}
	}
	return true, nil
}

func (jail *BhyveVm) Export() {
	// cbsd jexport jname=nim1
	var command string
	txtheader := "Exporting VM...\n"

	args := make([]string, 0)
	if host.USE_DOAS {
		args = append(args, host.CBSD_PROGRAM)
	}
	args = append(args, commandJailExport)
	args = append(args, fmt.Sprintf("%s=%s", argJailName, jail.Bname))

	if host.USE_DOAS {
		command = host.DOAS_PROGRAM
	} else {
		command = host.CBSD_PROGRAM
	}
	jail.jtui.ExecCommand(txtheader, command, args)
}

func (jail *BhyveVm) Destroy() {
	// cbsd jdestroy jname=nim1
	var command string
	txtheader := "Destroying VM " + jail.Bname + "...\n"
	args := make([]string, 0)
	if host.USE_DOAS {
		args = append(args, host.CBSD_PROGRAM)
	}
	args = append(args, commandJailDestroy)
	args = append(args, fmt.Sprintf("%s=%s", argJailName, jail.Bname))
	if host.USE_DOAS {
		command = host.DOAS_PROGRAM
	} else {
		command = host.CBSD_PROGRAM
	}
	jail.jtui.ExecCommand(txtheader, command, args)
	jail.evtRefresh.Emit(nil)
}

func (jail *BhyveVm) OpenDestroyDialog() {
	var cbsdDestroyJailDialog *dialog.Widget
	cbsdDestroyJailDialog = jail.jtui.MakeDialogForJail(
		jail.Bname,
		"Destroy VM "+jail.Bname,
		[]string{"Really destroy VM " + jail.Bname + "??"},
		nil, nil, nil, nil,
		func(jname string, boolparams []bool, strparams []string) {
			cbsdDestroyJailDialog.Close(jail.jtui.App)
			jail.Destroy()
		},
	)
	cbsdDestroyJailDialog.Open(jail.jtui.ViewHolder, gowid.RenderWithRatio{R: 0.3}, jail.jtui.App)
}

func (jail *BhyveVm) Snapshot(snapname string) {
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
	args = append(args, fmt.Sprintf("%s=%s", argJailName, jail.Bname))
	if host.USE_DOAS {
		command = host.DOAS_PROGRAM
	} else {
		command = host.CBSD_PROGRAM
	}
	jail.jtui.ExecCommand(txtheader, command, args)
}

func (jail *BhyveVm) OpenSnapshotDialog() {
	var cbsdSnapshotJailDialog *dialog.Widget
	cbsdSnapshotJailDialog = jail.jtui.MakeDialogForJail(
		jail.Bname,
		"Snapshot VM "+jail.Bname,
		nil, nil, nil,
		[]string{"Snapshot name: "}, []string{"gettimeofday"},
		func(jname string, boolparams []bool, strparams []string) {
			cbsdSnapshotJailDialog.Close(jail.jtui.App)
			jail.Snapshot(strparams[0])
		},
	)
	cbsdSnapshotJailDialog.Open(jail.jtui.ViewHolder, gowid.RenderWithRatio{R: 0.3}, jail.jtui.App)
}

func (jail *BhyveVm) Clone(jnewjname string, jnewhname string, newip string) {
	//log.Infof("Clone %s to %s (%s) IP %s", jname, jnewjname, jnewhname, newip)
	// cbsd jclone old=jail1 new=jail1clone host_hostname=jail1clone.domain.local ip4_addr=DHCP checkstate=0
	var command string
	txtheader := "Cloning VM...\n"

	args := make([]string, 0)
	if host.USE_DOAS {
		args = append(args, host.CBSD_PROGRAM)
	}
	args = append(args, commandJailClone)
	args = append(args, fmt.Sprintf("old=%s", jail.Bname))
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

func (jail *BhyveVm) OpenCloneDialog() {
	var cbsdCloneJailDialog *dialog.Widget
	cbsdCloneJailDialog = jail.jtui.MakeDialogForJail(
		jail.Bname,
		"Clone VM "+jail.Bname,
		nil, nil, nil,
		[]string{"New VM name: ", "New host name: ", "New IP address: "},
		[]string{jail.Bname + "clone", jail.Bname, "DHCP"},
		func(jname string, boolparams []bool, strparams []string) {
			cbsdCloneJailDialog.Close(jail.jtui.App)
			jail.Clone(strparams[0], strparams[1], strparams[2])
		},
	)
	cbsdCloneJailDialog.Open(jail.jtui.ViewHolder, gowid.RenderWithRatio{R: 0.3}, jail.jtui.App)
}

func (jail *BhyveVm) Edit(astart bool, vnc_console string, ip string) {
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
	jail.evtUpdated.Emit(jail.Bname)
}

func (jail *BhyveVm) OpenEditDialog() {
	var cbsdEditJailDialog *dialog.Widget
	if !jail.IsRunning() {
		cbsdEditJailDialog = jail.jtui.MakeDialogForJail(
			jail.Bname,
			"Edit VM "+jail.Bname,
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
			jail.Bname,
			"Edit VM "+jail.Bname,
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

func (jail *BhyveVm) View() {
	viewspace := edit.New(edit.Options{ReadOnly: true})
	outdlg := jail.jtui.CreateActionsLogDialog(viewspace, jail.jtui.Console.Height())
	outdlg.Open(jail.jtui.ViewHolder, gowid.RenderWithRatio{R: 0.7}, jail.jtui.App)
	viewspace.SetText(jail.GetJailViewString(), jail.jtui.App)
	jail.jtui.App.RedrawTerminal()
}

func (jail *BhyveVm) StartStop() {
	txtheader := ""
	var args []string
	var command string

	if jail.IsRunning() {
		if tui.CbsdJailConsoleActive == jail.Bname {
			jail.jtui.SendTerminalCommand("exit")
			tui.CbsdJailConsoleActive = ""
		}
		txtheader = "Stopping VM...\n"
		if host.USE_DOAS {
			args = append(args, host.CBSD_PROGRAM)
		}
		args = append(args, commandJailStop)
		args = append(args, "inter=1")
		args = append(args, fmt.Sprintf("%s=%s", argJailName, jail.Bname))
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
	jail.evtUpdated.Emit(jail.Bname)
}

func (jail *BhyveVm) GetStartCommand() string {
	cmd := fmt.Sprintf("%s inter=1 %s=%s", commandJailStart, argJailName, jail.Bname)
	return cmd
}

func (jail *BhyveVm) GetLoginCommand() string {
	cmd := fmt.Sprintf("%s %s=%s", commandJailLogin, argJailName, jail.Bname)
	return cmd
}

func (jail *BhyveVm) CreateScriptStartJail() (string, error) {
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

func (jail *BhyveVm) OpenActionDialog() {
	var cbsdActionsDialog *dialog.Widget
	var MenuLines []string
	if jail.IsRunning() {
		MenuLines = jail.GetStartedActionsMenuItems()
	} else if jail.IsRunnable() {
		MenuLines = jail.GetStoppedActionsMenuItems()
	} else {
		MenuLines = jail.GetNonRunnableActionsMenuItems()
	}
	cbsdActionsDialog = jail.jtui.MakeActionDialogForJail(jail.Bname, "Actions for "+jail.Bname, MenuLines,
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

func (jail *BhyveVm) ExecuteActionOnCommand(command string) {
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

func (jail *BhyveVm) ExecuteActionOnKey(tkey int16) {
	for i, k := range keysBottomMenu {
		if int16(k) == tkey {
			jail.ExecuteActionOnCommand(strBottomMenuText2[i])
		}
	}
}

func (jail *BhyveVm) GetSnapshots() [][2]string {
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
	args = append(args, fmt.Sprintf("%s=%s", argJailName, jail.Bname))
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

func (jail *BhyveVm) OpenSnapActionsDialog() {
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
	cbsdSnapActionsDialog = jail.jtui.MakeActionDialogForJail(jail.Bname, "Snapshots for "+jail.Bname, menulines, cbfunc)
	cbsdSnapActionsDialog.Open(jail.jtui.ViewHolder, gowid.RenderWithRatio{R: 0.3}, jail.jtui.App)
}

func (jail *BhyveVm) DestroySnapshot(snapname string) {
	// cbsd jsnapshot mode=destroy jname=nim1 snapname=20220319193339
	var command string
	txtheader := "Destroy VM snapshot...\n"
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
	if jail.jtui != nil {
		jail.jtui.ExecCommand(txtheader, command, args)
	}
}

func (jail *BhyveVm) OpenDestroySnapshotDialog(snapname string) {
	var cbsdDestroySnapDialog *dialog.Widget
	cbsdDestroySnapDialog = jail.jtui.MakeDialogForJail(
		jail.Bname,
		"Destroy snapshot "+snapname+"\nof jail "+jail.Bname,
		[]string{"Really destroy snapshot " + snapname + "\nof jail " + jail.Bname + "??"},
		nil, nil, nil, nil,
		func(jname string, boolparams []bool, strparams []string) {
			cbsdDestroySnapDialog.Close(jail.jtui.App)
			jail.DestroySnapshot(snapname)
		},
	)
	cbsdDestroySnapDialog.Open(jail.jtui.ViewHolder, gowid.RenderWithRatio{R: 0.3}, jail.jtui.App)
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

func (jail *BhyveVm) GetJailParam(param string) (string, error) {
	var stdout, stderr bytes.Buffer
	var command string = ""
	args := make([]string, 0)
	if host.USE_DOAS {
		args = append(args, host.CBSD_PROGRAM)
	}
	args = append(args, commandJailGetParam)
	args = append(args, "mode=quiet")
	args = append(args, param)
	args = append(args, fmt.Sprintf("%s=%s", argJailName, jail.Bname))
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
		return "", err
	}
	str_out := string(stdout.Bytes())
	str_out = strings.TrimSuffix(str_out, "\n")
	return str_out, nil
}

func (jail *BhyveVm) SetJailParam(param string, value string) error {
	var command string
	args := make([]string, 0)
	if host.USE_DOAS {
		args = append(args, host.CBSD_PROGRAM)
	}
	args = append(args, commandJailSetParam)
	args = append(args, fmt.Sprintf("%s=%s", param, value))
	args = append(args, fmt.Sprintf("%s=%s", argJailName, jail.Bname))
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
