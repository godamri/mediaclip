package builder

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/godamri/mediaclip/internal/config"
)

var candidateFonts = []string{
	"/System/Library/Fonts/Supplemental/Verdana Bold.ttf",
	"/System/Library/Fonts/Supplemental/Arial Bold.ttf",
	"/Library/Fonts/Arial Bold.ttf",
	"/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
	"/usr/share/fonts/dejavu/DejaVuSans-Bold.ttf",
	"/usr/share/fonts/truetype/liberation/LiberationSans-Bold.ttf",
	`C:\Windows\Fonts\arialbd.ttf`,
	`C:\Windows\Fonts\verdanab.ttf`,
}

func testFont(t *testing.T) string {
	t.Helper()
	for _, f := range candidateFonts {
		if _, err := os.Stat(f); err == nil {
			return f
		}
	}
	t.Skipf("no known system font found on %s", runtime.GOOS)
	return ""
}

func TestBuildThumbnailArgs(t *testing.T) {
	args := BuildThumbnailArgs("/tmp/v.mp4", 12.5, 1080, 1920, "blur", "/tmp/t.ass", "/tmp/fonts", "/tmp/t.jpg")

	assertHas(t, args, "-ss", "12.5")
	assertHas(t, args, "-i", "/tmp/v.mp4")
	assertHas(t, args, "-frames:v", "1")
	assertHas(t, args, "-q:v", "2")
	assertHas(t, args, "-y", "/tmp/t.jpg")

	fc := getFlag(t, args, "-filter_complex")
	if !strings.Contains(fc, "ass=filename='/tmp/t.ass'") {
		t.Error("filter_complex missing ass filter")
	}
	if !strings.Contains(fc, "fontsdir='/tmp/fonts'") {
		t.Error("filter_complex missing fontsdir")
	}
	if !strings.Contains(fc, "boxblur=10:5") {
		t.Error("filter_complex missing boxblur")
	}
}

func TestBuildThumbnailArgs_NoBlur(t *testing.T) {
	args := BuildThumbnailArgs("/tmp/v.mp4", 1, 1080, 1920, "", "/tmp/t.ass", "", "/tmp/t.jpg")
	fc := getFlag(t, args, "-filter_complex")
	if strings.Contains(fc, "boxblur") {
		t.Error("boxblur should be omitted when bg_mode is not blur")
	}
}

func TestBuildThumbnailASS_HighlightsLongestWord(t *testing.T) {
	hook := config.HookConfig{
		Text:     "Pujian susah didapat",
		FontFile: testFont(t),
		FontSize: 80,
	}
	canvas := config.CanvasConfig{Width: 1080, Height: 1920}

	content, err := BuildThumbnailASS(hook, canvas, config.ThumbnailConfig{}, "top-left", "#FFFFFF", "#FFD166")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(content, "didapat") {
		t.Error("expected longest word 'didapat' in content")
	}
	if !strings.Contains(content, "{\\c&H66D1FF&}didapat{\\r}") {
		t.Errorf("longest word not wrapped with highlight color, got:\n%s", content)
	}
	if !strings.Contains(content, "{\\an7}") {
		t.Error("expected an7 for top-left")
	}
}

func TestBuildThumbnailASS_Positions(t *testing.T) {
	hook := config.HookConfig{
		Text:     "test",
		FontFile: testFont(t),
		FontSize: 40,
	}
	canvas := config.CanvasConfig{Width: 1080, Height: 1920}

	cases := []struct {
		pos string
		an  string
	}{
		{"top-left", "\\an7"},
		{"top-center", "\\an8"},
		{"top-right", "\\an9"},
		{"middle-left", "\\an4"},
		{"center", "\\an5"},
		{"middle-center", "\\an5"},
		{"middle-right", "\\an6"},
		{"bottom-left", "\\an1"},
		{"bottom-center", "\\an2"},
		{"bottom-right", "\\an3"},
	}
	for _, c := range cases {
		content, err := BuildThumbnailASS(hook, canvas, config.ThumbnailConfig{}, c.pos, "#FFFFFF", "#FFD166")
		if err != nil {
			t.Fatalf("pos %s: %v", c.pos, err)
		}
		if !strings.Contains(content, "{"+c.an+"}") {
			t.Errorf("pos %s: expected {%s} in:\n%s", c.pos, c.an, content)
		}
	}
}

