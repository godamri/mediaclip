package builder

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"os"
	"strings"

	"github.com/godamri/mediaclip/internal/config"
)

var thumbPositions = []string{
	"top-left", "top-center", "top-right",
	"middle-left", "middle-center", "middle-right",
	"bottom-left", "bottom-center", "bottom-right",
}

var thumbPositionMargin = map[string]string{
	"top-left":      "\\an7",
	"top-center":    "\\an8",
	"top-right":     "\\an9",
	"middle-left":   "\\an4",
	"center":        "\\an5",
	"middle-center": "\\an5",
	"middle-right":  "\\an6",
	"bottom-left":   "\\an1",
	"bottom-center": "\\an2",
	"bottom-right":  "\\an3",
}

var enterprisePalette = [][2]string{
	{"#FFFFFF", "#2563EB"}, // white + blue-600
	{"#F8FAFC", "#475569"}, // slate-50 + slate-600
	{"#FFFFFF", "#1E40AF"}, // white + blue-800
	{"#F1F5F9", "#334155"}, // slate-100 + slate-700
	{"#FFFFFF", "#0EA5E9"}, // white + sky-500
}

func RandomThumbPosition() string {
	return thumbPositions[rand.Intn(len(thumbPositions))]
}

func PickThumbColors(thumb config.ThumbnailConfig) (string, string) {
	if thumb.TextColor != "" || thumb.HighlightColor != "" {
		base := thumb.TextColor
		if base == "" {
			base = "#FFFFFF"
		}
		hl := thumb.HighlightColor
		if hl == "" {
			hl = enterprisePalette[0][1]
		}
		return base, hl
	}
	p := enterprisePalette[rand.Intn(len(enterprisePalette))]
	return p[0], p[1]
}

func BuildThumbnailArgs(source string, ts float64, w, h int, bgMode, assFile, fontsDir, out string) []string {
	args := []string{"-ss", formatFloat(ts)}
	args = append(args, buildThumbnailImgArgs(source, w, h, bgMode, assFile, fontsDir, out)...)
	return args
}

// BuildImageThumbnailArgs renders a thumbnail from a still image source
// (no seeking, no video probing needed).
func BuildImageThumbnailArgs(source string, w, h int, bgMode, assFile, fontsDir, out string) []string {
	return buildThumbnailImgArgs(source, w, h, bgMode, assFile, fontsDir, out)
}

func buildThumbnailImgArgs(source string, w, h int, bgMode, assFile, fontsDir, out string) []string {
	bgScale := fmt.Sprintf("[0:v]scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d", w, h, w, h)
	blur := ""
	if bgMode == "blur" {
		blur = ",boxblur=10:5"
	}
	bg := fmt.Sprintf("%s%s[bg]", bgScale, blur)
	fg := fmt.Sprintf("[0:v]scale=%d:%d:force_original_aspect_ratio=decrease[fg]", w, h)
	overlay := "[bg][fg]overlay=(W-w)/2:(H-h)/2[ov]"
	ass := fmt.Sprintf("[ov]ass=filename='%s'", escapeFilterPath(assFile))
	if fontsDir != "" {
		ass += ":fontsdir='" + escapeFilterPath(fontsDir) + "'"
	}
	ass += "[out]"

	return []string{
		"-i", source,
		"-filter_complex", strings.Join([]string{bg, fg, overlay, ass}, ";"),
		"-map", "[out]",
		"-frames:v", "1",
		"-q:v", "2",
		"-y", out,
	}
}

func BuildThumbnailASS(h config.HookConfig, canvas config.CanvasConfig, thumb config.ThumbnailConfig, pos string, baseColor, highlightColor string) (string, error) {
	psName, err := FontPostScriptName(h.FontFile)
	if err != nil {
		return "", fmt.Errorf("thumbnail font: %w", err)
	}

	fontSize := thumb.FontSize
	if fontSize <= 0 {
		fontSize = h.FontSize
	}
	if fontSize <= 0 {
		fontSize = float64(canvas.Height) * 0.07
	}
	if fontSize < float64(canvas.Height)*0.055 {
		fontSize = float64(canvas.Height) * 0.055
	}

	pad := thumb.Padding
	if pad <= 0 {
		pad = 40
	}

	lines := h.Lines
	if len(lines) == 0 && h.Text != "" {
		lines = []string{h.Text}
	}
	if len(lines) == 0 {
		return "", fmt.Errorf("thumbnail requires hook text")
	}

	text := strings.Join(lines, "\\N")
	word := longestWord(lines)
	if word != "" {
		text = highlightWord(text, word, assColor(highlightColor))
	}

	an := thumbPositionMargin[pos]
	margins := marginForPosition(pos, pad)

	header := fmt.Sprintf(`[Script Info]
ScriptType: v4.00+
PlayResX: %d
PlayResY: %d
ScaledBorderAndShadow: yes

[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
Style: Thumb,%s,%d,%s,%s,&H00000000,&H80000000,-1,0,0,0,100,100,0,0,1,8,3,5,0,0,0,1

[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: 0,0:00:00.00,0:00:05.00,Thumb,,%d,%d,%d,,{%s}%s
`, canvas.Width, canvas.Height,
		psName, int(fontSize),
		assColor(baseColor), assColor(baseColor),
		margins[0], margins[1], margins[2],
		an, text)

	return header, nil
}

