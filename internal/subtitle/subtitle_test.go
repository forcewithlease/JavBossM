package subtitle

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func TestDiscoverLocalTracks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	video := filepath.Join(dir, "ABC-123.mp4")
	writeFile(t, video, "video")
	writeFile(t, filepath.Join(dir, "ABC-123.srt"), "1\n00:00:01,000 --> 00:00:02,000\nHello\n")
	writeFile(t, filepath.Join(dir, "ABC-123.zh.srt"), "1\n00:00:01,000 --> 00:00:02,000\n你好\n")
	writeFile(t, filepath.Join(dir, "ABC-123.cht.ass"), "[Script Info]\n")
	writeFile(t, filepath.Join(dir, "OTHER.srt"), "1\n00:00:01,000 --> 00:00:02,000\nOther\n")

	tracks := discoverLocal(video)
	if len(tracks) != 3 {
		t.Fatalf("expected 3 local tracks, got %d: %#v", len(tracks), tracks)
	}
	if tracks[0].Language != "zh-CN" || tracks[0].Label != "简体中文" {
		t.Fatalf("unexpected first track: %#v", tracks[0])
	}
	if tracks[0].ID != "local:ABC-123.zh.srt" {
		t.Fatalf("unexpected track id: %q", tracks[0].ID)
	}
	if tracks[1].Language != "zh-TW" {
		t.Fatalf("unexpected second track: %#v", tracks[1])
	}
	if tracks[2].Language != "" || tracks[2].Label != "字幕" {
		t.Fatalf("unexpected third track: %#v", tracks[2])
	}
}

func TestSRTToVTT(t *testing.T) {
	t.Parallel()
	input := []byte("\uFEFF1\r\n00:00:01,000 --> 00:00:02,500\r\nHello\r\n\r\n2\r\n00:00:03,000 --> 00:00:04,000\r\nWorld\r\n")
	output := string(srtToVTT(decodeText(input)))
	if !strings.HasPrefix(output, "WEBVTT\n\n") {
		t.Fatalf("missing WEBVTT header: %q", output)
	}
	if !strings.Contains(output, "00:00:01.000 --> 00:00:02.500") {
		t.Fatalf("timestamp was not converted: %q", output)
	}
	if !strings.Contains(output, "Hello") || !strings.Contains(output, "World") {
		t.Fatalf("missing cue text: %q", output)
	}
}

func TestRenderLocalSRT(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	video := filepath.Join(dir, "ABC-123.mp4")
	writeFile(t, video, "video")
	writeFile(t, filepath.Join(dir, "ABC-123.zh.srt"), "1\n00:00:01,000 --> 00:00:02,000\n你好\n")

	content, err := Render(context.Background(), video, "local:ABC-123.zh.srt", dir)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "WEBVTT") || !strings.Contains(text, "你好") {
		t.Fatalf("unexpected subtitle: %q", text)
	}
}

func TestRenderLocalGBK(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	video := filepath.Join(dir, "ABC-123.mp4")
	writeFile(t, video, "video")
	encoded, err := simplifiedchinese.GB18030.NewEncoder().Bytes([]byte("1\n00:00:01,000 --> 00:00:02,000\n中文字幕\n"))
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ABC-123.srt"), encoded, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	content, err := Render(context.Background(), video, "local:ABC-123.srt", dir)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(string(content), "中文字幕") {
		t.Fatalf("GBK subtitle was not decoded: %q", string(content))
	}
}

func TestRenderRejectsEscapingPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	video := filepath.Join(dir, "ABC-123.mp4")
	writeFile(t, video, "video")
	if _, err := Render(context.Background(), video, "local:../secret.srt", dir); err != ErrTrackNotFound {
		t.Fatalf("expected ErrTrackNotFound, got %v", err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
