package reduce

import (
	"os/exec"
	"strconv"
)

func prepare(cmd *exec.Cmd) {
	cmd.Cancel = func() error { return exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid)).Run() }
}
func cleanup(cmd *exec.Cmd) { /* Windows cancellation uses taskkill on timeout. */ }
