package download

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"javboss/internal/db"
	"javboss/internal/models"
)

var downloadVideoExtensions = map[string]bool{
	".mp4": true, ".mkv": true, ".avi": true, ".wmv": true, ".mov": true,
	".m4v": true, ".ts": true, ".m2ts": true, ".flv": true, ".webm": true,
	".rmvb": true, ".rm": true, ".iso": true, ".mpg": true, ".mpeg": true,
}

// MoveLargestVideo moves the biggest video file of a finished job into the
// target media directory. The download folder itself is left untouched.
func MoveLargestVideo(ctx context.Context, jobID int64, targetDir string) (string, error) {
	job, err := db.Default().GetDownloadJob(ctx, jobID)
	if err != nil {
		return "", err
	}
	if job == nil {
		return "", errors.New("download job not found")
	}
	targetDir = strings.TrimSpace(targetDir)
	if targetDir == "" {
		return "", errors.New("target media directory is empty")
	}

	source, err := largestVideoFile(job)
	if err != nil {
		markMoveFailed(ctx, jobID, err)
		return "", err
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		markMoveFailed(ctx, jobID, err)
		return "", fmt.Errorf("create media directory: %w", err)
	}
	target, err := uniqueTargetPath(targetDir, filepath.Base(source))
	if err != nil {
		markMoveFailed(ctx, jobID, err)
		return "", err
	}
	if samePath(source, target) {
		markMoved(ctx, jobID, target)
		return target, nil
	}
	if err := moveFile(source, target); err != nil {
		markMoveFailed(ctx, jobID, err)
		return "", err
	}
	files, _ := json.Marshal([]string{target})
	if err := db.Default().UpdateDownloadJob(ctx, jobID, map[string]any{
		"moved_path":       target,
		"move_status":      "moved",
		"move_error":       "",
		"local_files_json": string(files),
	}); err != nil {
		return target, err
	}
	return target, nil
}

func largestVideoFile(job *models.DownloadJob) (string, error) {
	candidates := make([]string, 0, 4)
	var stored []string
	if json.Unmarshal([]byte(job.LocalFilesJSON), &stored) == nil {
		for _, path := range stored {
			if strings.TrimSpace(path) != "" {
				candidates = append(candidates, path)
			}
		}
	}
	if len(candidates) == 0 {
		folder := filepath.Join(job.DownloadDirectory, downloadJobFolderName(job.MagnetName, job.InfoHash))
		_ = filepath.WalkDir(folder, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if entry.IsDir() {
				return nil
			}
			name := strings.ToLower(entry.Name())
			if !strings.HasSuffix(name, ".aria2") && !strings.HasSuffix(name, ".part") && !strings.HasSuffix(name, ".!qb") {
				candidates = append(candidates, path)
			}
			return nil
		})
	}

	var largest string
	var largestSize int64
	for _, path := range candidates {
		ext := strings.ToLower(filepath.Ext(path))
		if !downloadVideoExtensions[ext] {
			continue
		}
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}
		if info.Size() > largestSize {
			largest, largestSize = path, info.Size()
		}
	}
	if largest == "" {
		return "", errors.New("the download contains no video file")
	}
	return largest, nil
}

func uniqueTargetPath(dir, name string) (string, error) {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "", errors.New("invalid video file name")
	}
	target := filepath.Join(dir, name)
	if _, err := os.Stat(target); errors.Is(err, os.ErrNotExist) {
		return target, nil
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for index := 1; index < 1000; index++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, index, ext))
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		}
	}
	return "", errors.New("too many files with the same name")
}

func moveFile(source, target string) error {
	if err := os.Rename(source, target); err == nil {
		return nil
	}
	// Cross-device moves (for example ext4 downloads -> NTFS media) need a copy.
	if err := copyFile(source, target); err != nil {
		_ = os.Remove(target)
		return err
	}
	if err := os.Remove(source); err != nil {
		return fmt.Errorf("remove source after copy: %w", err)
	}
	return nil
}

func copyFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	temp := target + ".part"
	out, err := os.OpenFile(temp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(temp)
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		_ = os.Remove(temp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(temp)
		return err
	}
	if info, err := os.Stat(source); err == nil {
		if copied, statErr := os.Stat(temp); statErr != nil || copied.Size() != info.Size() {
			_ = os.Remove(temp)
			return errors.New("copied file size does not match")
		}
	}
	if err := os.Rename(temp, target); err != nil {
		_ = os.Remove(temp)
		return err
	}
	return nil
}

func samePath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	return leftErr == nil && rightErr == nil && leftAbs == rightAbs
}

func markMoved(ctx context.Context, jobID int64, target string) {
	files, _ := json.Marshal([]string{target})
	_ = db.Default().UpdateDownloadJob(ctx, jobID, map[string]any{
		"moved_path": target, "move_status": "moved", "move_error": "",
		"local_files_json": string(files),
	})
}

func markMoveFailed(ctx context.Context, jobID int64, moveErr error) {
	message := strings.TrimSpace(moveErr.Error())
	if len([]rune(message)) > 500 {
		message = string([]rune(message)[:500])
	}
	_ = db.Default().UpdateDownloadJob(ctx, jobID, map[string]any{
		"move_status": "failed", "move_error": message,
	})
}
