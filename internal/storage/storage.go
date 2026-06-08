package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Store abstracts file storage operations
type Store interface {
	// Put stores data at the given key
	Put(key string, data io.Reader) error
	// PutWithSize stores data with known size (enables streaming for S3)
	PutWithSize(key string, data io.Reader, size int64) error
	// Get returns a reader for the given key
	Get(key string) (io.ReadCloser, error)
	// Delete removes the object at the given key
	Delete(key string) error
}

// LocalStore implements Store using the local filesystem
type LocalStore struct {
	BasePath string
}

func NewLocalStore(basePath string) (*LocalStore, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("create storage dir: %w", err)
	}
	return &LocalStore{BasePath: basePath}, nil
}

func (s *LocalStore) Put(key string, data io.Reader) error {
	return s.PutWithSize(key, data, 0)
}

func (s *LocalStore) PutWithSize(key string, data io.Reader, _ int64) error {
	path := filepath.Join(s.BasePath, key)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, data); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

func (s *LocalStore) Get(key string) (io.ReadCloser, error) {
	path := filepath.Join(s.BasePath, key)
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	return f, nil
}

func (s *LocalStore) Delete(key string) error {
	path := filepath.Join(s.BasePath, key)
	return os.Remove(path)
}
