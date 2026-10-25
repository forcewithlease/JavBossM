package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"javboss/internal/common"
	"javboss/internal/common/logging"
	dbpkg "javboss/internal/db"
	"javboss/internal/directory"
	"javboss/internal/download"
	"javboss/internal/download/clouddrive"
	"javboss/internal/download/downloader"
	"javboss/internal/models"
	"javboss/internal/util"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type downloaderSettingsResponse struct {
	ActiveProvider    string `json:"active_provider"`
	DownloadDirectory string `json:"download_directory"`
	LocalConcurrency  int    `json:"local_concurrency"`
	MinVideoSizeMB    int64  `json:"min_video_size_mb"`
	Address           string `json:"address"`
	RemoteFolder      string `json:"remote_folder"`
	TokenConfigured   bool   `json:"token_configured"`
}

func getDownloaderSettings(c *gin.Context) {
	payload, err := loadDownloaderSettingsPayload(c.Request.Context())
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "读取下载器配置失败", "Failed to load downloader settings")
		return
	}
	c.JSON(http.StatusOK, payload)
}

func updateDownloaderSettings(c *gin.Context) {
	var request struct {
		DownloadDirectory string `json:"download_directory"`
		LocalConcurrency  int    `json:"local_concurrency"`
		MinVideoSizeMB    int64  `json:"min_video_size_mb"`
		ActiveProvider    string `json:"active_provider"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "下载器配置格式不正确", "Invalid downloader settings")
		return
	}
	if request.LocalConcurrency < 1 || request.LocalConcurrency > models.MaxLocalDownloadConcurrency {
		respondLocalizedError(c, http.StatusBadRequest, "本地下载并发数必须在 1 到 3 之间", "Local download concurrency must be between 1 and 3")
		return
	}
	if request.MinVideoSizeMB < 1 || request.MinVideoSizeMB > 102400 {
		respondLocalizedError(c, http.StatusBadRequest, "视频最小下载体积必须在 1 MB 到 102400 MB 之间", "The minimum video download size must be between 1 MB and 102400 MB")
		return
	}
	downloadDirectory, err := normalizeDownloadDirectory(request.DownloadDirectory)
	if err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "下载目录必须是已存在的本地绝对路径", "The download directory must be an existing absolute local path")
		return
	}
	activeProvider := strings.TrimSpace(request.ActiveProvider)
	if activeProvider == "" {
		if current, loadErr := dbpkg.Default().GetDownloaderSettings(c.Request.Context()); loadErr == nil {
			activeProvider = current.ActiveProvider
		}
	}
	if activeProvider != "" && !models.IsSupportedDownloaderProvider(activeProvider) {
		respondLocalizedError(c, http.StatusBadRequest, "下载器类型不受支持", "Unsupported downloader provider")
		return
	}
	settings := models.DownloaderSettings{
		ID: 1, ActiveProvider: activeProvider,
		DownloadDirectory: downloadDirectory,
		LocalConcurrency:  request.LocalConcurrency,
		MinVideoSizeBytes: request.MinVideoSizeMB * 1024 * 1024,
	}
	if err := dbpkg.Default().SaveDownloaderSettings(c.Request.Context(), &settings); err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "保存下载器配置失败", "Failed to save downloader settings")
		return
	}
	download.Wake()
	payload, err := loadDownloaderSettingsPayload(c.Request.Context())
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "读取下载器配置失败", "Failed to load downloader settings")
		return
	}
	c.JSON(http.StatusOK, payload)
}

func updateCloudDrive2Settings(c *gin.Context) {
	var request struct {
		Address       string  `json:"address"`
		APIToken      *string `json:"api_token"`
		ClearAPIToken bool    `json:"clear_api_token"`
		RemoteFolder  string  `json:"remote_folder"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "下载器配置格式不正确", "Invalid provider settings")
		return
	}
	address := strings.TrimSpace(request.Address)
	remoteFolder := strings.TrimSpace(request.RemoteFolder)
	if len(address) > 500 || len(remoteFolder) > 2000 {
		respondLocalizedError(c, http.StatusBadRequest, "下载器地址或目录过长", "Downloader address or folder is too long")
		return
	}
	if request.APIToken != nil && len(*request.APIToken) > 16384 {
		respondLocalizedError(c, http.StatusBadRequest, "API Token 过长", "The API token is too long")
		return
	}
	current, err := dbpkg.Default().GetDownloaderProviderSettings(c.Request.Context(), models.DownloaderProviderCloudDrive2)
	if err == nil {
		err = dbpkg.Default().SaveDownloaderProviderSettings(c.Request.Context(), &models.DownloaderProviderSettings{
			Provider: models.DownloaderProviderCloudDrive2, Address: address,
			APIToken:     updatedProviderToken(current.APIToken, request.APIToken, request.ClearAPIToken),
			RemoteFolder: remoteFolder,
		})
	}
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "保存下载器配置失败", "Failed to save provider settings")
		return
	}
	payload, err := loadDownloaderSettingsPayload(c.Request.Context())
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "读取下载器配置失败", "Failed to load downloader settings")
		return
	}
	c.JSON(http.StatusOK, payload)
}

