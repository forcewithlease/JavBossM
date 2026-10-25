package subtitle

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"javboss/internal/util"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
)

// Track describes one subtitle stream that can be attached to a video.
type Track struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Language string `json:"language,omitempty"`
	Source   string `json:"source"`
	Format   string `json:"format"`
	Default  bool   `json:"default"`
}

var (
	ErrTrackNotFound     = errors.New("subtitle track not found")
	ErrFFmpegUnavailable = errors.New("ffmpeg is required for this subtitle")

	subtitleExtensions = map[string]bool{".srt": true, ".ass": true, ".ssa": true, ".vtt": true}
	textSubtitleCodecs = map[string]bool{
		"subrip": true, "srt": true, "ass": true, "ssa": true, "mov_text": true,
		"webvtt": true, "text": true,
	}
	timestampPattern = regexp.MustCompile(`(?m)^(\d{1,2}:\d{2}:\d{2}),(\d{3})(\s*-->\s*)(\d{1,2}:\d{2}:\d{2}),(\d{3})`)
)

// Discover lists local subtitle files next to the video, then embedded text
// tracks. Local files are preferred because they usually keep Chinese styling.
func Discover(ctx context.Context, videoPath, dataDir string) ([]Track, error) {
	tracks := discoverLocal(videoPath)
	embedded, err := discoverEmbedded(ctx, videoPath)
	if err == nil {
		tracks = append(tracks, embedded...)
	}
	if len(tracks) > 0 {
		tracks[0].Default = true
	}
	return tracks, nil
}

func discoverLocal(videoPath string) []Track {
	dir := filepath.Dir(videoPath)
	stem := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	lowerStem := strings.ToLower(stem)
	tracks := make([]Track, 0, 4)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if !subtitleExtensions[ext] {
			continue
		}
		base := strings.TrimSuffix(name, filepath.Ext(name))
		lowerBase := strings.ToLower(base)
		if lowerBase != lowerStem && !strings.HasPrefix(lowerBase, lowerStem+".") &&
			!strings.HasPrefix(lowerBase, lowerStem+"_") && !strings.HasPrefix(lowerBase, lowerStem+"-") {
			continue
		}
		language, label := detectLanguage(strings.TrimPrefix(lowerBase, lowerStem))
		if label == "" {
			label = "字幕"
		}
		tracks = append(tracks, Track{
			ID:       "local:" + name,
			Label:    label,
			Language: language,
			Source:   "local",
			Format:   strings.TrimPrefix(ext, "."),
		})
	}
	sort.SliceStable(tracks, func(i, j int) bool {
		return languageRank(tracks[i].Language) < languageRank(tracks[j].Language)
	})
	return tracks
}

type probeResult struct {
	Streams []struct {
		Index     int               `json:"index"`
		CodecName string            `json:"codec_name"`
		Tags      map[string]string `json:"tags"`
	} `json:"streams"`
}

