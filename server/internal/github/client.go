package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/go-github/v66/github"
	"github.com/shaun/flux/server/internal/sync"
	"golang.org/x/oauth2"
)

type Client struct {
	hc *http.Client // optional; for tests
}

func NewClient() *Client {
	return &Client{}
}

// NewClientWithHTTPClient returns a client that uses the given http.Client for API calls (e.g. in tests).
func NewClientWithHTTPClient(hc *http.Client) *Client {
	return &Client{hc: hc}
}

func (c *Client) Sync(ctx context.Context, token, owner, repo string, files []*sync.File, deleted []string) error {
	if token == "" {
		return nil
	}
	var httpClient *http.Client
	if c.hc != nil {
		httpClient = c.hc
	} else {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
		httpClient = oauth2.NewClient(ctx, ts)
	}
	client := github.NewClient(httpClient)
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
