package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	optionsFileDefault = "/data/options.json"
	optionsFileEnvVar  = "OPTIONS_FILE"
	secretsFileDefault = "/data/secrets.json"
	secretsFileEnvVar  = "SECRETS_FILE"
)

type options struct {
	properties properties

	webservicePassword      string
	webservicePreAuthTokens string
	settingsEncryptionKey   string
}

type optionsPayload struct {
	Properties map[string]any `json:"properties"`
}

type secretsPayload struct {
	WebservicePassword      string `json:"webservicePassword"`
	WebservicePreAuthTokens string `json:"webservicePreAuthTokens"`
	SettingsEncryptionKey   string `json:"settingsEncryptionKey"`
}

func (opt *options) set(payload optionsPayload) error {
	readProperties := properties{}
	if err := readProperties.setMap(payload.Properties); err != nil {
		return fmt.Errorf("could decode properties: %v", err)
	}
	opt.properties = readProperties.merge(defaultProperties)
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