func discoverEmbedded(ctx context.Context, videoPath string) ([]Track, error) {
	ffprobe, err := util.ResolveFFprobePath()
	if err != nil {
		return nil, err
	}
	probeCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(probeCtx, ffprobe,
		"-v", "error",
		"-select_streams", "s",
		"-show_entries", "stream=index,codec_name:stream_tags=language,title",
		"-of", "json",
		videoPath,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("probe subtitles: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var probe probeResult
	if err := json.Unmarshal(stdout.Bytes(), &probe); err != nil {
		return nil, fmt.Errorf("decode subtitle probe: %w", err)
	}
	tracks := make([]Track, 0, len(probe.Streams))
	for _, stream := range probe.Streams {
		if !textSubtitleCodecs[strings.ToLower(stream.CodecName)] {
			continue
		}
		label := strings.TrimSpace(stream.Tags["title"])
		language := normalizeLanguage(stream.Tags["language"])
		if label == "" {
			label = languageLabel(language)
		}
		if label == "" {
			label = "内挂字幕"
		}
		tracks = append(tracks, Track{
			ID:       fmt.Sprintf("embedded:%d", stream.Index),
			Label:    label,
			Language: language,
			Source:   "embedded",
			Format:   "embedded",
		})
	}
	return tracks, nil
}

// Render returns WebVTT bytes for a discovered track. Local SRT/VTT files are
// converted in memory; ASS/SSA and embedded streams are converted with ffmpeg
// and cached under dataDir.
func Render(ctx context.Context, videoPath, trackID, dataDir string) ([]byte, error) {
	switch {
	case strings.HasPrefix(trackID, "local:"):
		return renderLocal(ctx, videoPath, strings.TrimPrefix(trackID, "local:"), dataDir)
	case strings.HasPrefix(trackID, "embedded:"):
		return renderEmbedded(ctx, videoPath, strings.TrimPrefix(trackID, "embedded:"), dataDir)
	default:
		return nil, ErrTrackNotFound
	}
}

func renderLocal(ctx context.Context, videoPath, name, dataDir string) ([]byte, error) {
	if name == "" || filepath.Base(name) != name {
		return nil, ErrTrackNotFound
	}
	ext := strings.ToLower(filepath.Ext(name))
	if !subtitleExtensions[ext] {
		return nil, ErrTrackNotFound
	}
	path := filepath.Join(filepath.Dir(videoPath), name)
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, ErrTrackNotFound
	}
	decoded := decodeText(raw)
	switch ext {
	case ".vtt":
		if bytes.HasPrefix(bytes.TrimSpace(decoded), []byte("WEBVTT")) {
			return decoded, nil
		}
		return []byte("WEBVTT\n\n" + string(decoded)), nil
	case ".srt":
		return srtToVTT(decoded), nil
	default:
		return convertWithFFmpeg(ctx, videoPath, path, "", dataDir)
	}
}

func renderEmbedded(ctx context.Context, videoPath, index, dataDir string) ([]byte, error) {
	if index == "" || strings.ContainsAny(index, ":/\\") {
		return nil, ErrTrackNotFound
	}
	return convertWithFFmpeg(ctx, videoPath, videoPath, index, dataDir)
}

// convertWithFFmpeg converts either an external subtitle file (sourcePath) or
// an embedded stream (streamIndex) to WebVTT. Results are cached by file state.
func convertWithFFmpeg(ctx context.Context, videoPath, sourcePath, streamIndex, dataDir string) ([]byte, error) {
	ffmpeg, err := util.ResolveFFmpegPath()
	if err != nil {
		return nil, ErrFFmpegUnavailable
	}
	cachePath := ""
	if dataDir != "" {
		info, statErr := os.Stat(sourcePath)
		if statErr == nil {
			key := fmt.Sprintf("%s|%d|%d|%s", sourcePath, info.Size(), info.ModTime().UnixNano(), streamIndex)
			sum := sha1.Sum([]byte(key))
			cachePath = filepath.Join(dataDir, "cache", "subtitles", hex.EncodeToString(sum[:])+".vtt")
			if cached, readErr := os.ReadFile(cachePath); readErr == nil && len(cached) > 0 {
				return cached, nil
			}
		}
	}

	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-i", videoPath}
	if sourcePath != videoPath {
		args = []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-i", sourcePath}
		args = append(args, "-map", "0:s:0")
	} else {
		args = append(args, "-map", "0:"+streamIndex)
	}
	args = append(args, "-c:s", "webvtt", "-f", "webvtt", "pipe:1")

	convertCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(convertCtx, ffmpeg, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("convert subtitle: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	output := stdout.Bytes()
	if len(bytes.TrimSpace(output)) == 0 {
		return nil, ErrTrackNotFound
	}
	if cachePath != "" {
		_ = os.MkdirAll(filepath.Dir(cachePath), 0o755)
		temp := cachePath + ".tmp"
		if writeErr := os.WriteFile(temp, output, 0o644); writeErr == nil {
			_ = os.Rename(temp, cachePath)
		}
	}
	return output, nil
}

func decodeText(raw []byte) []byte {
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	if utf8.Valid(raw) {
		return raw
	}
	for _, candidate := range []encoding.Encoding{simplifiedchinese.GB18030, traditionalchinese.Big5} {
		decoded, err := candidate.NewDecoder().Bytes(raw)
		if err == nil && utf8.Valid(decoded) && !bytes.ContainsRune(decoded, '\uFFFD') {
			return decoded
		}
	}
	return raw
}

func srtToVTT(raw []byte) []byte {
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = timestampPattern.ReplaceAllString(text, "$1.$2$3$4.$5")
	text = strings.TrimLeft(text, "\n")
	return []byte("WEBVTT\n\n" + text)
}

func detectLanguage(rest string) (string, string) {
	tokens := strings.FieldsFunc(strings.ToLower(rest), func(r rune) bool {
		return r == '.' || r == '_' || r == '-' || r == ' ' || r == '[' || r == ']' || r == '(' || r == ')'
	})
	for _, token := range tokens {
		switch token {
		case "zh", "cn", "chs", "sc", "zhcn", "zh-cn", "ch", "chi", "简体", "简中", "中文":
			return "zh-CN", "简体中文"
		case "cht", "tc", "zhtw", "zh-tw", "tw", "繁", "繁体", "繁中":
			return "zh-TW", "繁體中文"
		case "en", "eng", "english":
			return "en", "English"
		case "ja", "jp", "jpn", "japanese":
			return "ja", "日本語"
		case "ko", "kor", "korean":
			return "ko", "한국어"
		}
	}
	return "", ""
}

func normalizeLanguage(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "zh", "chi", "zho", "chs", "zh-cn", "cn":
		return "zh-CN"
	case "cht", "zh-tw", "tw":
		return "zh-TW"
	case "en", "eng":
		return "en"
	case "ja", "jp", "jpn":
		return "ja"
	case "ko", "kor":
		return "ko"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func languageLabel(language string) string {
	switch language {
	case "zh-CN":
		return "简体中文"
	case "zh-TW":
		return "繁體中文"
	case "en":
		return "English"
	case "ja":
		return "日本語"
	case "ko":
		return "한국어"
	default:
		return strings.TrimSpace(language)
	}
}

func languageRank(language string) int {
	switch language {
	case "zh-CN":
		return 0
	case "zh-TW":
		return 1
	case "zh":
		return 2
	case "en":
		return 3
	case "ja":
		return 4
	case "":
		return 6
	default:
		return 5
	}
}
