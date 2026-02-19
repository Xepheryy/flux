package sync

import (
	"testing"
)

func TestStore_UpsertGetDelete(t *testing.T) {
	s := NewStore()
	s.UpsertFile("a.md", "content a", "hash-a")
	s.UpsertFile("b.md", "content b", "hash-b")
	files, deleted := s.GetFiles()
	if len(files) != 2 || len(deleted) != 0 {
		t.Fatalf("got %d files, %d deleted", len(files), len(deleted))
	}
	s.DeleteFile("a.md")
	files, deleted = s.GetFiles()
	if len(files) != 1 || len(deleted) != 1 {
		t.Fatalf("after delete: %d files, %d deleted", len(files), len(deleted))
	}
	if files[0].Path != "b.md" || files[0].Content != "content b" {
		t.Fatalf("wrong file: %+v", files[0])
	}
}