func getCloudDrive2Token(c *gin.Context) {
	settings, err := dbpkg.Default().GetDownloaderProviderSettings(c.Request.Context(), models.DownloaderProviderCloudDrive2)
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "读取 API Token 失败", "Failed to load the API token")
		return
	}
	c.JSON(http.StatusOK, gin.H{"api_token": settings.APIToken})
}

// testCloudDrive2 accepts optional address, api_token and remote_folder JSON fields.
// A request without a body tests the saved configuration; a body tests only its supplied values.
func testCloudDrive2(c *gin.Context) {
	var request struct {
		Address      *string `json:"address"`
		APIToken     *string `json:"api_token"`
		RemoteFolder *string `json:"remote_folder"`
	}
	bindErr := c.ShouldBindJSON(&request)
	if bindErr != nil && !errors.Is(bindErr, io.EOF) {
		respondLocalizedError(c, http.StatusBadRequest, "下载器配置格式不正确", "Invalid provider settings")
		return
	}
	var draft *models.DownloaderProviderSettings
	if bindErr == nil {
		if request.Address == nil || request.APIToken == nil || request.RemoteFolder == nil {
			respondLocalizedError(c, http.StatusBadRequest, "请提供地址、API Token 和云端离线目录", "Provide the address, API token, and remote offline folder")
			return
		}
		draft = &models.DownloaderProviderSettings{
			Provider: models.DownloaderProviderCloudDrive2,
			Address:  strings.TrimSpace(*request.Address), APIToken: strings.TrimSpace(*request.APIToken),
			RemoteFolder: strings.TrimSpace(*request.RemoteFolder),
		}
		if draft.Address == "" || draft.APIToken == "" || draft.RemoteFolder == "" {
			respondLocalizedError(c, http.StatusBadRequest, "请填写地址、API Token 和云端离线目录后再检测", "Enter the address, API token, and remote offline folder before testing")
			return
		}
		if len(draft.Address) > 500 || len(draft.RemoteFolder) > 2000 || len(*request.APIToken) > 16384 {
			respondLocalizedError(c, http.StatusBadRequest, "下载器地址、目录或 API Token 过长", "Downloader address, folder, or API token is too long")
			return
		}
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	var result *downloader.TestResult
	var err error
	if draft != nil {
		result, err = download.TestDownloaderWithSettings(ctx, draft)
	} else {
		result, err = download.TestDownloader(ctx, models.DownloaderProviderCloudDrive2)
	}
	if err != nil {
		messageZH, messageEN := cloudDrive2TestErrorMessages(err)
		respondLocalizedError(c, http.StatusBadGateway, messageZH, messageEN)
		return
	}
	c.JSON(http.StatusOK, result)
}

func cloudDrive2TestErrorMessages(err error) (string, string) {
	var missing *clouddrive.MissingPermissionsError
	if errors.As(err, &missing) {
		labels := map[string][2]string{
			"allow_list":                   {"列出文件", "list files"},
			"allow_create_folder":          {"创建目录", "create folders"},
			"allow_read":                   {"读取文件", "read files"},
			"allow_add_offline_download":   {"添加离线下载", "add offline downloads"},
			"allow_list_offline_downloads": {"查看离线下载", "list offline downloads"},
		}
		var zh, en []string
		for _, permission := range missing.Permissions {
			label, ok := labels[permission]
			if !ok {
				label = [2]string{permission, permission}
			}
			zh, en = append(zh, label[0]), append(en, label[1])
		}
		return "API 令牌权限不足，缺少：" + strings.Join(zh, "、") + "。请在 CloudDrive2 中编辑该令牌并开启相应权限。",
			"The API token is missing permissions: " + strings.Join(en, ", ") + ". Edit this token in CloudDrive2 and enable these permissions."
	}
	if status.Code(err) == codes.PermissionDenied {
		return "API 令牌权限不足或授权目录受限，请在 CloudDrive2 中检查令牌权限，并确认云端离线目录在授权范围内。",
			"The API token lacks permission or has a restricted folder scope. Check its permissions in CloudDrive2 and ensure the remote offline folder is within the authorized scope."
	}
	return "下载器连接测试失败：" + err.Error(), "Downloader connection test failed: " + err.Error()
}

func loadDownloaderSettingsPayload(ctx context.Context) (*downloaderSettingsResponse, error) {
	settings, err := dbpkg.Default().GetDownloaderSettings(ctx)
	if err != nil {
		return nil, err
	}
	cloudDrive2, err := dbpkg.Default().GetDownloaderProviderSettings(ctx, models.DownloaderProviderCloudDrive2)
	if err != nil {
		return nil, err
	}
	return &downloaderSettingsResponse{
		ActiveProvider:    settings.ActiveProvider,
		DownloadDirectory: settings.DownloadDirectory, LocalConcurrency: settings.LocalConcurrency,
		MinVideoSizeMB: settings.MinVideoSizeBytes / (1024 * 1024),
		Address:        cloudDrive2.Address, RemoteFolder: cloudDrive2.RemoteFolder,
		TokenConfigured: strings.TrimSpace(cloudDrive2.APIToken) != "",
	}, nil
}

func updatedProviderToken(current string, requested *string, clear bool) string {
	if clear {
		return ""
	}
	if requested != nil {
		return strings.TrimSpace(*requested)
	}
	return current
}

// listDownloadJobs supports limit (default 20, max 500) and offset (default 0).
// Counts describe all tasks, independent of the requested page.
func listDownloadJobs(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	jobs, err := dbpkg.Default().ListDownloadJobs(c.Request.Context(), queryInt(c, "limit", 20), queryInt(c, "offset", 0))
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "读取下载队列失败", "Failed to load the download queue")
		return
	}
	c.JSON(http.StatusOK, jobs)
}

