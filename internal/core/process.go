package core

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
)

type ProcessResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

func RunProcess(command string, args []string, cwd string, extraEnv map[string]string, allowFailure bool) (ProcessResult, error) {
	cmd := exec.Command(command, args...)
	cmd.Dir = cwd
	env := append([]string{}, os.Environ()...)
	env = append(env, "GIT_TERMINAL_PROMPT=0")
	for key, value := range extraEnv {
		env = append(env, key+"="+value)
	}
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	result := ProcessResult{Stdout: strings.TrimRight(stdout.String(), "\r\n"), Stderr: strings.TrimSpace(stderr.String())}
	if err == nil {
		return result, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		if allowFailure {
			return result, nil
		}
		message := result.Stderr
		if message == "" {
			message = result.Stdout
		}
		return result, NewError(ErrRepositoryFailed, command+" failed: "+message, map[string]any{"command": command, "args": args, "exitCode": result.ExitCode})
	}
	code := ErrGeneric
	if command == "git" {
		code = ErrRepositoryFailed
	}
	return result, WrapError(code, "Cannot start "+command+": "+err.Error(), err, map[string]any{"command": command})
}
