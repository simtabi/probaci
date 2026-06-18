package docker

import (
	"os"
	"strconv"
)

// currentUserSpec returns "uid:gid" for the current process, used to run
// containers as a non-root host user on Linux.
func currentUserSpec() string {
	uid := os.Getuid()
	gid := os.Getgid()
	if uid < 0 {
		uid = 1000
	}
	if gid < 0 {
		gid = 1000
	}
	return strconv.Itoa(uid) + ":" + strconv.Itoa(gid)
}
