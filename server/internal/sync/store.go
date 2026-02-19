package sync

import (
	"sync"
	"time"
)

type File struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Hash      string `json:"hash"`
	UpdatedAt int64  `json:"updatedAt"`
}

type Store struct {
	mu      sync.RWMutex
	files   map[string]*File
	deleted map[string]int64
}

func NewStore() *Store {
	return &Store{
		files:   make(map[string]*File),
		deleted: make(map[string]int64),
	}
}

func (s *Store) UpsertFile(path, content, hash string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UnixMilli()
	s.files[path] = &File{Path: path, Content: content, Hash: hash, UpdatedAt: now}
	delete(s.deleted, path)
}

func (s *Store) DeleteFile(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UnixMilli()
	delete(s.files, path)
	s.deleted[path] = now
}

func (s *Store) GetFiles() ([]*File, []string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var files []*File
	for _, f := range s.files {
		files = append(files, f)
	}
	var deleted []string
	for p := range s.deleted {
		deleted = append(deleted, p)
	}
	return files, deleted
}
