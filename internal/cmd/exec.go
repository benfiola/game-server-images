package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
)

type CmdOpts struct {
	Cwd string
	Env []string
}

func Capture(ctx context.Context, cmd ...string) (string, error) {
	return CaptureWithOpts(ctx, CmdOpts{}, cmd...)
}

func CaptureWithOpts(ctx context.Context, opts CmdOpts, cmd ...string) (string, error) {
	command := exec.CommandContext(ctx, cmd[0], cmd[1:]...)
	var stdout bytes.Buffer
	command.Stdout = &stdout
	var stderr bytes.Buffer
	command.Stderr = &stderr

	if opts.Cwd != "" {
		command.Dir = opts.Cwd
	}

	if len(opts.Env) > 0 {
		command.Env = opts.Env
	}

	if err := command.Run(); err != nil {
		return "", fmt.Errorf("command failed: %w, stderr: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

func Stream(ctx context.Context, cmd ...string) error {
	return StreamWithOpts(ctx, CmdOpts{}, cmd...)
}

func StreamWithOpts(ctx context.Context, opts CmdOpts, cmd ...string) error {
	command := exec.CommandContext(ctx, cmd[0], cmd[1:]...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Stdin = os.Stdin

	if opts.Cwd != "" {
		command.Dir = opts.Cwd
	}

	if len(opts.Env) > 0 {
		command.Env = opts.Env
	}

	if err := command.Run(); err != nil {
		return fmt.Errorf("command failed: %w", err)
	}
	return nil
}
