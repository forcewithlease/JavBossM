package download

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanLocalDownloadFilesFiltersSmallAndControlFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write := func(name string, size int) string {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		return path
	}
	big := write("movie.mp4", 120)
	nested := write(filepath.Join("sub", "nested.mp4"), 80)
	write("small.mp4", 20)
	write("movie.mp4.aria2", 120)
	write("partial.mp4.!qB", 120)

	files, total, err := scanLocalDownloadFiles(root, 50)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d: %#v", len(files), files)
	}
	if total != 200 {
		t.Fatalf("expected 200 total bytes, got %d", total)
	}
	found := map[string]bool{}
	for _, file := range files {
		found[file] = true
	}
	if !found[big] || !found[nested] {
		t.Fatalf("missing expected files: %#v", files)
	}
}

func TestScanLocalDownloadFilesEmptyDirectory(t *testing.T) {
	t.Parallel()
	files, total, err := scanLocalDownloadFiles(t.TempDir(), 1)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(files) != 0 || total != 0 {
		t.Fatalf("expected no files, got %#v (%d bytes)", files, total)
	}
}
