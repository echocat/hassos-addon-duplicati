package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	log "github.com/echocat/slf4g"
	"github.com/mholt/archives"
)

const (
	processPort              = 8300
	processExecutableEnvVar  = "PROCESS_EXECUTABLE"
	processExecutableDefault = "/opt/duplicati/duplicati-server"

	customReleaseTargetDefault     = "/opt/duplicati/custom"
	customReleaseTargetEnvVar      = "CUSTOM_RELEASE_TARGET"
	customReleaseExecutableDefault = "duplicati-server"
	customReleaseExecutableEnvVar  = "CUSTOM_RELEASE_EXECUTABLE"
)

var (
	background = context.Background()
)

func newProcess(opts options) (result *process, err error) {
	result = &process{
		logger: log.GetLogger("duplicati"),
	}

	executable := processExecutable()
	if opts.customRelease != "" {
		executable, err = downloadCustomProcess(opts.customRelease)
		if err != nil {
			return nil, err
		}
	}

	result.cmd = exec.Command(executable,
		"--webservice-disable-https=True",
		"--log-file=/dev/stdout",
		"--webservice-interface=any",
		"--webservice-allowed-hostnames=*",
		"--server-datafolder=/data",
		"--require-db-encryption-key=True",
		fmt.Sprintf("--webservice-timezone=%s", opts.timezone),
		fmt.Sprintf("--log-level=%v", opts.logLevel),
		fmt.Sprintf("--webservice-port=%d", processPort),
	)
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

	return result, nil
}

func downloadCustomProcess(from string) (string, error) {
	logger := log.With("customRelease", from)
	logger.Info("downloading custom release, this could take a few minutes...")
	rsp, err := http.Get(from)
	if err != nil {
		return "", fmt.Errorf("cannot download custom release from URL %q: %w", from, err)
	}
	defer func() {
		_ = rsp.Body.Close()
	}()
	if rsp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("cannot download custom release from URL %q: %d - %s", from, rsp.StatusCode, rsp.Status)
	}
	fBuf, err := os.CreateTemp("", "custom-release-*.zip")
	if err != nil {
		return "", fmt.Errorf("cannot buffer custom release %q locally: %w", from, err)
	}
	defer func() {
		_ = fBuf.Close()
	}()
	if _, err := io.Copy(fBuf, rsp.Body); err != nil {
		return "", fmt.Errorf("cannot buffer custom release %q locally to %q: %w", from, fBuf.Name(), err)
	}
	if _, err := fBuf.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("cannot read buffer (%q) of custom release %q: %w", fBuf.Name(), from, err)
	}

	logger.Info("extracting custom release, this could take a few minutes...")

	target, err := filepath.Abs(customReleaseTarget())
	if err != nil {
		return "", fmt.Errorf("cannot place custom release %q: %w", from, err)
	}

	executableName := customReleaseExecutable()
	executable, err := filepath.Abs(filepath.Join(target, executableName))
	if err != nil {
		return "", fmt.Errorf("cannot use executable of custom release %q: %w", from, err)
	}
	executableFound := false

	format, stream, err := archives.Identify(background, fBuf.Name(), fBuf)
	if err != nil {
		return "", fmt.Errorf("cannot identifiy type of buffered (at %q) custom release %q: %w", fBuf.Name(), from, err)
	}
	if ex, ok := format.(archives.Extractor); ok {
		if err := os.RemoveAll(target); err != nil {
			return "", fmt.Errorf("cannot prepare custom release target %q: %w", target, err)
		}
		if err := ex.Extract(background, stream, func(ctx context.Context, in archives.FileInfo) error {
			if in.IsDir() {
				return nil
			}

			fIn, err := in.Open()
			if err != nil {
				return err
			}
			defer func() {
				_ = fIn.Close()
			}()

			fInStat, err := fIn.Stat()
			if err != nil {
				return err
			}

			i := strings.IndexByte(in.NameInArchive, '/')
			base, fName := in.NameInArchive[:i+1], in.NameInArchive[i+1:]
			if !strings.HasPrefix(base, "duplicati-") {
				return fmt.Errorf("does not comply with expected format")
			}

			targetFn := filepath.Join(target, fName)
			_ = os.MkdirAll(filepath.Dir(targetFn), 0755)
			if targetFn == executable {
				executableFound = true
			}

			fOut, err := os.OpenFile(targetFn, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fInStat.Mode())
			if err != nil {
				return err
			}
			defer func() {
				_ = fOut.Close()
			}()

			if _, err := io.Copy(fOut, fIn); err != nil {
				return err
			}

			logger.With("file", targetFn).
				With("size", in.Size()).
				Debug("file of custom release extracted")

			return nil
		}); err != nil {
			return "", fmt.Errorf("cannot extract custom release %q: %w", from, err)
		}
	} else {
		return "", fmt.Errorf("custom release %q does not comply with expected format", from)
	}

	if !executableFound {
		return "", fmt.Errorf("custom release %q does not contain %q", from, executableName)
	}

	logger.Info("custom release downloaded and extracted")

	return executable, nil
}

func customReleaseTarget() string {
	if v := os.Getenv(customReleaseTargetEnvVar); v != "" {
		return v
	}
	return customReleaseTargetDefault
}

func customReleaseExecutable() string {
	if v := os.Getenv(customReleaseExecutableEnvVar); v != "" {
		return v
	}
	return customReleaseExecutableDefault
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
