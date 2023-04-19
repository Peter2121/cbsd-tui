package main

import (
	"database/sql"

	"github.com/gcla/gowid"
	"github.com/gcla/gowid/widgets/dialog"
	"github.com/gcla/gowid/widgets/holder"
	_ "github.com/mattn/go-sqlite3"
)

type Jail struct {
	Jname    string
	Ip4_addr string
	Status   int
	Astart   int
	Ver      string
	params   map[string]string
}

var strStatus = []string{"Off", "On", "Slave", "Unknown(3)", "Unknown(4)", "Unknown(5)"}
var strAutoStart = []string{"Off", "On"}
var strHeaderTitles = []string{"NAME", "IP4_ADDRESS", "STATUS", "AUTOSTART", "VERSION"}
var strActionsMenuItems = []string{"Start/Stop", "Create Snapshot", "List Snapshots", "View ", "Edit", "Clone", "Export", "Destroy"}

//var cbsdActionsMenuText = []string{"Start/Stop", "Create Snapshot", "List Snapshots", "Clone", "Export", "Migrate", "Destroy", "Makeresolv", "Show Config"}

var strBottomMenuText1 = []string{" 1", " 2", " 3", " 4", " 5", " 6", " 7", " 8", " 10", " 11", " 12"}
var strBottomMenuText2 = []string{"Help ", "Actions... ", "View ", "Edit ", "Clone ", "Export ", "Create Snap. ", "Destroy ", "Exit ", "List Snap. ", "Start/Stop"}

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

func (jail *Jail) GetName() string {
	return jail.Jname
}

func (jail *Jail) GetStatus() int {
	return jail.Status
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

func New() *Jail {
	res := &Jail{
		Jname:    "",
		Ip4_addr: "",
		Status:   0,
		Astart:   0,
		Ver:      "",
		params:   make(map[string]string),
	}
	return res
}

func NewJail(jname string, ip4_addr string, status int, astart int, ver string) *Jail {
	res := &Jail{
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
		jails = append(jails, jail)
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

	return true, nil
}

func (jail *Jail) Export(*holder.Widget, *gowid.App) {
	// cbsd jexport jname=nim1
	var command string
	txtheader := "Exporting jail...\n"

	args := make([]string, 0)
	if USE_DOAS {
		args = append(args, "cbsd")
	}
	args = append(args, "jexport")
	args = append(args, "jname="+jail.Jname)

	if USE_DOAS {
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
	if USE_DOAS {
		args = append(args, "cbsd")
	}
	args = append(args, "jdestroy")
	args = append(args, "jname="+jail.Jname)
	if USE_DOAS {
		command = doasProgram
	} else {
		command = cbsdProgram
	}
	ExecCommand(txtheader, command, args)
	RefreshJailList()
}

func (jail *Jail) OpenDestroyDialog(viewHolder *holder.Widget, app *gowid.App) {
	var cbsdDestroyJailDialog *dialog.Widget
	cbsdDestroyJailDialog = MakeDialogForJail(
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
	if USE_DOAS {
		args = append(args, "cbsd")
	}
	args = append(args, "jsnapshot")
	args = append(args, "mode=create")
	args = append(args, "snapname="+snapname)
	args = append(args, "jname="+jail.Jname)
	if USE_DOAS {
		command = doasProgram
	} else {
		command = cbsdProgram
	}
	ExecCommand(txtheader, command, args)
}

func (jail *Jail) OpenSnapshotDialog(viewHolder *holder.Widget, app *gowid.App) {
	var cbsdSnapshotJailDialog *dialog.Widget
	cbsdSnapshotJailDialog = MakeDialogForJail(
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
	if USE_DOAS {
		args = append(args, "cbsd")
	}
	args = append(args, "jclone")
	args = append(args, "old="+jail.Jname)
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

func (jail *Jail) OpenCloneDialog(viewHolder *holder.Widget, app *gowid.App) {
	var cbsdCloneJailDialog *dialog.Widget
	cbsdCloneJailDialog = MakeDialogForJail(
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
		cbsdEditJailDialog = MakeDialogForJail(
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
		cbsdEditJailDialog = MakeDialogForJail(
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

func (jail *Jail) ListSnapshots(*holder.Widget, *gowid.App) {
	// cbsd jsnapshot mode=list jname=nim1
	var command string
	txtheader := "List jail snapshots...\n"
	args := make([]string, 0)
	if USE_DOAS {
		args = append(args, "cbsd")
	}
	args = append(args, "jsnapshot")
	args = append(args, "mode=list")
	args = append(args, "jname="+jail.Jname)
	if USE_DOAS {
		command = doasProgram
	} else {
		command = cbsdProgram
	}
	ExecCommand(txtheader, command, args)
}