func TestBuildThumbnailASS_MultiLine(t *testing.T) {
	hook := config.HookConfig{
		Lines:    []string{"Pujian susah didapat,", "tapi tetap bikin ngakak!"},
		FontFile: testFont(t),
		FontSize: 60,
	}
	canvas := config.CanvasConfig{Width: 1080, Height: 1920}

	content, err := BuildThumbnailASS(hook, canvas, config.ThumbnailConfig{}, "bottom-center", "#FFFFFF", "#FFD166")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(content, "\\N") {
		t.Error("expected \\N line separator for multi-line hook")
	}
	// longest word across all lines is "didapat," (8 chars) or "ngakak!" (7)
	if !strings.Contains(content, "{\\c&H66D1FF&}") {
		t.Error("expected highlight tag in multi-line content")
	}
	if !strings.Contains(content, "{\\an2}") {
		t.Error("expected an2 for bottom-center")
	}
}

func TestBuildThumbnailASS_CustomFontSize(t *testing.T) {
	hook := config.HookConfig{
		Text:     "test",
		FontFile: testFont(t),
		FontSize: 40,
	}
	canvas := config.CanvasConfig{Width: 1080, Height: 1920}

	content, err := BuildThumbnailASS(hook, canvas, config.ThumbnailConfig{FontSize: 120}, "center", "#FFFFFF", "#FFD166")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, ",120,") {
		t.Errorf("expected font size 120 in style, got:\n%s", content)
	}
}

func TestLongestWord(t *testing.T) {
	cases := []struct {
		lines []string
		want  string
	}{
		{[]string{"Pujian susah didapat"}, "didapat"},
		{[]string{"tapi tetap bikin ngakak!"}, "ngakak!"},
		{[]string{"a", "longer"}, "longer"},
	}
	for _, c := range cases {
		if got := longestWord(c.lines); got != c.want {
			t.Errorf("longestWord(%v) = %q, want %q", c.lines, got, c.want)
		}
	}
}

func TestAssColor(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"#FFD166", "&H66D1FF&"},
		{"#FFFFFF", "&HFFFFFF&"},
		{"#000000", "&H000000&"},
		{"garbage", "&HFFFFFF&"},
	}
	for _, c := range cases {
		if got := assColor(c.in); got != c.want {
			t.Errorf("assColor(%s) = %s, want %s", c.in, got, c.want)
		}
	}
}

func TestFontPostScriptName(t *testing.T) {
	name, err := FontPostScriptName(testFont(t))
	if err != nil {
		t.Fatal(err)
	}
	if name == "" {
		t.Error("expected non-empty PostScript name")
	}
}

func TestFontPostScriptName_CorruptFontNoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("PANIC on corrupt font: %v", r)
		}
	}()
	dir := t.TempDir()
	hdr := make([]byte, 12+16)
	hdr[0], hdr[1] = 0x00, 0x01
	hdr[4], hdr[5] = 0x00, 0x01
	copy(hdr[12:16], "name")
	copy(hdr[20:24], []byte{0xFF, 0xFF, 0xFF, 0xFF})
	copy(hdr[24:28], []byte{0xFF, 0xFF, 0xFF, 0xFF})
	f := filepath.Join(dir, "corrupt.ttf")
	if err := os.WriteFile(f, hdr, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := FontPostScriptName(f); err == nil {
		t.Error("expected error for corrupt font, got nil")
	}
}

func TestBuildImageThumbnailArgs(t *testing.T) {
	args := BuildImageThumbnailArgs("/tmp/slide.jpg", 1080, 1920, "blur", "/tmp/t.ass", "/tmp/fonts", "/tmp/t.jpg")
	if hasFlag(args, "-ss") {
		t.Error("image thumbnail must not use -ss")
	}
	assertHas(t, args, "-i", "/tmp/slide.jpg")
	assertHas(t, args, "-frames:v", "1")
	assertHas(t, args, "-y", "/tmp/t.jpg")
	fc := getFlag(t, args, "-filter_complex")
	if !strings.Contains(fc, "ass=filename='/tmp/t.ass'") {
		t.Error("missing ass filter")
	}
	if !strings.Contains(fc, "boxblur=10:5") {
		t.Error("missing boxblur")
	}
}

func TestBuildThumbnailArgs_StillHasSS(t *testing.T) {
	args := BuildThumbnailArgs("/tmp/v.mp4", 5, 1080, 1920, "blur", "/tmp/t.ass", "/tmp/fonts", "/tmp/t.jpg")
	assertHas(t, args, "-ss", "5")
}
