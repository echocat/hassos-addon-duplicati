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

	v, err := strconv.Atoi(strings.SplitN(fromArgs[i], "/", 2)[0])
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
	if eventName, err = this.getEventName(fromArgs, startI); err != nil {
		return "", "", 0, err
	}
	if eventNumber, err = this.getEventNumber(fromArgs, startI); err != nil {
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
		"echo \"annotations<<EOF\"\n" +
		"echo 'org.opencontainers.image.url=\"https://github.com/" + this.build.repo.Bare() + "\"'\n" +
		"echo 'org.opencontainers.image.source=\"https://github.com/" + this.build.repo.Bare() + "\"'\n" +
		"echo 'org.opencontainers.image.description=" + strconv.Quote(repoMeta.GetDescription()) + "'\n" +
		"echo 'org.opencontainers.image.created=\"" + time.Now().Format(time.RFC3339) + "\"'\n" +
		"echo 'org.opencontainers.image.title=" + strconv.Quote(this.metaConfig.Name) + "'\n" +
		"echo 'org.opencontainers.image.version=" + strconv.Quote(imageTag) + "'\n" +
		"echo 'org.opencontainers.image.licenses=" + strconv.Quote(licStr) + "'\n" +
		"echo \"EOF\"\n",
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
		"echo \"labels<<EOF\"\n" +
		"echo 'io.hass.type=\"addon\"'\n" +
		"echo 'io.hass.version=" + strconv.Quote(imageTag) + "'\n" +
		"echo 'io.hass.arch=\"" + ociPlatformToHaArch(platform) + "\"'\n" +
		"echo 'io.hass.name=" + strconv.Quote(this.metaConfig.Name) + "'\n" +
		"echo 'io.hass.description=" + strconv.Quote(this.metaConfig.Description) + "'\n" +
		"echo 'io.hass.url=\"https://github.com/" + this.build.repo.Bare() + "\"'\n" +
		"echo \"EOF\"\n",
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
