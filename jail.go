package main

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

type Jail struct {
	Jname    string
	Ip4_addr string
	Status   int
	Astart   int
	Ver      string
}

var strStatus = []string{"Off", "On", "Slave", "Unknown(3)", "Unknown(4)", "Unknown(5)"}
var strAutoStart = []string{"Off", "On"}

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

func New() *Jail {
	res := &Jail{
		Jname:    "",
		Ip4_addr: "",
		Status:   0,
		Astart:   0,
		Ver:      "",
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

	jails_list_query := "SELECT jname,ip4_addr,status,astart,ver FROM jails"
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
