package download

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"javboss/internal/db"
	"javboss/internal/models"
)

func setupCleanupTest(t *testing.T) (mediaRoot, downloadRoot string) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "download-cleanup.db"))
	if err != nil {
		t.Fatal(err)
	}
	store := db.NewStore(database)
	previous := db.SetDefault(store)
	t.Cleanup(func() { db.SetDefault(previous) })
	t.Cleanup(func() {
		if sqlDB, err := database.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	mediaRoot = t.TempDir()
	downloadRoot = t.TempDir()
	if _, err := store.CreateDirectory(context.Background(), mediaRoot); err != nil {
		t.Fatal(err)
	}
	return mediaRoot, downloadRoot
}

func createCompletedJob(t *testing.T, job *models.DownloadJob) {
	t.Helper()
	job.MagnetURL = "magnet:?xt=urn:btih:" + job.InfoHash
	if err := db.Default().CreateDownloadJob(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	if err := db.Default().UpdateDownloadJob(context.Background(), job.ID, map[string]any{
		"status": models.DownloadCompleted, "moved_path": job.MovedPath,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestCleanDownloadDirectoryRemovesSourceFolder(t *testing.T) {
	mediaRoot, downloadRoot := setupCleanupTest(t)
	job := &models.DownloadJob{
		DownloadDirectory: downloadRoot,
		Provider:          models.DownloaderProviderAria2,
		InfoHash:          "abc12345",
		MagnetName:        "ABC-001",
		Status:            models.DownloadCompleted,
		MovedPath:         filepath.Join(mediaRoot, "ABC-001.mp4"),
	}
	sourceFolder := filepath.Join(downloadRoot, downloadJobFolderName(job.MagnetName, job.InfoHash))
	writeSize(t, filepath.Join(sourceFolder, "ABC-001.mp4"), 4096)
	writeSize(t, filepath.Join(sourceFolder, "ad.mp4"), 2048)
	writeSize(t, filepath.Join(sourceFolder, "sub", "extra.mp4"), 1024)
	writeSize(t, job.MovedPath, 4096)
	otherVideo := filepath.Join(mediaRoot, "OTHER-001.mp4")
	writeSize(t, otherVideo, 1234)
	createCompletedJob(t, job)

	result, err := CleanDownloadDirectory(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("CleanDownloadDirectory: %v", err)
	}
	if result.RemovedFiles != 3 {
		t.Fatalf("removed files = %d, want 3 (%+v)", result.RemovedFiles, result)
	}
	if _, err := os.Stat(sourceFolder); !os.IsNotExist(err) {
		t.Fatalf("source folder still exists: %v", err)
	}
	if _, err := os.Stat(job.MovedPath); err != nil {
		t.Fatalf("moved video was deleted: %v", err)
	}
	if _, err := os.Stat(otherVideo); err != nil {
		t.Fatalf("unrelated media file was deleted: %v", err)
	}
	stored, err := db.Default().GetDownloadJob(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.CleanupStatus != "cleaned" {
		t.Fatalf("cleanup status = %q", stored.CleanupStatus)
	}
}

func TestCleanDownloadDirectoryKeepsMovedVideoAndSubtitles(t *testing.T) {
	mediaRoot, downloadRoot := setupCleanupTest(t)
	taskFolder := filepath.Join(mediaRoot, "TANF-045")
	moved := filepath.Join(taskFolder, "TANF-045.mp4")
	job := &models.DownloadJob{
		DownloadDirectory: downloadRoot,
		Provider:          models.DownloaderProviderXunlei,
		InfoHash:          "abc12345",
		MagnetName:        "TANF-045",
		Status:            models.DownloadCompleted,
		MovedPath:         moved,
	}
	writeSize(t, moved, 4096)
	writeSize(t, filepath.Join(taskFolder, "TANF-045.zh.srt"), 100)
	writeSize(t, filepath.Join(taskFolder, "TANF-045.chs.ass"), 100)
	writeSize(t, filepath.Join(taskFolder, "ad.mp4"), 2048)
	writeSize(t, filepath.Join(taskFolder, "poster.jpg"), 128)
	writeSize(t, filepath.Join(taskFolder, "extras", "clip.mp4"), 512)
	createCompletedJob(t, job)

	result, err := CleanDownloadDirectory(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("CleanDownloadDirectory: %v", err)
	}
	if result.RemovedFiles != 3 {
		t.Fatalf("removed files = %d, want 3 (%+v)", result.RemovedFiles, result)
	}
	for _, kept := range []string{
		moved,
		filepath.Join(taskFolder, "TANF-045.zh.srt"),
		filepath.Join(taskFolder, "TANF-045.chs.ass"),
	} {
		if _, err := os.Stat(kept); err != nil {
			t.Fatalf("expected %s to be kept: %v", kept, err)
		}
	}
	for _, removed := range []string{
		filepath.Join(taskFolder, "ad.mp4"),
		filepath.Join(taskFolder, "poster.jpg"),
		filepath.Join(taskFolder, "extras"),
	} {
		if _, err := os.Stat(removed); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed: %v", removed, err)
		}
	}
	if _, err := os.Stat(taskFolder); err != nil {
		t.Fatalf("task folder should remain: %v", err)
	}
}

func TestLargestVideoInIgnoresPartialsAndAds(t *testing.T) {
	dir := t.TempDir()
	writeSize(t, filepath.Join(dir, "TANF-045.mp4"), 4096)
	writeSize(t, filepath.Join(dir, "ad.mp4"), 2048)
	writeSize(t, filepath.Join(dir, "TANF-045.mp4.xltd"), 8192)
	writeSize(t, filepath.Join(dir, "poster.jpg"), 128)
	got, err := largestVideoIn(dir)
	if err != nil {
		t.Fatalf("largestVideoIn: %v", err)
	}
	if got != filepath.Join(dir, "TANF-045.mp4") {
		t.Fatalf("largestVideoIn = %q", got)
	}
}

func TestCleanDownloadDirectoryRefusesMediaRoot(t *testing.T) {
	mediaRoot, downloadRoot := setupCleanupTest(t)
	moved := filepath.Join(mediaRoot, "ABC-001.mp4")
	writeSize(t, moved, 4096)
	otherVideo := filepath.Join(mediaRoot, "OTHER-001.mp4")
	writeSize(t, otherVideo, 1234)
	job := &models.DownloadJob{
		DownloadDirectory: downloadRoot,
		Provider:          models.DownloaderProviderAria2,
		InfoHash:          "abc12345",
		MagnetName:        "ABC-001",
		Status:            models.DownloadCompleted,
		MovedPath:         moved,
	}
	createCompletedJob(t, job)
	if _, err := CleanDownloadDirectory(context.Background(), job.ID); err != nil {
		t.Fatalf("CleanDownloadDirectory: %v", err)
	}
	if _, err := os.Stat(otherVideo); err != nil {
		t.Fatalf("unrelated media file was deleted: %v", err)
	}
	if _, err := os.Stat(moved); err != nil {
		t.Fatalf("moved video was deleted: %v", err)
	}
}
