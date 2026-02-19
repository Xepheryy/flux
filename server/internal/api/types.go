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

type PullResponse struct {
	Files   []PullFile `json:"files"`
	Deleted []string   `json:"deleted"`
}

type PullFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Hash    string `json:"hash"`
}
