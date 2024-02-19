package host

import (
	"os/user"

	log "github.com/sirupsen/logrus"
)

var USE_DOAS bool = false

const DOAS_PROGRAM string = "/usr/local/bin/doas"
const CBSD_PROGRAM string = "/usr/local/bin/cbsd"
const SHELL_PROGRAM string = "/bin/sh"
const STDBUF_PROGRAM string = "/usr/bin/stdbuf"
const LOGFILE_JSTART string = "/var/log/jstart.log"

const CBSD_USER_NAME string = "cbsd"
const CBSD_DB_NAME string = "/var/db/local.sqlite"

func NeedDoAs() (bool, error) {
	curuser, err := user.Current()
	if err == nil {
		if curuser.Username == "root" {
			USE_DOAS = false
		} else {
			USE_DOAS = true
		}
		return USE_DOAS, nil
	} else {
		return false, err
	}
}

func LogError(strerr string, err error) {
	log.Errorf(strerr+": %w", err)
}

func GetCbsdDbConnString(readwrite bool) string {
	cbsdUser, err := user.Lookup(CBSD_USER_NAME)
	if err != nil {
		panic(err)
	}
	if readwrite {
		return "file:" + cbsdUser.HomeDir + CBSD_DB_NAME + "?mode=rw"
	} else {
		return "file:" + cbsdUser.HomeDir + CBSD_DB_NAME + "?mode=ro"
	}
}
