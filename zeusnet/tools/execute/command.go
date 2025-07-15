package execute

import (
	"fmt"
	"os"
	"os/exec"
)

// Command execute command with no return
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

// CommandWithResult execute command with return
func CommandWithResult(start string, args []string) (string, error) {
	cmd := exec.Command(start, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("CombinedOutput error")
	}
	return string(output), nil
}
