package builder

import (
	"strings"
	"testing"

	"github.com/godamri/mediaclip/internal/config"
)

func validPayload() config.JobPayload {
	return config.JobPayload{
		JobID: "test",
		Assets: config.Assets{
			SourceVideo:  "/tmp/video.mp4",
			SubtitleFile: "/tmp/sub.ass",
		},
		OutputPath: "/tmp/out.mp4",
		Config: config.Config{
			Trim: config.TrimConfig{StartSec: 10, EndSec: 20},
			Canvas: config.CanvasConfig{
				Width: 1080, Height: 1920, BgMode: "blur",
			},
			Hook: config.HookConfig{
				Text: "Hello World", FontFile: "/tmp/font.ttf",
				FontSize: 72, FontColor: "white", YPosition: 200,
			},
		},
	}
}

func TestBuildFFmpegArgs_Trim(t *testing.T) {
	p := validPayload()
	args := BuildFFmpegArgs(p)

	assertHas(t, args, "-ss", "10")
	assertHas(t, args, "-to", "20")
}

func TestBuildFFmpegArgs_InputOutput(t *testing.T) {
	p := validPayload()
	args := BuildFFmpegArgs(p)

	assertHas(t, args, "-i", "/tmp/video.mp4")
	assertHas(t, args, "-y", "/tmp/out.mp4")
}

func TestBuildFFmpegArgs_FilterComplex(t *testing.T) {
	p := validPayload()
	args := BuildFFmpegArgs(p)

	fc := getFlag(t, args, "-filter_complex")
	if fc == "" {
		t.Fatal("missing -filter_complex")
	}

	if !strings.Contains(fc, "boxblur=10:5") {
		t.Error("filter_complex missing boxblur")
	}
	if !strings.Contains(fc, "overlay=") {
		t.Error("filter_complex missing overlay")
	}
	if !strings.Contains(fc, "drawtext=") {
		t.Error("filter_complex missing drawtext")
	}
	if !strings.Contains(fc, "ass=") {
		t.Error("filter_complex missing ass")
	}
	if !strings.Contains(fc, "[out]") {
		t.Error("filter_complex missing [out] label")
	}
}

func TestBuildFFmpegArgs_NoTrim(t *testing.T) {
	p := validPayload()
	p.Config.Trim = config.TrimConfig{}
	args := BuildFFmpegArgs(p)

	if hasFlag(args, "-ss") {
		t.Error("unexpected -ss when no trim")
	}
	if hasFlag(args, "-to") {
		t.Error("unexpected -to when no trim")
	}
}

func TestBuildFFmpegArgs_EmptyHook(t *testing.T) {
	p := validPayload()
	p.Config.Hook = config.HookConfig{Text: ""}
	args := BuildFFmpegArgs(p)

	fc := getFlag(t, args, "-filter_complex")
	if strings.Contains(fc, "drawtext=") {
		t.Error("drawtext should be omitted when hook text is empty")
	}
}

func TestBuildFFmpegArgs_HookNoFont(t *testing.T) {
	p := validPayload()
	p.Config.Hook = config.HookConfig{Text: "test"}
	args := BuildFFmpegArgs(p)

	fc := getFlag(t, args, "-filter_complex")
	if strings.Contains(fc, "drawtext=") {
		t.Error("drawtext should be omitted when font_file is empty")
	}
}

func TestBuildFFmpegArgs_DrawtextContent(t *testing.T) {
	p := validPayload()
	args := BuildFFmpegArgs(p)

	fc := getFlag(t, args, "-filter_complex")
	if !strings.Contains(fc, "text='Hello World'") {
		t.Errorf("drawtext missing text, got: %s", fc)
	}
	if !strings.Contains(fc, "fontfile='/tmp/font.ttf'") {
		t.Errorf("drawtext missing fontfile, got: %s", fc)
	}
	if !strings.Contains(fc, "fontsize=72") {
		t.Errorf("drawtext missing fontsize, got: %s", fc)
	}
	if !strings.Contains(fc, "y=200") {
		t.Errorf("drawtext missing y position, got: %s", fc)
	}
}

func TestBuildFFmpegArgs_AssContent(t *testing.T) {
	p := validPayload()
	args := BuildFFmpegArgs(p)

	fc := getFlag(t, args, "-filter_complex")
	if !strings.Contains(fc, "filename='/tmp/sub.ass'") {
		t.Errorf("ass missing filename, got: %s", fc)
	}
}

func TestBuildFFmpegArgs_ScaleDimensions(t *testing.T) {
	p := validPayload()
	args := BuildFFmpegArgs(p)

	fc := getFlag(t, args, "-filter_complex")
	if !strings.Contains(fc, "scale=1080:1920") {
		t.Errorf("background scale missing canvas dimensions, got: %s", fc)
	}
}

func TestBuildFFmpegArgs_DefaultFontColor(t *testing.T) {
	p := validPayload()
	p.Config.Hook.FontColor = ""
	args := BuildFFmpegArgs(p)

	fc := getFlag(t, args, "-filter_complex")
	if !strings.Contains(fc, "fontcolor=white") {
		t.Errorf("expected default fontcolor=white, got: %s", fc)
	}
}

