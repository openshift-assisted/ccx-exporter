package e2e

import (
	"bytes"
	"fmt"
	"io"

	"github.com/vladimirvivien/gexe/exec"
)

func runCommand(command string, env []string) error {
	stdout := bytes.NewBufferString("")
	stderr := bytes.NewBufferString("")

	proc := exec.NewProc(command)
	proc.Command().Stdout = stdout
	proc.Command().Stderr = stderr

	if len(env) > 0 {
		proc.Command().Env = env
	}

	proc.Start().Wait()

	err := proc.Err()
	if err != nil {
		sOutput, _ := io.ReadAll(stdout)
		sErr, _ := io.ReadAll(stderr)

		return fmt.Errorf("failed to run command (%w): stdout:%s stderr:%s", err, string(sOutput), string(sErr))
	}

	return nil
}

func runMakefileCommand(target string, keyValues map[string]string) error {
	command := fmt.Sprintf("make -C ../.. %s", target)
	for key, value := range keyValues {
		command = fmt.Sprintf("%s %s=%s", command, key, value)
	}

	return runCommand(command, nil)
}
