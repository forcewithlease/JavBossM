package download

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"javboss/internal/db"
	"javboss/internal/models"
)

// CleanupResult reports what a download-folder cleanup removed. The moved
// video and its subtitle sidecars are never deleted.
type CleanupResult struct {
	RemovedFiles int      `json:"removed_files"`
	RemovedBytes int64    `json:"removed_bytes"`
	Directories  []string `json:"directories"`
	Kept         []string `json:"kept"`
}

// CleanDownloadDirectory removes the job's download folder after its video was
// moved into the media library. For downloads that landed directly in the
// media library (Xunlei), only the task's own subfolder is cleaned and the
// moved video plus subtitle sidecars are kept, so advertising clips and other
// junk from the torrent disappear without touching the rest of the library.
func CleanDownloadDirectory(ctx context.Context, jobID int64) (*CleanupResult, error) {
	job, err := db.Default().GetDownloadJob(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, errors.New("download job not found")
	}
	if job.Status != models.DownloadCompleted {
		return nil, errors.New("only completed downloads can be cleaned")
	}
	moved := strings.TrimSpace(job.MovedPath)
	if moved == "" {
		return nil, errors.New("move the video to the media library first")
	}
	moved = filepath.Clean(moved)
	result := &CleanupResult{Directories: []string{}, Kept: []string{moved}}
	if job.CleanupStatus == "cleaned" {
		return result, nil
	}

	mediaRoots, mediaErr := downloadMediaRoots(ctx)
	if mediaErr != nil {
		return result, mediaErr
	}
	downloadRoot := filepath.Clean(strings.TrimSpace(job.DownloadDirectory))
	candidates := []string{
		filepath.Join(downloadRoot, downloadJobFolderName(job.MagnetName, job.InfoHash)),
		filepath.Dir(moved),
	}
	seen := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		candidate = filepath.Clean(candidate)
		if candidate == "" || candidate == "." || candidate == string(filepath.Separator) {
			continue
		}
		if seen[candidate] {
			continue
		}
		seen[candidate] = true
		if !safeCleanupDirectory(candidate, downloadRoot, mediaRoots) {
			continue
		}
		info, statErr := os.Stat(candidate)
		if errors.Is(statErr, os.ErrNotExist) {
			continue
		}
		if statErr != nil {
			return result, statErr
		}
		if !info.IsDir() {
			continue
		}
		if pathWithin(candidate, moved) {
			// The video lives in this folder (Xunlei downloads directly into the
			// media tree). Keep it and its subtitles, remove everything else.
			if cleanErr := cleanKeepingVideo(candidate, moved, result); cleanErr != nil {
				return result, cleanErr
			}
		} else if removeErr := removeCounted(candidate, result); removeErr != nil {
			return result, removeErr
		}
		result.Directories = append(result.Directories, candidate)
	}

	if err := db.Default().UpdateDownloadJob(ctx, jobID, map[string]any{
		"cleanup_status": "cleaned",
	}); err != nil {
		return result, err
	}
	return result, nil
}

func downloadMediaRoots(ctx context.Context) ([]string, error) {
	dirs, err := db.Default().ListDirectories(ctx)
	if err != nil {
		return nil, err
	}
	roots := make([]string, 0, len(dirs))
	for index := range dirs {
		path := strings.TrimSpace(dirs[index].Path)
		if path == "" {
			continue
		}
		roots = append(roots, filepath.Clean(path))
	}
	return roots, nil
}

// safeCleanupDirectory only allows dedicated job folders: a strict subfolder
// of the download directory or of a media directory. Roots themselves are
// never valid cleanup targets.
func safeCleanupDirectory(candidate, downloadRoot string, mediaRoots []string) bool {
	if downloadRoot != "" && downloadRoot != "." && candidate != downloadRoot && pathWithin(downloadRoot, candidate) {
		return true
	}
	for _, root := range mediaRoots {
		if candidate != root && pathWithin(root, candidate) {
			return true
		}
	}
	return false
}

func cleanKeepingVideo(dir, moved string, result *CleanupResult) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if samePath(path, moved) {
			continue
		}
		if entry.IsDir() {
			if pathWithin(path, moved) {
				if err := cleanKeepingVideo(path, moved, result); err != nil {
					return err
				}
				continue
			}
			if err := removeCounted(path, result); err != nil {
				return err
			}
			continue
		}
		if filepath.Dir(path) == filepath.Dir(moved) && isSubtitleSidecar(path, moved) {
			result.Kept = appendUnique(result.Kept, path)
			continue
		}
		if err := removeCounted(path, result); err != nil {
			return err
		}
	}
	return nil
}

func isSubtitleSidecar(path, moved string) bool {
	base := strings.TrimSuffix(filepath.Base(moved), filepath.Ext(moved))
	name := filepath.Base(path)
	if !strings.HasPrefix(name, base+".") {
		return false
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".srt", ".ass", ".ssa", ".vtt", ".sub", ".idx", ".sup", ".smi", ".nfo":
		return true
	default:
		return false
	}
}

func removeCounted(path string, result *CleanupResult) error {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		if err := os.Remove(path); err != nil {
			return err
		}
		result.RemovedFiles++
		result.RemovedBytes += info.Size()
		return nil
	}
	var files int
	var bytes int64
	if err := filepath.WalkDir(path, func(_ string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return infoErr
		}
		files++
		bytes += info.Size()
		return nil
	}); err != nil {
		return err
	}
	if err := os.RemoveAll(path); err != nil {
		return err
	}
	result.RemovedFiles += files
	result.RemovedBytes += bytes
	return nil
}

func pathWithin(parent, child string) bool {
	parent = filepath.Clean(parent)
	child = filepath.Clean(child)
	if parent == child {
		return true
	}
	relative, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