func marginForPosition(pos string, pad int) [3]int {
	switch pos {
	case "top-left":
		return [3]int{pad, 0, pad}
	case "top-center":
		return [3]int{0, 0, pad}
	case "top-right":
		return [3]int{0, pad, pad}
	case "middle-left":
		return [3]int{pad, 0, 0}
	case "center", "middle-center":
		return [3]int{0, 0, 0}
	case "middle-right":
		return [3]int{0, pad, 0}
	case "bottom-left":
		return [3]int{pad, 0, pad}
	case "bottom-center":
		return [3]int{0, 0, pad}
	case "bottom-right":
		return [3]int{0, pad, pad}
	default:
		return [3]int{0, 0, 0}
	}
}

func longestWord(lines []string) string {
	longest := ""
	for _, l := range lines {
		for _, w := range strings.Fields(l) {
			if len(w) > len(longest) {
				longest = w
			}
		}
	}
	return longest
}

func highlightWord(text, word, color string) string {
	idx := strings.Index(text, word)
	if idx < 0 {
		return text
	}
	return text[:idx] + "{\\c" + color + "}" + word + "{\\r}" + text[idx+len(word):]
}

func assColor(hexColor string) string {
	h := strings.TrimPrefix(hexColor, "#")
	if len(h) != 6 {
		return "&HFFFFFF&"
	}
	var r, g, b byte
	fmt.Sscanf(h[0:2], "%02x", &r)
	fmt.Sscanf(h[2:4], "%02x", &g)
	fmt.Sscanf(h[4:6], "%02x", &b)
	return fmt.Sprintf("&H%02X%02X%02X&", b, g, r)
}

func FontPostScriptName(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(data) < 4 {
		return "", fmt.Errorf("font file too small")
	}

	base := 0
	if string(data[0:4]) == "ttcf" {
		if len(data) < 16 {
			return "", fmt.Errorf("bad ttc header")
		}
		numFonts := binary.BigEndian.Uint32(data[8:12])
		if numFonts == 0 {
			return "", fmt.Errorf("empty ttc")
		}
		base = int(binary.BigEndian.Uint32(data[12:16]))
		if base >= len(data) {
			return "", fmt.Errorf("bad ttc offset")
		}
	}

	if base+12 > len(data) {
		return "", fmt.Errorf("bad font header")
	}
	numTables := binary.BigEndian.Uint16(data[base+4 : base+6])

	var nameOff, nameLen uint32
	for i := 0; i < int(numTables); i++ {
		rec := base + 12 + i*16
		if rec+16 > len(data) {
			break
		}
		if string(data[rec:rec+4]) == "name" {
			nameOff = binary.BigEndian.Uint32(data[rec+8 : rec+12])
			nameLen = binary.BigEndian.Uint32(data[rec+12 : rec+16])
		}
	}
	if nameLen == 0 {
		return "", fmt.Errorf("no name table")
	}
	if uint64(nameOff)+uint64(nameLen) > uint64(len(data)) {
		return "", fmt.Errorf("name table out of bounds")
	}

	nt := data[nameOff : nameOff+nameLen]
	if len(nt) < 6 {
		return "", fmt.Errorf("bad name table")
	}
	count := binary.BigEndian.Uint16(nt[2:4])
	strOff := binary.BigEndian.Uint16(nt[4:6])

	ps := ""
	family := ""
	for i := 0; i < int(count); i++ {
		rec := 6 + i*12
		if rec+12 > len(nt) {
			break
		}
		platform := binary.BigEndian.Uint16(nt[rec : rec+2])
		nameID := binary.BigEndian.Uint16(nt[rec+6 : rec+8])
		l := binary.BigEndian.Uint16(nt[rec+8 : rec+10])
		o := binary.BigEndian.Uint16(nt[rec+10 : rec+12])

		start := int(strOff) + int(o)
		if start+int(l) > len(nt) {
			continue
		}
		var s string
		if platform == 3 {
			s = strings.TrimSpace(decodeUTF16(nt[start : start+int(l)]))
		} else if platform == 0 || platform == 1 {
			s = strings.TrimSpace(string(nt[start : start+int(l)]))
		}
		if s == "" {
			continue
		}
		switch nameID {
		case 6:
			if ps == "" {
				ps = s
			}
		case 1, 16:
			if family == "" {
				family = s
			}
		}
	}

	if ps != "" {
		return ps, nil
	}
	if family != "" {
		return family, nil
	}
	return "", fmt.Errorf("no font name found")
}

func decodeUTF16(b []byte) string {
	runes := make([]rune, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		u := binary.BigEndian.Uint16(b[i : i+2])
		runes = append(runes, rune(u))
	}
	return string(runes)
}
