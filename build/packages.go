package main

import (
	"context"
	"flag"
	"fmt"
	"iter"
	"log"
	"slices"

	"github.com/google/go-github/v65/github"
)

func newPackages() packages {
	return packages{}
}

func (this *packages) init(b *build, _ *flag.FlagSet) error {
	this.build = b
	b.registerCommand("delete-image-tag", "<tag> [<tag> ..]", this.deleteVersionsWithTags)

	return nil
}

func (this *packages) Validate() error { return nil }

type packages struct {
	build *build
}

func (this *packages) deleteVersionsWithTags(ctx context.Context, tags []string) error {
	if len(tags) == 0 {
		return flagFail("no tags specified")
	}

	for candidate, err := range this.versionsWithAtLeastOneTag(ctx, tags) {
		if err != nil {
			return err
		}

		if err := candidate.delete(ctx); err != nil {
			log.Println("WARN", err)
		} else {
			fmt.Printf("INFO successfully deleted package version %v", candidate)
		}
	}

	return nil
}

func (this *packages) versions(ctx context.Context) iter.Seq2[*packageVersion, error] {
	var m func(context.Context, string, string, string, *github.PackageListOptions) ([]*github.PackageVersion, *github.Response, error)
	if this.build.ownerType == user {
		m = this.build.client.Users.PackageGetAllVersions
	} else {
		m = this.build.client.Organizations.PackageGetAllVersions
	}
	return func(yield func(*packageVersion, error) bool) {
		var opts github.PackageListOptions
		opts.PerPage = 100

		for {
			candidates, rsp, err := m(ctx, this.build.owner.String(), "container", this.build.repo.name.String(), &opts)
			if err != nil {
				yield(nil, fmt.Errorf("cannot retrieve package versions information for %v (page: %d): %w", this.build, opts.Page, err))
				return
			}
			for _, v := range candidates {
				if !yield(&packageVersion{v, this}, nil) {
					return
				}
			}
			if rsp.NextPage == 0 {
				return
			}
			opts.Page = rsp.NextPage
		}
	}
}

func (this *packages) versionsWithAtLeastOneTag(ctx context.Context, tags []string) iter.Seq2[*packageVersion, error] {
	return func(yield func(*packageVersion, error) bool) {
		for candidate, err := range this.versions(ctx) {
			if err != nil {
				yield(nil, err)
				return
			}
			if candidate.Metadata != nil && candidate.Metadata.Container != nil {
				if slices.ContainsFunc(candidate.Metadata.Container.Tags, func(s string) bool {
					return slices.Contains(tags, s)
				}) {
					if !yield(candidate, nil) {
						return
					}
				}
			}
		}
	}
}

type packageVersion struct {
	*github.PackageVersion
	parent *packages
}

func (this *packageVersion) delete(ctx context.Context) error {
	var m func(context.Context, string, string, string, int64) (*github.Response, error)
	if this.parent.build.ownerType == user {
		m = this.parent.build.client.Users.PackageDeleteVersion
	} else {
		m = this.parent.build.client.Organizations.PackageDeleteVersion
	}

	if _, err := m(ctx, this.parent.build.owner.String(), "container", this.parent.build.repo.name.String(), *this.ID); err != nil {
		return fmt.Errorf("cannot delete package version %v: %w", this, err)
	}

	return nil
}

func (this packageVersion) String() string {
	return fmt.Sprintf("%s(%d)@%v", *this.Name, *this.ID, this.parent.build)
}
