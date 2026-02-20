package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

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

func isConflict(err error) bool {
	var ghErr *github.ErrorResponse
	return errors.As(err, &ghErr) && ghErr.Response != nil && ghErr.Response.StatusCode == http.StatusConflict
}

// FetchFromRepo recursively fetches all .md files from the repo and returns them.
// Used to seed the store on startup.
func (c *Client) FetchFromRepo(ctx context.Context, token, owner, repo string) ([]*sync.File, error) {
	if token == "" {
		return nil, nil
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

	var out []*sync.File
	var walk func(path string) error
	walk = func(path string) error {
		opts := &github.RepositoryContentGetOptions{Ref: branch}
		_, dirContents, _, err := client.Repositories.GetContents(ctx, owner, repo, path, opts)
		if err != nil {
			return err
		}
		if dirContents == nil {
			return nil
		}
		for _, e := range dirContents {
			if e.Type == nil {
				continue
			}
			p := path
			if p != "" {
				p += "/"
			}
			p += *e.Name
			switch *e.Type {
			case "dir":
				if err := walk(p); err != nil {
					return err
				}
			case "file":
				if !strings.HasSuffix(strings.ToLower(p), ".md") {
					continue
				}
				file, _, _, err := client.Repositories.GetContents(ctx, owner, repo, p, opts)
				if err != nil {
					return err
				}
				if file == nil || file.Content == nil {
					continue
				}
				content, err := file.GetContent()
				if err != nil {
					return err
				}
				out = append(out, &sync.File{Path: p, Content: content, Hash: sync.ContentHash(content)})
			}
		}
		return nil
	}
	if err := walk(""); err != nil {
		return nil, err
	}
	return out, nil
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
		existing, _, _, err := client.Repositories.GetContents(ctx, owner, repo, f.Path, &github.RepositoryContentGetOptions{Ref: branch})
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
			if isConflict(err) {
				// Race: file changed since we fetched. Re-fetch SHA and retry once.
				existing, _, _, err2 := client.Repositories.GetContents(ctx, owner, repo, f.Path, &github.RepositoryContentGetOptions{Ref: branch})
				if err2 != nil {
					return err
				}
				opts.SHA = existing.SHA
				_, _, err = client.Repositories.UpdateFile(ctx, owner, repo, f.Path, opts)
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}
