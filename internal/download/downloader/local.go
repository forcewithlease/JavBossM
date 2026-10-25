package downloader

import "context"

// LocalStatus describes a task submitted to a downloader that writes files
// directly into JavBoss's download directory.
type LocalStatus struct {
	State      OfflineState
	DoneBytes  int64
	TotalBytes int64
	Message    string
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
