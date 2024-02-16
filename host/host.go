package host

import (
	"os/user"
)

var USE_DOAS bool = false

const DOAS_PROGRAM string = "/usr/local/bin/doas"
const CBSD_PROGRAM string = "/usr/local/bin/cbsd"

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
