package fs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileExists_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if !FileExists(path) {
		t.Error("expected true for existing file")
	}
}

func TestFileExists_MissingFile(t *testing.T) {
	if FileExists("/nonexistent/path/file.txt") {
		t.Error("expected false for missing file")
	}
}

func TestFileExists_Directory(t *testing.T) {
	dir := t.TempDir()
	if FileExists(dir) {
		t.Error("expected false for directory")
	}
}

func TestFileExists_EmptyPath(t *testing.T) {
	if FileExists("") {
		t.Error("expected false for empty path")
	}
}