func createDownloadJob(c *gin.Context) {
	var request struct {
		MagnetURL string `json:"magnet_url"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "下载请求格式不正确", "Invalid download request")
		return
	}
	enqueueDownloadJob(c, request.MagnetURL)
}

func enqueueDownloadJob(c *gin.Context, magnetURL string) {
	magnetURL = strings.TrimSpace(magnetURL)
	infoHash, err := download.ParseMagnetInfoHash(magnetURL)
	if err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "磁力链接格式不正确", "Invalid magnet link")
		return
	}
	settings, err := dbpkg.Default().GetDownloaderSettings(c.Request.Context())
	if err != nil {
		respondLocalizedError(c, http.StatusConflict, "尚未激活下载器", "No downloader is active")
		return
	}
	downloadDirectory, err := normalizeDownloadDirectory(settings.DownloadDirectory)
	if err != nil || downloadDirectory == "" {
		respondLocalizedError(c, http.StatusBadRequest, "请选择本地下载目录", "Select a local download directory")
		return
	}
	provider := settings.ActiveProvider
	if !models.IsSupportedDownloaderProvider(provider) {
		provider = models.DefaultDownloaderProvider
	}
	job := models.DownloadJob{
		DownloadDirectory: downloadDirectory, InfoHash: infoHash, MagnetURL: magnetURL,
		MagnetName: download.ParseMagnetName(magnetURL), Provider: provider,
	}
	if err := dbpkg.Default().CreateDownloadJob(c.Request.Context(), &job); err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "创建下载任务失败", "Failed to create the download job")
		return
	}
	download.Wake()
	c.JSON(http.StatusCreated, job)
}

func normalizeDownloadDirectory(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	value = filepath.Clean(value)
	if !filepath.IsAbs(value) {
		return "", errors.New("download directory is not absolute")
	}
	info, err := os.Stat(value)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("download directory is not a directory")
	}
	return value, nil
}

func retryDownloadJob(c *gin.Context) {
	id, ok := downloadJobID(c)
	if !ok {
		return
	}
	job, err := dbpkg.Default().GetDownloadJob(c.Request.Context(), id)
	if err != nil {
		respondLocalizedError(c, http.StatusNotFound, "下载任务不存在", "Download job not found")
		return
	}
	if !models.IsSupportedDownloaderProvider(job.Provider) {
		respondLocalizedError(c, http.StatusConflict, "该下载任务的下载器不受支持", "The download provider for this job is unsupported")
		return
	}
	if err := dbpkg.Default().RetryDownloadJob(c.Request.Context(), id); err != nil {
		respondLocalizedError(c, http.StatusConflict, "该任务当前不能重试", "The job cannot be retried in its current state")
		return
	}
	download.Wake()
	c.Status(http.StatusNoContent)
}

func cancelDownloadJob(c *gin.Context) {
	id, ok := downloadJobID(c)
	if !ok {
		return
	}
	if err := dbpkg.Default().CancelDownloadJob(c.Request.Context(), id); err != nil {
		respondLocalizedError(c, http.StatusConflict, "该任务当前不能取消", "The job cannot be canceled in its current state")
		return
	}
	download.Cancel(id)
	c.Status(http.StatusNoContent)
}

// moveDownloadJob moves the largest video of a completed job into a library
// directory. The copy runs in the background because it may cross filesystems.
func moveDownloadJob(c *gin.Context) {
	id, ok := downloadJobID(c)
	if !ok {
		return
	}
	var request struct {
		DirectoryID int64 `json:"directory_id"`
	}
	if bindErr := c.ShouldBindJSON(&request); bindErr != nil && !errors.Is(bindErr, io.EOF) {
		respondLocalizedError(c, http.StatusBadRequest, "移动请求格式不正确", "Invalid move request")
		return
	}
	job, err := dbpkg.Default().GetDownloadJob(c.Request.Context(), id)
	if err != nil || job == nil {
		respondLocalizedError(c, http.StatusNotFound, "下载任务不存在", "Download job not found")
		return
	}
	if job.Status != models.DownloadCompleted {
		respondLocalizedError(c, http.StatusConflict, "只能移动已完成的下载", "Only completed downloads can be moved")
		return
	}
	if job.MoveStatus == "moving" {
		respondLocalizedError(c, http.StatusConflict, "正在移动文件，请稍候", "The file is already being moved")
		return
	}
	if strings.TrimSpace(job.MovedPath) != "" {
		respondLocalizedError(c, http.StatusConflict, "该文件已移入媒体库", "The file has already been moved")
		return
	}
	target, err := selectMediaDirectory(c.Request.Context(), request.DirectoryID)
	if err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "请先在目录管理中添加媒体目录", "Add a media directory in Directory Management first")
		return
	}
	if err := dbpkg.Default().UpdateDownloadJob(c.Request.Context(), job.ID, map[string]any{
		"move_status": "moving", "move_error": "",
	}); err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "启动移动任务失败", "Failed to start the move")
		return
	}

	go func(jobID int64, directoryModel models.Directory) {
		moveCtx, cancel := context.WithTimeout(context.Background(), 6*time.Hour)
		defer cancel()
		movedPath, moveErr := download.MoveLargestVideo(moveCtx, jobID, directoryModel.Path)
		if moveErr != nil {
			logging.Error("move download job %d failed: %v", jobID, moveErr)
			return
		}
		logging.Info("moved download job %d to %s", jobID, movedPath)
		refreshed, loadErr := dbpkg.Default().GetDirectory(context.Background(), directoryModel.ID)
		if loadErr != nil || refreshed == nil {
			return
		}
		if scanErr := directory.StartManualDirectoryScan(*refreshed); scanErr != nil {
			logging.Info("scan after move skipped directory=%d: %v", directoryModel.ID, scanErr)
		}
	}(job.ID, *target)

	job.MoveStatus = "moving"
	job.MoveError = ""
	c.JSON(http.StatusAccepted, job)
}

func selectMediaDirectory(ctx context.Context, directoryID int64) (*models.Directory, error) {
	dirs, err := dbpkg.Default().ListDirectories(ctx)
	if err != nil {
		return nil, err
	}
	for index := range dirs {
		dir := dirs[index]
		if dir.IsDelete || !dir.Enabled {
			continue
		}
		if directoryID > 0 && dir.ID != directoryID {
			continue
		}
		return &dir, nil
	}
	return nil, errors.New("no media directory")
}

func revealDownloadLocation(c *gin.Context) {
	if isRemoteRequest(c.Request.RemoteAddr) {
		respondLocalizedError(c, http.StatusForbidden, "通过局域网访问时无法打开下载位置", "Cannot reveal download locations when accessing over the local network")
		return
	}
	if common.ContainerMode() {
		respondLocalizedError(c, http.StatusNotImplemented, "当前部署模式已禁用打开下载位置", "Desktop folder revealing is disabled")
		return
	}
	id, ok := downloadJobID(c)
	if !ok {
		return
	}
	job, err := dbpkg.Default().GetDownloadJob(c.Request.Context(), id)
	if err != nil {
		respondLocalizedError(c, http.StatusNotFound, "下载任务不存在", "Download job not found")
		return
	}
	target := downloadRevealTarget(job)
	info, err := os.Stat(target)
	if err != nil {
		respondLocalizedError(c, http.StatusNotFound, "下载位置不存在", "Download location does not exist")
		return
	}
	if info.IsDir() {
		err = util.OpenFile(target)
	} else {
		err = util.RevealFile(target)
	}
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "打开下载位置失败", "Failed to reveal download location")
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func downloadRevealTarget(job *models.DownloadJob) string {
	if job == nil {
		return ""
	}
	root := filepath.Clean(strings.TrimSpace(job.DownloadDirectory))
	var files []string
	_ = json.Unmarshal([]byte(job.LocalFilesJSON), &files)
	for _, candidate := range files {
		candidate = filepath.Clean(strings.TrimSpace(candidate))
		relative, err := filepath.Rel(root, candidate)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return root
}

func deleteDownloadJob(c *gin.Context) {
	id, ok := downloadJobID(c)
	if !ok {
		return
	}
	if err := dbpkg.Default().DeleteDownloadJob(c.Request.Context(), id); err != nil {
		respondLocalizedError(c, http.StatusConflict, "只能删除已结束的下载任务", "Only finished download jobs can be deleted")
		return
	}
	c.Status(http.StatusNoContent)
}

func downloadJobID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondLocalizedError(c, http.StatusBadRequest, "下载任务 ID 不正确", "Invalid download job ID")
		return 0, false
	}
	return id, true
}

// listDownloaderProviders probes the local machine for usable downloaders.
// Detection is best-effort; unreachable providers are returned with detected=false.
func listDownloaderProviders(c *gin.Context) {
	providers, active := download.DetectProviders(c.Request.Context())
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{
		"active_provider":  active,
		"default_provider": models.DefaultDownloaderProvider,
		"providers":        providers,
	})
}

func downloaderProviderParam(c *gin.Context) (string, bool) {
	provider := strings.TrimSpace(c.Param("provider"))
	if !models.IsSupportedDownloaderProvider(provider) {
		respondLocalizedError(c, http.StatusNotFound, "下载器类型不受支持", "Unsupported downloader provider")
		return "", false
	}
	return provider, true
}

func updateDownloaderActiveProvider(c *gin.Context) {
	var request struct {
		Provider string `json:"provider"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "下载器配置格式不正确", "Invalid downloader settings")
		return
	}
	provider := strings.TrimSpace(request.Provider)
	if !models.IsSupportedDownloaderProvider(provider) {
		respondLocalizedError(c, http.StatusBadRequest, "下载器类型不受支持", "Unsupported downloader provider")
		return
	}
	settings, err := dbpkg.Default().GetDownloaderSettings(c.Request.Context())
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "读取下载器配置失败", "Failed to load downloader settings")
		return
	}
	settings.ActiveProvider = provider
	if err := dbpkg.Default().SaveDownloaderSettings(c.Request.Context(), settings); err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "保存下载器配置失败", "Failed to save downloader settings")
		return
	}
	download.Wake()
	payload, err := loadDownloaderSettingsPayload(c.Request.Context())
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "读取下载器配置失败", "Failed to load downloader settings")
		return
	}
	c.JSON(http.StatusOK, payload)
}

