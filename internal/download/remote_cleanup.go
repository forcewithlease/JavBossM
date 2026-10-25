package download

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"javboss/internal/db"
	"javboss/internal/download/downloader"
	"javboss/internal/models"
)

// CleanRemoteTask removes advertising clips and other junk from a completed
// downloader task that was created outside JavBoss (for example in the Xunlei
// dashboard). It keeps the largest video and subtitle sidecars and only works
// inside a task-specific subfolder of a configured media directory.
func CleanRemoteTask(ctx context.Context, taskID string) (*CleanupResult, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, errors.New("xunlei task id is empty")
	}
	settings, err := db.Default().GetDownloaderSettings(ctx)
	if err != nil {
		return nil, err
	}
	if settings.ActiveProvider != models.DownloaderProviderXunlei {
		return nil, errors.New("the active downloader does not support remote cleanup")
	}
	providerSettings, err := db.Default().GetDownloaderProviderSettings(ctx, settings.ActiveProvider)
	if err != nil {
		return nil, err
	}
	client, err := openLocalClient(providerSettings)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	lister, ok := client.(downloader.TaskLister)
	if !ok {
		return nil, errors.New("the active downloader does not expose a task list")
	}
	tasks, err := lister.ListTasks(ctx)
	if err != nil {
		return nil, err
	}
	var task *downloader.RemoteTask
	for index := range tasks {
		if tasks[index].ID == taskID {
			task = &tasks[index]
			break
		}
	}
	if task == nil {
		return nil, errors.New("xunlei task not found")
	}
	if task.State != "complete" {
		return nil, errors.New("only completed xunlei tasks can be cleaned")
	}
	taskPath := strings.TrimSpace(task.Path)
	if taskPath == "" {
		return nil, errors.New("xunlei task path is empty")
	}
	remoteRoot := filepath.Clean(strings.TrimSpace(providerSettings.RemoteFolder))
	if remoteRoot == "" || remoteRoot == "." {
		return nil, errors.New("set the xunlei download root first")
	}
	if !strings.HasPrefix(taskPath, remoteRoot) {
		return nil, errors.New("xunlei task path is outside the configured download root")
	}
	relative := strings.TrimLeft(strings.TrimPrefix(taskPath, remoteRoot), string(os.PathSeparator))
	mediaRoots, err := downloadMediaRoots(ctx)
	if err != nil {
		return nil, err
	}
	var localPath string
	for _, root := range mediaRoots {
		candidate := filepath.Join(root, relative)
		info, statErr := os.Stat(candidate)
		if statErr != nil {
			continue
		}
		if !info.IsDir() {
			// Single-file tasks have no task folder to clean.
			return &CleanupResult{Kept: []string{candidate}}, nil
		}
		localPath = candidate
		break
	}
	if localPath == "" {
		return nil, errors.New("xunlei task folder was not found in the media library")
	}
	safe := false
	for _, root := range mediaRoots {
		if localPath != root && pathWithin(root, localPath) {
			safe = true
			break
		}
	}
	if !safe {
		return nil, errors.New("refusing to clean a media directory root")
	}
	largest, err := largestVideoIn(localPath)
	if err != nil {
		return nil, err
	}
	result := &CleanupResult{Directories: []string{localPath}, Kept: []string{largest}}
	if err := cleanKeepingVideo(localPath, largest, result); err != nil {
		return result, err
	}
	return result, nil
}

func largestVideoIn(dir string) (string, error) {
	var largest string
	var largestSize int64
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || isPartialDownloadName(entry.Name()) {
			return nil
		}
		if !downloadVideoExtensions[strings.ToLower(filepath.Ext(entry.Name()))] {
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return infoErr
		}
		if info.Size() > largestSize {
			largest, largestSize = path, info.Size()
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if largest == "" {
		return "", errors.New("no completed video found in the xunlei task folder")
	}
	return largest, nil
}

func isPartialDownloadName(name string) bool {
	name = strings.ToLower(name)
	for _, suffix := range []string{".xltd", ".aria2", ".part", ".!qb", ".crdownload", ".download"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}
