package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/go-github/v65/github"
)

func newBuild() (build, error) {
	r, err := newRepo()
	if err != nil {
		return build{}, err
	}

	result := build{
		repo:        r,
		waitTimeout: time.Second * 3,
		commands:    make(map[string]command),

		resolveOutput: "var/resolve-output",
		summaryOutput: "var/summary-output",
	}

	if v, ok := os.LookupEnv("GITHUB_OUTPUT"); ok {
		result.resolveOutput = v
	}
	if v, ok := os.LookupEnv("GITHUB_STEP_SUMMARY"); ok {
		result.summaryOutput = v
	}

	return result, nil
}

type build struct {
	repo

	waitTimeout   time.Duration
	resolveOutput string
	summaryOutput string

	client *github.Client

	commands map[string]command
}

type command struct {
	f     func(context.Context, []string) error
	usage string
}

func (this *build) init(fs *flag.FlagSet) error {
	this.client = github.NewClient(nil).
		WithAuthToken(os.Getenv("GITHUB_TOKEN"))

	fs.DurationVar(&this.waitTimeout, "wait-timeout", this.waitTimeout, "")

	return this.repo.init(this, fs)
}

func (this *build) Validate() error {
	if err := this.repo.Validate(); err != nil {
		return err
	}
	return nil
}

func (this *build) registerCommand(name, usage string, action func(context.Context, []string) error) {
	this.commands[name] = command{action, usage}
}

func (this *build) flagUsage(fs *flag.FlagSet, reasonMsg string, args ...any) {
	w := fs.Output()
	_, _ = fmt.Fprint(w, `Usage of .build:
`)
	if reasonMsg != "" {
		_, _ = fmt.Fprintf(w, "Error: %s\n", fmt.Sprintf(reasonMsg, args...))
	}
	_, _ = fmt.Fprintf(w, "Syntax: %s [flags] <command> [commandSpecificArgs]\nCommands:\n", fs.Name())
	for n, c := range this.commands {
		_, _ = fmt.Fprintf(w, "\t%s %s\n", n, c.usage)
	}
	_, _ = fmt.Fprint(w, "Flags:\n")
	fs.PrintDefaults()
}

func (this *build) appendTo(fn, fnType, msg string, args ...any) error {
	_ = os.MkdirAll(filepath.Dir(fn), 0755)
	f, err := os.OpenFile(fn, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("cannot open %s file %q: %w", fnType, fn, err)
	}
	defer func() {
		_ = f.Close()
	}()
	if _, err := fmt.Fprintf(f, msg, args...); err != nil {
		return fmt.Errorf("cannot write %s file %q: %w", fnType, fn, err)
	}

	_ = f.Close()
	content, err := os.ReadFile(fn)
	if err != nil {
		return fmt.Errorf("cannot read %s file %q: %w", fnType, fn, err)
	}
	fmt.Printf("+++ Content of %q+++\n%s\n--- Content of %q---\n", fn, string(content), fn)

	return nil
}
func (this *build) appendToResolveOutput(msg string, args ...any) error {
	return this.appendTo(this.resolveOutput, "resolve output", msg, args...)
}

func (this *build) appendToSummaryOutput(msg string, args ...any) error {
	return this.appendTo(this.summaryOutput, "summary output", msg, args...)
}
