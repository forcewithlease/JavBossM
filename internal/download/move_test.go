package download

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"javboss/internal/models"
)

func TestLargestVideoFilePrefersBiggestVideo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeSize(t, filepath.Join(dir, "a.mp4"), 1024)
	writeSize(t, filepath.Join(dir, "b.mkv"), 4096)
	writeSize(t, filepath.Join(dir, "notes.txt"), 8192)
	files, _ := json.Marshal([]string{
		filepath.Join(dir, "a.mp4"),
		filepath.Join(dir, "b.mkv"),
		filepath.Join(dir, "notes.txt"),
	})
	job := &models.DownloadJob{LocalFilesJSON: string(files)}
	got, err := largestVideoFile(job)
	if err != nil {
		t.Fatalf("largestVideoFile: %v", err)
	}
	if got != filepath.Join(dir, "b.mkv") {
		t.Fatalf("unexpected file: %q", got)
	}
}

func TestLargestVideoFileFallsBackToFolder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	folder := filepath.Join(root, "ABC-001-abc12345")
	writeSize(t, filepath.Join(folder, "ABC-001.mp4"), 2048)
	writeSize(t, filepath.Join(folder, "sample.mp4"), 512)
	writeSize(t, filepath.Join(folder, "ABC-001.mp4.aria2"), 4096)
	job := &models.DownloadJob{
		DownloadDirectory: root,
		MagnetName:        "ABC-001",
		InfoHash:          "abc12345",
		LocalFilesJSON:    "[]",
	}
	got, err := largestVideoFile(job)
	if err != nil {
		t.Fatalf("largestVideoFile: %v", err)
	}
	if got != filepath.Join(folder, "ABC-001.mp4") {
		t.Fatalf("unexpected file: %q", got)
	}
}

func TestUniqueTargetPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeSize(t, filepath.Join(dir, "ABC-001.mp4"), 10)
	target, err := uniqueTargetPath(dir, "ABC-001.mp4")
	if err != nil {
		t.Fatalf("uniqueTargetPath: %v", err)
	}
	if filepath.Base(target) != "ABC-001 (1).mp4" {
		t.Fatalf("unexpected target: %q", target)
	}
}

func TestMoveFileSameFilesystem(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	source := filepath.Join(dir, "source.mp4")
	target := filepath.Join(dir, "target.mp4")
	writeSize(t, source, 123)
	if err := moveFile(source, target); err != nil {
		t.Fatalf("moveFile: %v", err)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("source still exists: %v", err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat target: %v", err)
	}
	if info.Size() != 123 {
		t.Fatalf("unexpected size: %d", info.Size())
	}
}

func writeSize(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}
