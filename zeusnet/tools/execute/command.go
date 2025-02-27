package execute

import (
	"fmt"
	"os"
	"os/exec"
)

// Command execute command
func Command(start string, args []string) error {
	cmd := exec.Command(start, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("execute command failed with error: %v", err)
	}
	return nil
}
