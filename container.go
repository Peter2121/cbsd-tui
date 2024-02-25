package main

import (
	"tui"

	"github.com/quasilyte/gsignal"
)

type Container interface {
	GetSignalUpdated() *gsignal.Event[string]
	GetSignalRefresh() *gsignal.Event[any]
	SetTui(t *tui.Tui)
	GetCommandHelp() string
	GetCommandExit() string
	GetBottomMenuText1() []string
	GetBottomMenuText2() []string
	GetHeaderTitles() []string
	GetActionsMenuItems() []string
	GetStartedActionsMenuItems() []string
	GetStoppedActionsMenuItems() []string
	GetNonRunnableActionsMenuItems() []string
	GetName() string
	GetStatus() int
	GetCurrentStatus() int
	GetAddr() string
	SetAddr(addr string)
	GetAstart() int
	SetAstart(as int)
	//GetVer() string
	//SetVer(ver string)
	IsRunning() bool
	IsRunnable() bool
	GetStatusString() string
	GetAutoStartString() string
	GetAutoStartBool() bool
	GetAutoStartCode(astart string) int
	GetStatusCode(status string) int
	GetParam(pn string) string
	SetParam(pn string, pv string) bool
	PutJailToDb(dbname string) (bool, error)
	GetJailFromDb(dbname string, jname string) (bool, error)
	GetJailFromDbFull(dbname string, jname string) (bool, error)
	GetJailViewString() string
	UpdateJailFromDb(dbname string) (bool, error)
	Export()
	Destroy()
	OpenDestroyDialog()
	Snapshot(snapname string)
	OpenSnapshotDialog()
	Clone(jnewjname string, jnewhname string, newip string)
	OpenCloneDialog()
	Edit(astart bool, version string, ip string)
	OpenEditDialog()
	View()
	StartStop()
	GetStartCommand() string
	GetLoginCommand() string
	CreateScriptStartJail() (string, error)
	OpenActionDialog()
	ExecuteActionOnCommand(command string)
	ExecuteActionOnKey(tkey int16)
	GetSnapshots() [][2]string
	OpenSnapActionsDialog()
	DestroySnapshot(snapname string)
	OpenDestroySnapshotDialog(snapname string)
	GetAllParams() []string
}
