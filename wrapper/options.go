package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/echocat/slf4g/level"
	"github.com/echocat/slf4g/native"
)

const (
	optionsFileDefault    = "/data/options.json"
	optionsFileEnvVar     = "OPTIONS_FILE"
	secretsFileDefault    = "/data/secrets.json"
	secretsFileEnvVar     = "SECRETS_FILE"
	haInfoUrlDefault      = "http://supervisor/info"
	haInfoUrlEnvVar       = "HA_INFO_URL"
	supervisorTokenEnvVar = "SUPERVISOR_TOKEN"
)

type options struct {
	gui             optionsGui
	customRelease   string
	logLevel        optionsLogLevel
	wrapperLogLevel optionsWrapperLogLevel
	timezone        string

	webservicePassword      string
	webservicePreAuthTokens string
	settingsEncryptionKey   string
}

type optionsPayload struct {
	Gui             options                `json:"gui,omitempty"`
	CustomRelease   string                 `json:"custom_release,omitempty"`
	LogLevel        optionsLogLevel        `json:"log_level,omitempty"`
	WrapperLogLevel optionsWrapperLogLevel `json:"wrapper_log_level,omitempty"`
}

type secretsPayload struct {
	WebservicePassword      string `json:"webservicePassword"`
	WebservicePreAuthTokens string `json:"webservicePreAuthTokens"`
	SettingsEncryptionKey   string `json:"settingsEncryptionKey"`
}

type haInfoPayload struct {
	Data haInfoPayloadData `json:"data"`
}

type haInfoPayloadData struct {
	Timezone string `json:"timezone"`
}

func (opt *options) set(payload optionsPayload) error {
	opt.customRelease = payload.CustomRelease
	opt.logLevel = payload.LogLevel
	opt.wrapperLogLevel = payload.WrapperLogLevel
	return nil
}

func (opt *options) setSecrets(payload secretsPayload) error {
	opt.webservicePassword = payload.WebservicePassword
	opt.webservicePreAuthTokens = payload.WebservicePreAuthTokens
	opt.settingsEncryptionKey = payload.SettingsEncryptionKey
	return nil
}

func (opt *options) getSecrets() (result secretsPayload) {
	result.WebservicePassword = opt.webservicePassword
	result.WebservicePreAuthTokens = opt.webservicePreAuthTokens
	result.SettingsEncryptionKey = opt.settingsEncryptionKey
	return result
}

func (opt *options) setHaInfo(payload haInfoPayload) error {
	opt.timezone = payload.Data.Timezone
	if opt.timezone == "" {
		opt.timezone = "Etc/UTC"
	} else if _, err := time.LoadLocation(opt.timezone); err != nil {
		opt.timezone = "Etc/UTC"
	}
	return nil
}

func (opt *options) readFrom(r io.Reader) error {
	dec := json.NewDecoder(r)
	var buf optionsPayload
	if err := dec.Decode(&buf); err != nil {
		return fmt.Errorf("could not decode options: %w", err)
	}

	if err := opt.set(buf); err != nil {
		return fmt.Errorf("could not decode options: %w", err)
	}

	return nil
}

