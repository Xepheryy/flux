package api

type PushRequest struct {
	Files   []PushFile `json:"files"`
	Deleted []string   `json:"deleted"`
}

type PushFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Hash    string `json:"hash"`
}

type SetupRequest struct {
	GitHubToken string `json:"githubToken"`
	RepoOwner   string `json:"repoOwner"`
	RepoName    string `json:"repoName"`
}

type PullResponse struct {
	Files   []PullFile `json:"files"`
	Deleted []string   `json:"deleted"`
}

type PullFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Hash    string `json:"hash"`
}
