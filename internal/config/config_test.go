package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsePayload_Valid(t *testing.T) {
	data := `{
		"job_id": "clip_007",
		"assets": {
			"source_video": "/tmp/video.mp4",
			"subtitle_file": "/tmp/sub.ass"
		},
		"config": {
			"trim": { "start_sec": 10, "end_sec": 20 },
			"canvas": { "width": 1080, "height": 1920, "bg_mode": "blur" },
			"hook": {
				"text": "Hello",
				"font_file": "/tmp/font.ttf",
				"font_size": 72,
				"font_color": "white",
				"y_position": 200
			}
		},
		"output_path": "/tmp/out.mp4"
	}`
	p, err := ParsePayload([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if p.JobID != "clip_007" {
		t.Errorf("expected clip_007, got %s", p.JobID)
	}
	if p.Config.Canvas.Width != 1080 {
		t.Errorf("expected 1080, got %d", p.Config.Canvas.Width)
	}
}

func TestParsePayload_InvalidJSON(t *testing.T) {
	_, err := ParsePayload([]byte(`{bad json}`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParsePayload_Empty(t *testing.T) {
	_, err := ParsePayload([]byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidatePayload_MissingJobID(t *testing.T) {
	err := ValidatePayload(JobPayload{})
	if err == nil {
		t.Fatal("expected error for missing job_id")
	}
}

func TestValidatePayload_MissingSourceVideo(t *testing.T) {
	p := JobPayload{JobID: "test"}
	err := ValidatePayload(p)
	if err == nil || err.Error() != "source_video is required" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePayload_MissingSubtitleFile(t *testing.T) {
	p := JobPayload{JobID: "test", Assets: Assets{SourceVideo: "/tmp/ok.mp4"}}
	err := ValidatePayload(p)
	if err == nil || err.Error() != "subtitle_file is required" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePayload_MissingOutputPath(t *testing.T) {
	p := JobPayload{JobID: "test", Assets: Assets{SourceVideo: "/tmp/ok.mp4", SubtitleFile: "/tmp/ok.ass"}}
	err := ValidatePayload(p)
	if err == nil || err.Error() != "output_path is required" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePayload_SourceVideoNotFound(t *testing.T) {
	p := JobPayload{
		JobID:      "test",
		Assets:     Assets{SourceVideo: "/nonexistent/video.mp4", SubtitleFile: "/nonexistent/sub.ass"},
		OutputPath: "/tmp/out.mp4",
	}
	err := ValidatePayload(p)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestValidatePayload_SubtitleFileNotFound(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "video.mp4")
	os.WriteFile(video, []byte("x"), 0644)

	p := JobPayload{
		JobID:      "test",
		Assets:     Assets{SourceVideo: video, SubtitleFile: "/nonexistent/sub.ass"},
		OutputPath: "/tmp/out.mp4",
	}
	err := ValidatePayload(p)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestValidatePayload_FontFileOptional(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "video.mp4")
	sub := filepath.Join(dir, "sub.ass")
	os.WriteFile(video, []byte("x"), 0644)
	os.WriteFile(sub, []byte("x"), 0644)

	p := JobPayload{
		JobID:      "test",
		Assets:     Assets{SourceVideo: video, SubtitleFile: sub},
		OutputPath: "/tmp/out.mp4",
		Config: Config{
			Trim:   TrimConfig{StartSec: 0, EndSec: 10},
			Canvas: CanvasConfig{Width: 1080, Height: 1920, BgMode: "blur"},
			Hook:   HookConfig{Text: "test", FontSize: 72},
		},
	}
	err := ValidatePayload(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePayload_FontFileNotFound(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "video.mp4")
	sub := filepath.Join(dir, "sub.ass")
	os.WriteFile(video, []byte("x"), 0644)
	os.WriteFile(sub, []byte("x"), 0644)

	p := JobPayload{
		JobID:      "test",
		Assets:     Assets{SourceVideo: video, SubtitleFile: sub},
		OutputPath: "/tmp/out.mp4",
		Config: Config{
			Hook: HookConfig{FontFile: "/nonexistent/font.ttf", FontSize: 72},
		},
	}
	err := ValidatePayload(p)
	if err == nil {
		t.Fatal("expected error for missing font_file")
	}
}

func TestValidatePayload_NegativeStart(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "video.mp4")
	sub := filepath.Join(dir, "sub.ass")
	os.WriteFile(video, []byte("x"), 0644)
	os.WriteFile(sub, []byte("x"), 0644)

	p := JobPayload{
		JobID:      "test",
		Assets:     Assets{SourceVideo: video, SubtitleFile: sub},
		OutputPath: "/tmp/out.mp4",
		Config: Config{
			Trim:   TrimConfig{StartSec: -1, EndSec: 10},
			Canvas: CanvasConfig{Width: 1080, Height: 1920},
		},
	}
	err := ValidatePayload(p)
	if err == nil {
		t.Fatal("expected error for negative start_sec")
	}
}

func TestValidatePayload_EndBeforeStart(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "video.mp4")
	sub := filepath.Join(dir, "sub.ass")
	os.WriteFile(video, []byte("x"), 0644)
	os.WriteFile(sub, []byte("x"), 0644)

	p := JobPayload{
		JobID:      "test",
		Assets:     Assets{SourceVideo: video, SubtitleFile: sub},
		OutputPath: "/tmp/out.mp4",
		Config: Config{
			Trim:   TrimConfig{StartSec: 20, EndSec: 10},
			Canvas: CanvasConfig{Width: 1080, Height: 1920},
		},
	}
	err := ValidatePayload(p)
	if err == nil {
		t.Fatal("expected error for end_sec < start_sec")
	}
}

func TestValidatePayload_ZeroCanvasDimensions(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "video.mp4")
	sub := filepath.Join(dir, "sub.ass")
	os.WriteFile(video, []byte("x"), 0644)
	os.WriteFile(sub, []byte("x"), 0644)

	p := JobPayload{
		JobID:      "test",
		Assets:     Assets{SourceVideo: video, SubtitleFile: sub},
		OutputPath: "/tmp/out.mp4",
		Config: Config{
			Trim:   TrimConfig{StartSec: 0, EndSec: 10},
			Canvas: CanvasConfig{Width: 0, Height: 0},
		},
	}
	err := ValidatePayload(p)
	if err == nil {
		t.Fatal("expected error for zero canvas dimensions")
	}
}

func TestValidatePayload_UnsupportedBgMode(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "video.mp4")
	sub := filepath.Join(dir, "sub.ass")
	os.WriteFile(video, []byte("x"), 0644)
	os.WriteFile(sub, []byte("x"), 0644)

	p := JobPayload{
		JobID:      "test",
		Assets:     Assets{SourceVideo: video, SubtitleFile: sub},
		OutputPath: "/tmp/out.mp4",
		Config: Config{
			Trim:   TrimConfig{StartSec: 0, EndSec: 10},
			Canvas: CanvasConfig{Width: 1080, Height: 1920, BgMode: "invalid"},
		},
	}
	err := ValidatePayload(p)
	if err == nil {
		t.Fatal("expected error for unsupported bg_mode")
	}
}

func TestValidatePayload_ValidFull(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "video.mp4")
	sub := filepath.Join(dir, "sub.ass")
	font := filepath.Join(dir, "font.ttf")
	os.WriteFile(video, []byte("x"), 0644)
	os.WriteFile(sub, []byte("x"), 0644)
	os.WriteFile(font, []byte("x"), 0644)

	p := JobPayload{
		JobID:      "test",
		Assets:     Assets{SourceVideo: video, SubtitleFile: sub},
		OutputPath: "/tmp/out.mp4",
		Config: Config{
			Trim:   TrimConfig{StartSec: 10.5, EndSec: 20.5},
			Canvas: CanvasConfig{Width: 1080, Height: 1920, BgMode: "blur"},
			Hook: HookConfig{
				Text: "Hello", FontFile: font, FontSize: 72, FontColor: "white", YPosition: 200,
			},
		},
	}
	err := ValidatePayload(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateThumbnailConfig_InvalidPosition(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "video.mp4")
	sub := filepath.Join(dir, "sub.ass")
	os.WriteFile(video, []byte("x"), 0644)
	os.WriteFile(sub, []byte("x"), 0644)

	p := JobPayload{
		JobID:      "test",
		Assets:     Assets{SourceVideo: video, SubtitleFile: sub},
		OutputPath: "/tmp/out.mp4",
		Config: Config{
			Trim:      TrimConfig{StartSec: 1, EndSec: 5},
			Canvas:    CanvasConfig{Width: 1080, Height: 1920},
			Thumbnail: ThumbnailConfig{Position: "somewhere-else"},
		},
	}
	err := ValidatePayload(p)
	if err == nil || err.Error() != "unsupported thumbnail.position: somewhere-else" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateThumbnailConfig_ValidPositions(t *testing.T) {
	positions := []string{
		"auto", "center",
		"top-left", "top-center", "top-right",
		"middle-left", "middle-center", "middle-right",
		"bottom-left", "bottom-center", "bottom-right",
	}
	for _, pos := range positions {
		if !ValidThumbPosition(pos) && pos != "auto" {
			t.Errorf("position %s should be valid", pos)
		}
	}
}