func (opt *options) readFromFile(fn string) error {
	f, err := os.Open(fn)
	if err != nil {
		return fmt.Errorf("could not open options file %q: %w", fn, err)
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(f)
	if err := opt.readFrom(f); err != nil {
		return fmt.Errorf("could not read options file %q: %w", fn, err)
	}
	return nil
}

func (opt *options) readAllDefaults() error {
	if err := opt.readFromDefaultFile(); err != nil {
		return err
	}
	if err := opt.ensureSecretsFromDefaultFile(); err != nil {
		return err
	}
	if err := opt.readHaInfoFromDefaultUrl(); err != nil {
		return err
	}
	return nil
}

func (opt *options) readFromDefaultFile() error {
	return opt.readFromFile(opt.defaultFile())
}

func (opt *options) defaultFile() string {
	if v := os.Getenv(optionsFileEnvVar); v != "" {
		return v
	}
	return optionsFileDefault
}

func (opt *options) ensureSecretsFrom(r io.Reader) (modified bool, err error) {
	if r != nil {
		dec := json.NewDecoder(r)
		var buf secretsPayload
		if err := dec.Decode(&buf); err != nil {
			return false, fmt.Errorf("could not decode secrets: %w", err)
		}
		if err := opt.setSecrets(buf); err != nil {
			return false, fmt.Errorf("could not decode secrets: %w", err)
		}
	}

	if len(opt.webservicePassword) < 10 {
		if opt.webservicePassword, err = generateSecretString(); err != nil {
			return false, fmt.Errorf("could not generate webservicePassword: %w", err)
		}
		modified = true
	}
	if len(opt.webservicePreAuthTokens) < 10 {
		if opt.webservicePreAuthTokens, err = generateSecretString(); err != nil {
			return false, fmt.Errorf("could not generate webservicePreAuthTokens: %w", err)
		}
		modified = true
	}
	if len(opt.settingsEncryptionKey) < 10 {
		if opt.settingsEncryptionKey, err = generateSecretString(); err != nil {
			return false, fmt.Errorf("could not generate settingsEncryptionKey: %w", err)
		}
		modified = true
	}

	return modified, nil
}

func (opt *options) writeSecretsTo(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(opt.getSecrets())
}

func (opt *options) ensureSecretsFromFile(fn string) error {
	f, err := os.Open(fn)
	if os.IsNotExist(err) {
		// ignore
	} else if err != nil {
		return fmt.Errorf("could not open secrets file %q: %w", fn, err)
	}
	defer func(f *os.File) {
		if f != nil {
			_ = f.Close()
		}
	}(f)
	var rf io.Reader
	if f != nil {
		rf = f
	}
	modified, err := opt.ensureSecretsFrom(rf)
	if err != nil {
		return fmt.Errorf("could not ensure secrets file %q: %w", fn, err)
	}
	if modified {
		if f != nil {
			_ = f.Close()
		}

		_ = os.MkdirAll(filepath.Dir(fn), 0700)
		fw, err := os.OpenFile(fn, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			return fmt.Errorf("could not open secrets file %q for write: %w", fn, err)
		}
		defer func(fw *os.File) {
			if fw != nil {
				_ = fw.Close()
			}
		}(fw)
		if err := opt.writeSecretsTo(fw); err != nil {
			return fmt.Errorf("could not write secrets file %q: %w", fn, err)
		}
	}
	return nil
}

func (opt *options) ensureSecretsFromDefaultFile() error {
	return opt.ensureSecretsFromFile(opt.defaultSecretsFile())
}

func (opt *options) defaultSecretsFile() string {
	if v := os.Getenv(secretsFileEnvVar); v != "" {
		return v
	}
	return secretsFileDefault
}

func (opt *options) readHaInfoFrom(r io.Reader) error {
	dec := json.NewDecoder(r)
	var buf haInfoPayload
	if err := dec.Decode(&buf); err != nil {
		return fmt.Errorf("could not decode home assisstant info: %w", err)
	}

	if err := opt.setHaInfo(buf); err != nil {
		return fmt.Errorf("could not decode home assisstant info: %w", err)
	}

	return nil
}

func (opt *options) readHaInfoFile(url, accessToken string) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("could not create request for home assistant info %q: %w", url, err)
	}
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}

	rsp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not execute request to home assistant info %q: %w", url, err)
	}
	defer func() {
		_ = rsp.Body.Close()
	}()
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("could not execute request to home assistant info %q: got %d - %s", url, rsp.StatusCode, rsp.Status)
	}

	if err := opt.readHaInfoFrom(rsp.Body); err != nil {
		return fmt.Errorf("could not parse response of home assistant info %q: %v", url, err)
	}

	return nil
}

func (opt *options) readHaInfoFromDefaultUrl() error {
	return opt.readHaInfoFile(opt.defaultHaInfoUrl(), os.Getenv(supervisorTokenEnvVar))
}

func (opt *options) defaultHaInfoUrl() string {
	if v := os.Getenv(haInfoUrlEnvVar); v != "" {
		return v
	}
	return haInfoUrlDefault
}

type optionsGui string

func (ol *optionsGui) UnmarshalText(text []byte) error {
	*ol = optionsGui(optionsGui(text).String())
	return nil
}

func (ol optionsGui) MarshalText() ([]byte, error) {
	return []byte(ol.String()), nil
}

func (ol optionsGui) String() string {
	switch strings.ToLower(string(ol)) {
	case "ngclient":
		return "ngclient"
	default:
		return "ngax"
	}
}

func (ol optionsGui) initPath() string {
	return "/" + ol.String() + "/"
}

type optionsLogLevel string

func (ol *optionsLogLevel) UnmarshalText(text []byte) error {
	*ol = optionsLogLevel(optionsLogLevel(text).String())
	return nil
}

func (ol optionsLogLevel) MarshalText() ([]byte, error) {
	return []byte(ol.String()), nil
}

func (ol optionsLogLevel) String() string {
	switch strings.ToLower(string(ol)) {
	case "error":
		return "Error"
	case "warning", "warn":
		return "Warning"
	case "verbose", "debug":
		return "Verbose"
	case "profiling", "trace":
		return "Profiling"
	default:
		return "Information"
	}
}

type optionsWrapperLogLevel level.Level

func (ol *optionsWrapperLogLevel) UnmarshalText(text []byte) error {
	v, err := native.DefaultProvider.GetLevelNames().ToLevel(string(text))
	if err != nil {
		return err
	}
	*ol = optionsWrapperLogLevel(v)
	return nil
}

func (ol optionsWrapperLogLevel) MarshalText() ([]byte, error) {
	v, err := native.DefaultProvider.GetLevelNames().ToName(level.Level(ol))
	if err != nil {
		return nil, err
	}
	return []byte(v), nil
}

func (ol optionsWrapperLogLevel) String() string {
	text, err := ol.MarshalText()
	if err != nil {
		return fmt.Sprintf("ERR-%v", err)
	}
	return string(text)
}

func (ol optionsWrapperLogLevel) get() level.Level {
	return level.Level(ol)
}
