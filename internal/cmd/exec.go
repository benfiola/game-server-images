package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
)

func Capture(ctx context.Context, cmd ...string) (string, error) {
	command := exec.CommandContext(ctx, cmd[0], cmd[1:]...)
	var stdout bytes.Buffer
	command.Stdout = &stdout
	var stderr bytes.Buffer
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		return "", fmt.Errorf("command failed: %w, stderr: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

func Stream(ctx context.Context, cmd ...string) error {
	command := exec.Command(cmd[0], cmd[1:]...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	if err := command.Run(); err != nil {
		return fmt.Errorf("command failed: %w", err)
	}
	return nil
}