func TestBuildFFmpegArgs_NoBgMode(t *testing.T) {
	p := validPayload()
	p.Config.Canvas.BgMode = ""
	args := BuildFFmpegArgs(p)

	fc := getFlag(t, args, "-filter_complex")
	if strings.Contains(fc, "boxblur") {
		t.Error("boxblur should not be present when bg_mode is empty")
	}
}

func TestEscapeDrawText_SingleQuote(t *testing.T) {
	result := escapeDrawText("it's a test")
	if !strings.Contains(result, "'\\\\''") {
		t.Errorf("expected single quote to be escaped, got: %s", result)
	}
}

func TestBuildFFmpegArgs_MapFlags(t *testing.T) {
	p := validPayload()
	args := BuildFFmpegArgs(p)

	assertHas(t, args, "-map", "[out]")
	assertHas(t, args, "-map", "0:a?")
}

func TestBuildFFmpegArgs_Codecs(t *testing.T) {
	p := validPayload()
	args := BuildFFmpegArgs(p)

	assertHas(t, args, "-c:v", "libx264")
	assertHas(t, args, "-c:a", "aac")
}

func TestFormatFloat_WholeNumber(t *testing.T) {
	r := formatFloat(10.0)
	if r != "10" {
		t.Errorf("expected 10, got %s", r)
	}
}

func TestFormatFloat_Decimal(t *testing.T) {
	r := formatFloat(10.5)
	if r != "10.5" {
		t.Errorf("expected 10.5, got %s", r)
	}
}

func TestFormatFloat_Zero(t *testing.T) {
	r := formatFloat(0)
	if r != "0" {
		t.Errorf("expected 0, got %s", r)
	}
}

// helpers

func assertHas(t *testing.T, args []string, key, val string) {
	t.Helper()
	for i, a := range args {
		if a == key && i+1 < len(args) && args[i+1] == val {
			return
		}
	}
	t.Errorf("expected %s %s not found in args: %v", key, val, args)
}

func hasFlag(args []string, key string) bool {
	for i, a := range args {
		if a == key {
			_ = i
			return true
		}
	}
	return false
}

func getFlag(t *testing.T, args []string, key string) string {
	t.Helper()
	for i, a := range args {
		if a == key && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func TestEscapeFilterPath(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"/tmp/font.ttf", "/tmp/font.ttf"},
		{`C:\dir\file.ass`, `C\:\\dir\\file.ass`},
		{`C:\Users\Me\font.ttf`, `C\:\\Users\\Me\\font.ttf`},
	}
	for _, c := range cases {
		if got := escapeFilterPath(c.in); got != c.want {
			t.Errorf("escapeFilterPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBuildSectionSegmentArgs(t *testing.T) {
	s := config.SectionConfig{
		Image:        "/tmp/slide.jpg",
		Audio:        "/tmp/voice.mp3",
		SubtitleFile: "/tmp/voice.ass",
		BgFill:       "blur",
		Hook: config.HookConfig{
			Text: "donal trum makan cobek", FontFile: "/tmp/font.ttf", FontSize: 80,
		},
	}
	canvas := config.CanvasConfig{Width: 1080, Height: 1920, BgMode: "blur"}
	args := BuildSectionSegmentArgs(s, canvas, "/tmp/voice.ass", "/tmp/sec.mp4", 3.5)

	assertHas(t, args, "-loop", "1")
	assertHas(t, args, "-t", "3.5")
	assertHas(t, args, "-i", "/tmp/slide.jpg")
	assertHas(t, args, "-i", "/tmp/voice.mp3")
	assertHas(t, args, "-y", "/tmp/sec.mp4")

	fc := getFlag(t, args, "-filter_complex")
	if !strings.Contains(fc, "ass=filename='/tmp/voice.ass'") {
		t.Errorf("missing ass filter, got: %s", fc)
	}
	if !strings.Contains(fc, "drawtext=") {
		t.Errorf("missing drawtext, got: %s", fc)
	}
	if !strings.Contains(fc, "boxblur=10:5") {
		t.Errorf("missing boxblur for bg_fill=blur, got: %s", fc)
	}
}

func TestBuildSectionSegmentArgs_Contain(t *testing.T) {
	s := config.SectionConfig{
		Image: "/tmp/slide.jpg", Audio: "/tmp/voice.mp3", SubtitleFile: "/tmp/v.ass",
		BgFill: "contain",
	}
	canvas := config.CanvasConfig{Width: 1080, Height: 1920}
	args := BuildSectionSegmentArgs(s, canvas, "/tmp/v.ass", "/tmp/sec.mp4", 2)
	fc := getFlag(t, args, "-filter_complex")
	if strings.Contains(fc, "force_original_aspect_ratio=increase") {
		t.Errorf("contain must not crop, got: %s", fc)
	}
	if strings.Contains(fc, "boxblur") {
		t.Errorf("contain must not blur, got: %s", fc)
	}
}

func TestBuildSectionConcatArgs(t *testing.T) {
	args := BuildSectionConcatArgs("/tmp/concat.txt", "/tmp/final.mp4")
	assertHas(t, args, "-f", "concat")
	assertHas(t, args, "-safe", "0")
	assertHas(t, args, "-i", "/tmp/concat.txt")
	assertHas(t, args, "-c", "copy")
	assertHas(t, args, "-y", "/tmp/final.mp4")
}
