package download

import (
	"context"

	"javboss/internal/db"
	"javboss/internal/download/downloader"
	"javboss/internal/models"
)

// DownloaderTaskList is the remote task list exposed by the active local
// downloader. Tasks created outside JavBoss (for example in the Xunlei
// dashboard) appear here as well.
type DownloaderTaskList struct {
	Provider  string                  `json:"provider"`
	Supported bool                    `json:"supported"`
	Tasks     []downloader.RemoteTask `json:"tasks"`
	Error     string                  `json:"error,omitempty"`
}

// ListDownloaderTasks reads the active downloader's own task list. It never
// mutates anything and is safe to poll from the downloads page.
func ListDownloaderTasks(ctx context.Context) *DownloaderTaskList {
	result := &DownloaderTaskList{Tasks: []downloader.RemoteTask{}}
	settings, err := db.Default().GetDownloaderSettings(ctx)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.Provider = settings.ActiveProvider
	if !models.IsLocalDownloader(result.Provider) {
		return result
	}
	providerSettings, err := db.Default().GetDownloaderProviderSettings(ctx, result.Provider)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	client, err := openLocalClient(providerSettings)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer client.Close()
	lister, ok := client.(downloader.TaskLister)
	if !ok {
		return result
	}
	result.Supported = true
	tasks, err := lister.ListTasks(ctx)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	if tasks != nil {
		// Tasks that JavBoss created are already shown in the download queue.
		// Keep the remote list for tasks added directly in the downloader UI.
		if tracked, trackedErr := db.Default().DownloadJobRemoteTaskIDs(ctx); trackedErr == nil {
			filtered := make([]downloader.RemoteTask, 0, len(tasks))
			for _, task := range tasks {
				if _, exists := tracked[task.ID]; exists {
					continue
				}
				filtered = append(filtered, task)
			}
			tasks = filtered
		}
		result.Tasks = tasks
	}
	return result
}
