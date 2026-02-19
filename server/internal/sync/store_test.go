package sync

import (
	"testing"
)

func TestStore_UpsertGetDelete(t *testing.T) {
	s := NewStore()
	user := "alice"

	s.UpsertFile(user, "a.md", "content a", "hash-a")
	s.UpsertFile(user, "b.md", "content b", "hash-b")
	files, deleted := s.GetFiles(user)
	if len(files) != 2 || len(deleted) != 0 {
		t.Fatalf("got %d files, %d deleted", len(files), len(deleted))
	}

	s.DeleteFile(user, "a.md")
	files, deleted = s.GetFiles(user)
	if len(files) != 1 || len(deleted) != 1 {
		t.Fatalf("after delete: %d files, %d deleted", len(files), len(deleted))
	}
	if files[0].Path != "b.md" || files[0].Content != "content b" {
		t.Fatalf("wrong file: %+v", files[0])
	}
}

func TestStore_UserMeta(t *testing.T) {
	s := NewStore()
	meta := &UserMeta{GitHubToken: "tk", RepoOwner: "o", RepoName: "r"}
	s.SetUserMeta("u1", meta)
	got := s.GetUserMeta("u1")
	if got == nil || got.RepoOwner != "o" || got.RepoName != "r" {
		t.Fatalf("GetUserMeta: got %+v", got)
	}
	if s.GetUserMeta("u2") != nil {
		t.Fatal("expected nil for unknown user")
	}
}
