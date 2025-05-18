package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	log "github.com/echocat/slf4g"
)

const (
	processPort              = 8300
	processExecutableEnvVar  = "PROCESS_EXECUTABLE"
	processExecutableDefault = "/opt/duplicati/duplicati-server"
)

func newProcess(opts options) (result *process, err error) {
	result = &process{
		logger: log.GetLogger("duplicati"),
	}

	executable := processExecutable()
	args := opts.properties.
		with("webservice-port", strconv.Itoa(processPort)).
		toArguments()

	result.cmd = exec.Command(executable, args...)
	result.cmd.Env = []string{
		"PATH=" + filepath.Dir(executable) + ":" + os.Getenv("PATH"),
		"DUPLICATI__WEBSERVICE_PASSWORD=" + opts.webservicePassword,
		"DUPLICATI__WEBSERVICE_PRE_AUTH_TOKENS=" + opts.webservicePreAuthTokens,
		"SETTINGS_ENCRYPTION_KEY=" + opts.settingsEncryptionKey,
	}

	result.cmd.Stdout = os.Stdout
	result.cmd.Stderr = os.Stderr
	if err = result.cmd.Start(); err != nil {
		return nil, fmt.Errorf("cannot start process %v: %w", result.cmd, err)
	}

	return result, err
}

type process struct {
	logger log.Logger
	cmd    *exec.Cmd
}

func (p *process) signal(sig os.Signal) {
	cmd := p.cmd
	if cmd == nil {
		return
	}
	ps := cmd.ProcessState
	if ps == nil {
		return
	}
	if ps.Exited() {
		return
	}
	proc := cmd.Process
	if proc == nil {
		return
	}
	if err := cmd.Process.Signal(sig); err != nil {
		p.logger.Warnf("cannot send signal to process %v (#%d): %v", cmd, ps.Pid(), err)
	}
}

func (p *process) wait() (int, error) {
	cmd := p.cmd
	if cmd == nil {
		return 0, nil
	}
	err := cmd.Wait()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				return status.ExitStatus(), nil
			}
		}
		return 1, err
	} else {
		if status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok {
			return status.ExitStatus(), nil
		}
		return 0, nil
	}
}

func (p *process) Close() (rErr error) {
	defer p.signal(syscall.SIGTERM)
	return nil
}

func processExecutable() string {
	if v := os.Getenv(processExecutableEnvVar); v != "" {
		return v
	}
	return processExecutableDefault
}
