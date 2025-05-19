package main

import (
	"context"
	"encoding/json"
	"flag"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func newMeta() meta {
	return meta{}
}

func (this *meta) init(b *build, _ *flag.FlagSet) error {
	this.build = b

	b.registerCommand("resolve", "<ref_name> <event_name> <event_number>", this.resolve)
	b.registerCommand("resolve-build", "<platform> <ref_name> <event_name> <event_number>", this.resolveBuild)

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

func (this *meta) getRefName(fromArgs []string, i int) (string, error) {
	if len(fromArgs) < i+1 {
		return "", flagFail("ref_name missing")
	}
	return fromArgs[i], nil
}

func (this *meta) getEventName(fromArgs []string, i int) (string, error) {
	if len(fromArgs) < i+1 {
		return "", flagFail("event_name missing")
	}
	return fromArgs[i], nil
}

func (this *meta) getEventNumber(fromArgs []string, i int) (int, error) {
	if len(fromArgs) < i+1 {
		return 0, flagFail("event_number missing")
	}

	if fromArgs[i] == "" {
		return 0, nil
	}

	v, err := strconv.Atoi(fromArgs[i])
	if err != nil {
		return 0, flagFail("illegal event_number: %q", fromArgs[i])
	}
	return v, nil
}
func (this *meta) getPlatform(fromArgs []string, i int) (string, error) {
	if len(fromArgs) < i+1 {
		return "", flagFail("platform missing")
	}

	return fromArgs[i], nil
}

func (this *meta) getBasics(fromArgs []string, startI int) (refName, eventName string, eventNumber int, err error) {
	if refName, err = this.getRefName(fromArgs, startI); err != nil {
		return "", "", 0, err
	}
	if eventName, err = this.getEventName(fromArgs, startI+1); err != nil {
		return "", "", 0, err
	}
	if eventNumber, err = this.getEventNumber(fromArgs, startI+2); err != nil {
		return "", "", 0, err
	}
	return refName, eventName, eventNumber, nil
}

func (this *meta) resolve(ctx context.Context, args []string) error {
	refName, eventName, eventNumber, err := this.getBasics(args, 0)
	if err != nil {
		return err
	}

	platforms, err := this.metaConfig.platforms()
	if err != nil {
		return err
	}
	paltformsB, err := json.Marshal(platforms)
	if err != nil {
		return err
	}

	imageTag, image := this.resolveImage(eventName, eventNumber, refName)

	push := "false"
	if eventName == "release" {
		push = "true"
	} else if eventName == "pull_request" {
		pr, err := this.build.prs.byId(ctx, eventNumber)
		if err != nil {
			return err
		}
		if pr.hasLabel("test_publish") {
			push = "true"
		}
	}

	repoMeta, err := this.build.repo.getMeta(ctx)
	if err != nil {
		return err
	}
	licStr := ""
	lic := repoMeta.GetLicense()
	if lic != nil {
		licStr = lic.GetName()
	}

	if err := this.build.appendToResolveOutput("" +
		"registry=" + this.build.registry + "\n" +
		"image=" + image + "\n" +
		"imageTag=" + imageTag + "\n" +
		"push=" + push + "\n" +
		"platforms=" + string(paltformsB) + "\n" +
		"annotations<<EOF\n" +
		"org.opencontainers.image.url=\"https://github.com/" + this.build.repo.Bare() + "\"\n" +
		"org.opencontainers.image.source=\"https://github.com/" + this.build.repo.Bare() + "\"\n" +
		"org.opencontainers.image.description=" + strconv.Quote(repoMeta.GetDescription()) + "\n" +
		"org.opencontainers.image.created=\"" + time.Now().Format(time.RFC3339) + "\"\n" +
		"org.opencontainers.image.title=" + strconv.Quote(this.metaConfig.Name) + "\n" +
		"org.opencontainers.image.version=" + strconv.Quote(imageTag) + "\n" +
		"org.opencontainers.image.licenses=" + strconv.Quote(licStr) + "\n" +
		"EOF\n",
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

func (this *meta) resolveBuild(_ context.Context, args []string) error {
	platform, err := this.getPlatform(args, 0)
	if err != nil {
		return err
	}
	refName, eventName, eventNumber, err := this.getBasics(args, 1)
	if err != nil {
		return err
	}

	imageTag, image := this.resolveImage(eventName, eventNumber, refName)

	if err := this.build.appendToResolveOutput("" +
		"image=" + image + "\n" +
		"imageTag=" + imageTag + "\n" +
		"platformToken=" + strings.ReplaceAll(platform, "/", "-") + "\n" +
		"labels<<EOF\"\n" +
		"io.hass.type=\"addon\"\n" +
		"io.hass.version=" + strconv.Quote(imageTag) + "\n" +
		"io.hass.arch=\"" + ociPlatformToHaArch(platform) + "\"\n" +
		"io.hass.name=" + strconv.Quote(this.metaConfig.Name) + "\n" +
		"io.hass.description=" + strconv.Quote(this.metaConfig.Description) + "\n" +
		"io.hass.url=\"https://github.com/" + this.build.repo.Bare() + "\"\n" +
		"EOF\n",
	); err != nil {
		return err
	}

	return nil
}

func (this *meta) resolveImage(eventName string, eventNumber int, refName string) (tag, image string) {
	switch eventName {
	case "pull_request":
		tag = "pr-" + strconv.Itoa(eventNumber)
	default:
		tag = semverPattern.ReplaceAllString(refName, "$1")
	}
	return tag, this.build.registry + "/" + this.build.repo.Bare()
}
