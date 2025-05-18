package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type metaConfig struct {
	Name string   `json:"name,omitempty"`
	Slug string   `json:"slug,omitempty"`
	Url  string   `json:"url,omitempty"`
	Arch []string `json:"arch,omitempty"`
}

func (this *metaConfig) read(in io.Reader) error {
	dec := yaml.NewDecoder(in)
	return dec.Decode(this)
}

func (this *metaConfig) readFromFile(fn string) error {
	f, err := os.Open(fn)
	if err != nil {
		return fmt.Errorf("cannot open %q: %w", fn, err)
	}
	defer func() {
		_ = f.Close()
	}()
	if err := this.read(f); err != nil {
		return fmt.Errorf("cannot parse %q: %w", fn, err)
	}
	return nil
}

func (this *metaConfig) readFromDefault() error {
	return this.readFromFile("config.yaml")
}

func (this *metaConfig) platforms() ([]string, error) {
	result := make([]string, len(this.Arch))
	for i, arch := range this.Arch {
		result[i] = haArchToOciPlatform(arch)
	}
	return result, nil
}

func haArchToOciArch(in string) string {
	switch strings.ToLower(in) {
	case "amd64":
		return "amd64"
	case "aarch64":
		return "arm64"
	case "armv7":
		return "arm/v7"
	case "armv6":
		return "arm/v6"
	default:
		return in
	}
}

func haArchToOciPlatform(in string) string {
	return "linux/" + haArchToOciArch(in)
}