func getDownloaderProvider(c *gin.Context) {
	provider, ok := downloaderProviderParam(c)
	if !ok {
		return
	}
	settings, err := dbpkg.Default().GetDownloaderProviderSettings(c.Request.Context(), provider)
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "读取下载器配置失败", "Failed to load downloader settings")
		return
	}
	address := settings.Address
	if response, lookupErr := loadDownloaderSettingsPayload(c.Request.Context()); lookupErr == nil && provider == models.DownloaderProviderCloudDrive2 {
		address = response.Address
	}
	c.JSON(http.StatusOK, gin.H{
		"provider":         provider,
		"address":          address,
		"username":         settings.Username,
		"remote_folder":    settings.RemoteFolder,
		"token_configured": strings.TrimSpace(settings.APIToken) != "",
	})
}

type downloaderProviderRequest struct {
	Address       *string `json:"address"`
	Username      *string `json:"username"`
	APIToken      *string `json:"api_token"`
	ClearAPIToken bool    `json:"clear_api_token"`
	RemoteFolder  *string `json:"remote_folder"`
}

func (request downloaderProviderRequest) draft(provider string, current *models.DownloaderProviderSettings) (*models.DownloaderProviderSettings, error) {
	draft := &models.DownloaderProviderSettings{Provider: provider}
	if current != nil {
		*draft = *current
	}
	if request.Address != nil {
		draft.Address = strings.TrimSpace(*request.Address)
	}
	if request.Username != nil {
		draft.Username = strings.TrimSpace(*request.Username)
	}
	if request.RemoteFolder != nil {
		draft.RemoteFolder = strings.TrimSpace(*request.RemoteFolder)
	}
	if request.ClearAPIToken {
		draft.APIToken = ""
	} else if request.APIToken != nil {
		draft.APIToken = strings.TrimSpace(*request.APIToken)
	}
	draft.Provider = provider
	if len(draft.Address) > 500 || len(draft.RemoteFolder) > 2000 || len(draft.Username) > 200 || len(draft.APIToken) > 16384 {
		return nil, errors.New("downloader settings are too long")
	}
	return draft, nil
}

