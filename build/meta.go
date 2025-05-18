package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

func newMeta() meta {
	return meta{}
}

func (this *meta) init(b *build, _ *flag.FlagSet) error {
	this.build = b

	b.registerCommand("resolve", "<ref_name> <event_name> <event_number>", this.resolve)

	return this.metaConfig.readFromDefault()
}

func (this *meta) Validate() error { return nil }

type meta struct {
	build *build

	metaConfig metaConfig
}

var (
	semverPattern = regexp.MustCompile(`^v(\d+\.\d+\.\d+)(-.+)?$`)
)

func (this *meta) resolve(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return flagFail("ref_name missing")
	}
	refName := args[0]

	if len(args) < 2 {
		return flagFail("event_name missing")
	}
	eventName := args[1]

	if len(args) < 3 {
		return flagFail("event_number missing")
	}
	eventNumber := args[2]

	platforms, err := this.metaConfig.platforms()
	if err != nil {
		return err
	}
	paltformsB, err := json.Marshal(platforms)
	if err != nil {
		return err
	}

	var imageTag string
	switch eventName {
	case "pull_request":
		imageTag = "pr-" + eventNumber
	default:
		imageTag = semverPattern.ReplaceAllString(refName, "$1")
	}

	image := this.build.registry + "/" + this.build.repo.Bare()

	push := "false"
	if eventName == "release" {
		push = "true"
	} else if eventName == "pull_request" {
		n, err := strconv.Atoi(eventNumber)
		if err != nil {
			return fmt.Errorf("invalid pr number: %s", eventNumber)
		}
		pr, err := this.build.prs.byId(ctx, n)
		if err != nil {
			return err
		}
		if pr.hasLabel("test_publish") {
			push = "true"
		}
	}

	if err := this.build.appendToResolveOutput("" +
		"registry=" + this.build.registry + "\n" +
		"image=" + image + "\n" +
		"imageTag=" + imageTag + "\n" +
		"push=" + push + "\n" +
		"platforms=" + string(paltformsB) + "\n",
	); err != nil {
		return err
	}
	if err := this.build.appendToSummaryOutput("" +
		"## Task information\n" +
		"| Name | Value |\n" +
		"| - | - |\n" +
		"| Image | `" + image + "` |\n" +
		"| Should push | `" + push + "` |\n" +
		"| Platforms | `" + strings.Join(platforms, "`, `") + "` |\n",
	); err != nil {
		return err
	}

	return nil
}
