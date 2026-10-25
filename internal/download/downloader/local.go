package downloader

import "context"

// LocalStatus describes a task submitted to a downloader that writes files
// directly into JavBoss's download directory.
type LocalStatus struct {
	State      OfflineState
	DoneBytes  int64
	TotalBytes int64
	SpeedBytes int64
	// Path is the provider-side folder that received the files (when known).
	Path    string
	Message string
}

// LocalClient is implemented by download daemons running next to JavBoss
// (aria2, qBittorrent). They own the transfer and write into a directory that
// JavBoss can read, so no remote file copy step is required.
type LocalClient interface {
	Test(ctx context.Context) (*TestResult, error)
	Add(ctx context.Context, magnet, directory string) (string, error)
	Status(ctx context.Context, taskID string) (*LocalStatus, error)
	Remove(ctx context.Context, taskID string) error
	Close() error
}

// RemoteTask is a task that already exists inside a downloader. It is used to
// surface tasks created outside JavBoss (for example in the Xunlei dashboard).
type RemoteTask struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	State      string `json:"state"`
	Phase      string `json:"phase"`
	TotalBytes int64  `json:"total_bytes"`
	DoneBytes  int64  `json:"done_bytes"`
	SpeedBytes int64  `json:"speed_bytes"`
	Path       string `json:"path"`
	Message    string `json:"message"`
	CreatedAt  string `json:"created_at"`
}

// TaskLister is implemented by downloaders that can enumerate their remote
// task list.
type TaskLister interface {
	ListTasks(ctx context.Context) ([]RemoteTask, error)
}