func updateDownloaderProvider(c *gin.Context) {
	provider, ok := downloaderProviderParam(c)
	if !ok {
		return
	}
	var request downloaderProviderRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "下载器配置格式不正确", "Invalid provider settings")
		return
	}
	current, err := dbpkg.Default().GetDownloaderProviderSettings(c.Request.Context(), provider)
	if err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "读取下载器配置失败", "Failed to load downloader settings")
		return
	}
	draft, err := request.draft(provider, current)
	if err != nil {
		respondLocalizedError(c, http.StatusBadRequest, "下载器配置过长", "Downloader settings are too long")
		return
	}
	if err := dbpkg.Default().SaveDownloaderProviderSettings(c.Request.Context(), draft); err != nil {
		respondLocalizedError(c, http.StatusInternalServerError, "保存下载器配置失败", "Failed to save provider settings")
		return
	}
	getDownloaderProvider(c)
}

// testDownloaderProvider accepts an optional draft body. Without a body it
// tests the saved settings.
func testDownloaderProvider(c *gin.Context) {
	provider, ok := downloaderProviderParam(c)
	if !ok {
		return
	}
	var request downloaderProviderRequest
	bindErr := c.ShouldBindJSON(&request)
	if bindErr != nil && !errors.Is(bindErr, io.EOF) {
		respondLocalizedError(c, http.StatusBadRequest, "下载器配置格式不正确", "Invalid provider settings")
		return
	}
	current, err := dbpkg.Default().GetDownloaderProviderSettings(c.Request.Context(), provider)
	if err != nil {
		current = &models.DownloaderProviderSettings{Provider: provider}
	}
	draft := current
	if bindErr == nil {
		draft, err = request.draft(provider, current)
		if err != nil {
			respondLocalizedError(c, http.StatusBadRequest, "下载器配置过长", "Downloader settings are too long")
			return
		}
	}
	if strings.TrimSpace(draft.Address) == "" {
		respondLocalizedError(c, http.StatusBadRequest, "请填写下载器地址", "Enter the downloader address")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	result, err := download.TestDownloaderWithSettings(ctx, draft)
	if err != nil {
		respondLocalizedError(c, http.StatusBadGateway, "下载器连接失败："+err.Error(), "Downloader connection failed: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, result)
}
