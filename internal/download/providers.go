package download

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"javboss/internal/db"
	"javboss/internal/download/downloader"
	downloaderaria2 "javboss/internal/download/downloader/aria2"
	downloaderqbittorrent "javboss/internal/download/downloader/qbittorrent"
	"javboss/internal/models"
)

// ProviderInfo describes one selectable download provider and whether it can be
// reached from this machine right now.
type ProviderInfo struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	Implemented  bool   `json:"implemented"`
	Detected     bool   `json:"detected"`
	Address      string `json:"address"`
	Version      string `json:"version,omitempty"`
	RequiresAuth bool   `json:"requires_auth,omitempty"`
	Message      string `json:"message,omitempty"`
}

const detectTimeout = 1200 * time.Millisecond

var providerDefaults = map[string]string{
	models.DownloaderProviderCloudDrive2: "http://127.0.0.1:19798",
	models.DownloaderProviderAria2:       "http://127.0.0.1:6800",
	models.DownloaderProviderQBittorrent: "http://127.0.0.1:8080",
}

// DetectProviders probes reachable downloaders and returns them in a stable
// order. Detection never fails the caller: unreachable providers are reported
// with Detected=false.
func DetectProviders(ctx context.Context) ([]ProviderInfo, string) {
	settings, err := db.Default().GetDownloaderSettings(ctx)
	active := models.DefaultDownloaderProvider
	if err == nil && settings != nil {
		active = settings.ActiveProvider
	}
	providers := []ProviderInfo{
		{ID: models.DownloaderProviderCloudDrive2, Kind: "cloud", Implemented: true},
		{ID: models.DownloaderProviderAria2, Kind: "local", Implemented: true},
		{ID: models.DownloaderProviderQBittorrent, Kind: "local", Implemented: true},
	}
	var wg sync.WaitGroup
	for i := range providers {
		provider := &providers[i]
		stored, _ := db.Default().GetDownloaderProviderSettings(ctx, provider.ID)
		provider.Address = providerDefaults[provider.ID]
		if stored != nil && strings.TrimSpace(stored.Address) != "" {
			provider.Address = strings.TrimSpace(stored.Address)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			probeCtx, cancel := context.WithTimeout(ctx, detectTimeout)
			defer cancel()
			switch provider.ID {
			case models.DownloaderProviderCloudDrive2:
				detectCloudDrive2(probeCtx, provider)
			case models.DownloaderProviderAria2:
				detectAria2(probeCtx, provider, stored)
			case models.DownloaderProviderQBittorrent:
				detectQBittorrent(probeCtx, provider, stored)
			}
		}()
	}
	wg.Wait()
	return providers, active
}

func detectCloudDrive2(ctx context.Context, provider *ProviderInfo) {
	parsed, err := url.Parse(provider.Address)
	if err != nil {
		provider.Message = err.Error()
		return
	}
	host := parsed.Host
	if !strings.Contains(host, ":") {
		host = net.JoinHostPort(host, "19798")
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", host)
	if err != nil {
		provider.Message = "unreachable"
		return
	}
	_ = conn.Close()
	provider.Detected = true
}

func detectAria2(ctx context.Context, provider *ProviderInfo, stored *models.DownloaderProviderSettings) {
	token := ""
	if stored != nil {
		token = stored.APIToken
	}
	client, err := downloaderaria2.New(provider.Address, token)
	if err != nil {
		provider.Message = err.Error()
		return
	}
	result, err := client.Test(ctx)
	if errors.Is(err, downloaderaria2.ErrUnauthorized) {
		provider.Detected = true
		provider.RequiresAuth = true
		return
	}
	if err != nil {
		provider.Message = "unreachable"
		return
	}
	provider.Detected = true
	if result != nil && result.Details != nil {
		if version, ok := result.Details["version"].(string); ok {
			provider.Version = version
		}
	}
}

func detectQBittorrent(ctx context.Context, provider *ProviderInfo, stored *models.DownloaderProviderSettings) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(provider.Address, "/")+"/api/v2/app/version", nil)
	if err != nil {
		provider.Message = err.Error()
		return
	}
	response, err := (&http.Client{Timeout: detectTimeout}).Do(request)
	if err != nil {
		provider.Message = "unreachable"
		return
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusOK:
		provider.Detected = true
	case http.StatusForbidden:
		provider.Detected = true
		provider.RequiresAuth = true
	default:
		provider.Message = fmt.Sprintf("HTTP %d", response.StatusCode)
	}
}

func openLocalClient(settings *models.DownloaderProviderSettings) (downloader.LocalClient, error) {
	if settings == nil {
		return nil, errors.New("downloader settings are missing")
	}
	address := strings.TrimSpace(settings.Address)
	if address == "" {
		address = providerDefaults[settings.Provider]
	}
	switch settings.Provider {
	case models.DownloaderProviderAria2:
		return downloaderaria2.New(address, strings.TrimSpace(settings.APIToken))
	case models.DownloaderProviderQBittorrent:
		return downloaderqbittorrent.New(address, settings.Username, settings.APIToken)
	default:
		return nil, fmt.Errorf("unsupported local downloader %q", settings.Provider)
	}
}
