package github

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/go-github/v66/github"
	"github.com/shaun/flux/server/internal/sync"
	"golang.org/x/oauth2"
)

type Client struct{}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) Sync(ctx context.Context, token, owner, repo string, files []*sync.File, deleted []string) error {
	if token == "" {
		return nil
	}
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	client := github.NewClient(oauth2.NewClient(ctx, ts))
	branch := "main"

	for _, path := range deleted {
		existing, _, _, err := client.Repositories.GetContents(ctx, owner, repo, path, nil)
		if err != nil {
			var ghErr *github.ErrorResponse
			if errors.As(err, &ghErr) && ghErr.Response != nil && ghErr.Response.StatusCode == 404 {
				continue
			}
			return err
		}
		_, _, err = client.Repositories.DeleteFile(ctx, owner, repo, path, &github.RepositoryContentFileOptions{
			Message: github.String(fmt.Sprintf("Flux: delete %s", path)),
			SHA:     existing.SHA,
			Branch:  &branch,
		})
		if err != nil {
			return err
		}
	}

	for _, f := range files {
		opts := &github.RepositoryContentFileOptions{
			Message: github.String(fmt.Sprintf("Flux: sync %s", f.Path)),
			Content: []byte(f.Content),
			Branch:  &branch,
		}
		existing, _, _, err := client.Repositories.GetContents(ctx, owner, repo, f.Path, nil)
		if err != nil {
			var ghErr *github.ErrorResponse
			if errors.As(err, &ghErr) && ghErr.Response != nil && ghErr.Response.StatusCode == 404 {
				_, _, err = client.Repositories.CreateFile(ctx, owner, repo, f.Path, opts)
				if err != nil {
					return err
				}
				continue
			}
			return err
		}
		opts.SHA = existing.SHA
		_, _, err = client.Repositories.UpdateFile(ctx, owner, repo, f.Path, opts)
		if err != nil {
			return err
		}
	}
	return nil
}
