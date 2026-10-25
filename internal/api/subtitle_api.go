package api

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"

	"javboss/internal/common"
	"javboss/internal/subtitle"

	"github.com/gin-gonic/gin"
)

// listVideoSubtitles discovers local subtitle files and embedded text tracks.
func listVideoSubtitles(c *gin.Context) {
	_, fullPath, _, err := resolveVideoStreamTarget(c)
	if err != nil {
		respondPlaybackError(c, err)
		return
	}
	tracks, err := subtitle.Discover(c.Request.Context(), fullPath, subtitleCacheDir())
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"tracks": []subtitle.Track{}})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"tracks": tracks})
}

// getVideoSubtitle serves one track as WebVTT.
func getVideoSubtitle(c *gin.Context) {
	_, fullPath, _, err := resolveVideoStreamTarget(c)
	if err != nil {
		respondPlaybackError(c, err)
		return
	}
	content, err := subtitle.Render(
		c.Request.Context(),
		fullPath,
		c.Param("track"),
		subtitleCacheDir(),
	)
	if err != nil {
		switch {
		case errors.Is(err, subtitle.ErrTrackNotFound):
			respondLocalizedError(c, http.StatusNotFound, "字幕不存在", "Subtitle track not found")
		case errors.Is(err, subtitle.ErrFFmpegUnavailable):
			respondLocalizedError(c, http.StatusServiceUnavailable, "需要安装 FFmpeg 才能转换该字幕", "FFmpeg is required to convert this subtitle")
		default:
			respondLocalizedError(c, http.StatusBadGateway, "字幕转换失败", "Failed to convert the subtitle")
		}
		return
	}
	c.Header("Cache-Control", "private, max-age=3600")
	c.Data(http.StatusOK, "text/vtt; charset=utf-8", content)
}

func subtitleCacheDir() string {
	if cfg := common.CurrentConfig(); cfg != nil {
		return filepath.Dir(cfg.DatabasePath)
	}
	return os.TempDir()
}
