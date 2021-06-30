package goyek

import (
	"os/exec"
	"strings"
)

// Cmd is like exec.Command, but it assigns tf's context
// and assigns Stdout and Stderr to tf's output.
func (tf *TF) Cmd(name string, args ...string) *exec.Cmd {
	cmdStr := strings.Join(append([]string{name}, args...), " ")
	tf.Logf("Cmd: %s", cmdStr)

	cmd := exec.CommandContext(tf.Context(), name, args...) //nolint:gosec // yes, this runs a subprocess
	cmd.Stderr = tf.Output().Messaging
	cmd.Stdout = tf.Output().Standard
	return cmd
}
