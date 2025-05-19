package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/google/go-github/v65/github"
)

func newRepo() (repo, error) {
	result := repo{
		packages: newPackages(),
		prs:      newPrs(),
		actions:  newActions(),
		meta:     newMeta(),
	}
	if v, ok := os.LookupEnv("GITHUB_OWNER_TYPE"); ok {
		if err := result.ownerType.Set(v); err != nil {
			return repo{}, fmt.Errorf("GITHUB_OWNER_TYPE: %w", err)
		}
	}
	if v, ok := os.LookupEnv("GITHUB_REPOSITORY"); ok {
		parts := strings.Split(v, "/")
		if len(parts) != 2 {
			return repo{}, fmt.Errorf("GITHUB_REPOSITORY: illegal github repository: %s", v)
		}
		if err := result.owner.Set(parts[0]); err != nil {
			return repo{}, fmt.Errorf("GITHUB_REPOSITORY: %w", err)
		}
		if err := result.name.Set(parts[1]); err != nil {
			return repo{}, fmt.Errorf("GITHUB_REPOSITORY: %w", err)
		}
	}
	if v, ok := os.LookupEnv("GITHUB_REPOSITORY_OWNER"); ok {
		if err := result.owner.Set(v); err != nil {
			return repo{}, fmt.Errorf("GITHUB_REPOSITORY_OWNER: %w", err)
		}
	}
	if v, ok := os.LookupEnv("GITHUB_REPO"); ok {
		if err := result.name.Set(v); err != nil {
			return repo{}, fmt.Errorf("GITHUB_REPO: %w", err)
		}
	}
	return result, nil
}

func (this *repo) init(b *build, fs *flag.FlagSet) error {
	this.build = b
	fs.Var(&this.ownerType, "ownerType", "Can be either 'user' or 'org'")
	fs.Var(&this.owner, "owner", "")
	fs.Var(&this.name, "repo", "")
	fs.StringVar(&this.registry, "registry", "ghcr.io", "")
	if err := this.packages.init(b, fs); err != nil {
		return err
	}
	if err := this.prs.init(b, fs); err != nil {
		return err
	}
	if err := this.actions.init(b, fs); err != nil {
		return err
	}
	if err := this.meta.init(b, fs); err != nil {
		return err
	}
	return nil
}

func (this *repo) Validate() error {
	if err := this.ownerType.Validate(); err != nil {
		return err
	}
	if err := this.owner.Validate(); err != nil {
		return err
	}
	if err := this.name.Validate(); err != nil {
		return err
	}
	if err := this.packages.Validate(); err != nil {
		return err
	}
	if err := this.prs.Validate(); err != nil {
		return err
	}
	if err := this.actions.Validate(); err != nil {
		return err
	}
	if err := this.meta.Validate(); err != nil {
		return err
	}
	return nil
}

type repo struct {
	build *build

	ownerType ownerType
	owner     owner
	name      repoName

	registry string

	packages packages
	prs      prs
	actions  actions
	meta     meta
}

func (this repo) Bare() string {
	return fmt.Sprintf("%s/%s", this.owner, this.name)
}

func (this repo) Url() string {
	return fmt.Sprintf("https://github.com/%s", this.Bare())
}

func (this repo) String() string {
	return fmt.Sprintf("%v:%s", this.ownerType, this.Bare())
}

func (this repo) getMeta(ctx context.Context) (*github.Repository, error) {
	v, _, err := this.build.client.Repositories.Get(ctx, this.owner.String(), this.name.String())
	if err != nil {
		return nil, fmt.Errorf("unable to get metadata for repo %s: %w", this.Bare(), err)
	}
	return v, nil
}

type ownerType uint8

func (this ownerType) String() string {
	switch this {
	case user:
		return "user"
	case org:
		return "org"
	default:
		return fmt.Sprintf("illegal-owner-type-%d", this)
	}
}

func (this ownerType) Validate() error {
	switch this {
	case user:
		return nil
	case org:
		return nil
	default:
		return fmt.Errorf("illegal-owner-type-%d", this)
	}
}

func (this *ownerType) Set(v string) error {
	switch v {
	case "user":
		*this = user
	case "org":
		*this = org
	default:
		return fmt.Errorf("unknown ownerType: %s", v)
	}
	return nil
}

const (
	user ownerType = iota
	org
)

type owner string

func (this owner) String() string {
	return string(this)
}

var ownerRegex = regexp.MustCompile("^[a-zA-Z0-9](?:[a-zA-Z0-9-]*[a-zA-Z0-9])?$")

func (this *owner) Set(v string) error {
	buf := owner(v)
	if err := buf.Validate(); err != nil {
		return err
	}
	*this = buf
	return nil
}

func (this owner) Validate() error {
	if this == "" {
		return fmt.Errorf("no owner provided")
	}
	if !ownerRegex.MatchString(string(this)) {
		return fmt.Errorf("illegal owner: %s", this)
	}
	return nil
}

type repoName string

func (this repoName) String() string {
	return string(this)
}

var repoNameRegex = regexp.MustCompile("^[a-zA-Z0-9-_.]+$")

func (this *repoName) Set(v string) error {
	buf := repoName(v)
	if err := buf.Validate(); err != nil {
		return err
	}
	*this = repoName(v)
	return nil
}

func (this repoName) Validate() error {
	if this == "" {
		return fmt.Errorf("no repo name provided")
	}
	if !repoNameRegex.MatchString(string(this)) {
		return fmt.Errorf("illegal repo name: %s", this)
	}
	return nil
}
