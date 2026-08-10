package builder

import (
	"fmt"
	"strings"

	"github.com/godamri/mediaclip/internal/config"
)

func BuildFFmpegArgs(p config.JobPayload) []string {
	args := []string{}

	if p.Config.Trim.StartSec > 0 || p.Config.Trim.EndSec > 0 {
		if p.Config.Trim.StartSec > 0 {
			args = append(args, "-ss", formatFloat(p.Config.Trim.StartSec))
		}
		if p.Config.Trim.EndSec > 0 {
			args = append(args, "-to", formatFloat(p.Config.Trim.EndSec))
		}
	}

	args = append(args, "-i", p.Assets.SourceVideo)

	filter := buildFilterComplex(p)
	args = append(args, "-filter_complex", filter)
	args = append(args, "-map", "[out]")
	args = append(args, "-map", "0:a?")

	args = append(args, "-c:v", "libx264", "-preset", "fast", "-crf", "22")
	args = append(args, "-c:a", "aac", "-b:a", "128k")

	args = append(args, "-y", p.OutputPath)

	return args
}

func buildFilterComplex(p config.JobPayload) string {
	w := p.Config.Canvas.Width
	h := p.Config.Canvas.Height

	bgScale := fmt.Sprintf("[0:v]scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d", w, h, w, h)

	var blur string
	switch p.Config.Canvas.BgMode {
	case "blur":
		blur = ",boxblur=10:5"
	default:
		blur = ""
	}

	bg := fmt.Sprintf("%s%s[bg]", bgScale, blur)
	fg := fmt.Sprintf("[0:v]scale=%d:%d:force_original_aspect_ratio=decrease[fg]", w, h)
	overlay := "[bg][fg]overlay=(W-w)/2:(H-h)/2[overlaid]"

	parts := []string{bg, fg, overlay}

	subFile := p.Assets.SubtitleFile
	ass := fmt.Sprintf("[overlaid]ass=filename='%s'[subbed]", subFile)
	parts = append(parts, ass)

	if (p.Config.Hook.Text != "" || len(p.Config.Hook.Lines) > 0) && p.Config.Hook.FontFile != "" {
		hookParts := buildDrawtext(p.Config.Hook, p.Config.Canvas.Height, "subbed", "out")
		parts = append(parts, hookParts...)
	} else {
		parts = append(parts, "[subbed]null[out]")
	}

	return strings.Join(parts, ";")
}

func buildDrawtext(h config.HookConfig, canvasHeight int, inputLabel, outputLabel string) []string {
	text := h.Text
	lines := h.Lines

	if h.AccentText != "" && h.AccentColor != "" {
		cleaned := []string{}
		for _, l := range lines {
			if l != h.AccentText {
				cleaned = append(cleaned, l)
			}
		}
		lines = cleaned
	}

	if len(lines) > 0 {
		text = strings.Join(lines, "\n")
	}

	fontColor := h.FontColor
	if fontColor == "" {
		fontColor = "white"
	}
	fontSize := formatFloat(h.FontSize)

	mainY := calcY(h, len(lines), canvasHeight)

	params := buildDrawtextParams(text, fontColor, h.FontFile, fontSize, mainY, h)

	if h.AccentText != "" && h.AccentColor != "" {
		accentY := fmt.Sprintf("(%s)+%d", mainY, int(h.FontSize*1.3))
		accentParams := buildDrawtextParams(h.AccentText, h.AccentColor, h.FontFile, fontSize, accentY, h)
		return []string{
			fmt.Sprintf("[%s]drawtext=%s[out_dt]", inputLabel, strings.Join(params, ":")),
			fmt.Sprintf("[out_dt]drawtext=%s[%s]", strings.Join(accentParams, ":"), outputLabel),
		}
	}

	return []string{
		fmt.Sprintf("[%s]drawtext=%s[%s]", inputLabel, strings.Join(params, ":"), outputLabel),
	}
}

func calcY(h config.HookConfig, numLines, canvasHeight int) string {
	lineH := int(h.FontSize * 1.2)
	totalH := numLines * lineH

	switch h.Position {
	case "center":
		y := (canvasHeight - totalH) / 2
		return formatFloat(float64(y))
	case "bottom":
		y := canvasHeight - totalH - 350
		return formatFloat(float64(y))
	case "top":
		if h.YPosition > 0 {
			return formatFloat(h.YPosition)
		}
		topMargin := float64(canvasHeight) * 0.12
		y := topMargin + h.FontSize
		return formatFloat(y)
	default:
		if h.YPosition > 0 {
			return formatFloat(h.YPosition)
		}
		topMargin := float64(canvasHeight) * 0.12
		y := topMargin + h.FontSize
		return formatFloat(y)
	}
}

func buildDrawtextParams(text, color, fontFile, fontSize, yPos string, h config.HookConfig) []string {
	p := []string{}
	p = append(p, "text='"+escapeDrawText(text)+"'")
	p = append(p, "fontfile='"+fontFile+"'")
	p = append(p, "fontsize="+fontSize)
	p = append(p, "fontcolor="+color)
	p = append(p, "x=(w-text_w)/2")
	p = append(p, "y="+yPos)

	if h.BoxBg != "" {
		p = append(p, "box=1")
		p = append(p, "boxcolor="+h.BoxBg)
	}
	if h.BoxPadding > 0 {
		p = append(p, fmt.Sprintf("boxborderw=%d", h.BoxPadding))
	}
	if h.StrokeWidth > 0 {
		p = append(p, fmt.Sprintf("borderw=%d", h.StrokeWidth))
		p = append(p, "bordercolor=black")
	}
	if h.Shadow {
		p = append(p, "shadowx=4")
		p = append(p, "shadowy=4")
		p = append(p, "shadowcolor=black@0.45")
	}
	p = append(p, "text_align=center")

	duration := h.Duration
	if duration <= 0 {
		duration = 1.5
	}
	p = append(p, fmt.Sprintf("enable='between(t,0,%s)'", formatFloat(duration)))
	return p
}

func BuildMergeFFmpegArgs(p config.JobPayload) []string {
	args := []string{}
	w := p.Config.Canvas.Width
	h := p.Config.Canvas.Height
	blur := ""
	if p.Config.Canvas.BgMode == "blur" {
		blur = ",boxblur=10:5"
	}

	for _, c := range p.Clips {
		if c.Trim.StartSec > 0 {
			args = append(args, "-ss", formatFloat(c.Trim.StartSec))
		}
		if c.Trim.EndSec > 0 {
			args = append(args, "-to", formatFloat(c.Trim.EndSec))
		}
		args = append(args, "-i", c.SourceVideo)
	}

	filters := []string{}
	segments := len(p.Clips) * 2

	for i, c := range p.Clips {
		hookDur := c.Hook.Duration
		if hookDur <= 0 {
			hookDur = 1.0
		}

		c.Hook.Duration = hookDur
		if c.Hook.FontColor == "" {
			c.Hook.FontColor = "white"
		}
		if c.Hook.Position == "" {
			c.Hook.Position = "center"
		}

		imgLabel := fmt.Sprintf("hook_img_%d", i)
		hookLabel := fmt.Sprintf("hook_v_%d", i)

		if c.Hook.Image != "" {
			filters = append(filters,
				fmt.Sprintf("movie='%s':loop=1,setpts=PTS-STARTPTS,scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d%s[%s]",
					c.Hook.Image, w, h, w, h, blur, imgLabel))
		} else {
			sharp := ""
			if blur == "" {
				sharp = ",boxblur=10:5"
			}
			filters = append(filters,
				fmt.Sprintf("[%d:v]select=eq(n\\,0),setpts=PTS-STARTPTS,scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d%s[%s]",
					i, w, h, w, h, sharp, imgLabel))
		}

		if (c.Hook.Text != "" || len(c.Hook.Lines) > 0) && c.Hook.FontFile != "" {
			for _, part := range buildDrawtext(c.Hook, h, imgLabel, hookLabel) {
				filters = append(filters, part)
			}
		} else {
			filters = append(filters, fmt.Sprintf("[%s]null[%s]", imgLabel, hookLabel))
		}

		fgLabel := fmt.Sprintf("fg_%d", i)
		bgLabel := fmt.Sprintf("bg_%d", i)
		contentLabel := fmt.Sprintf("content_v_%d", i)

		filters = append(filters,
			fmt.Sprintf("[%d:v]setpts=PTS-STARTPTS,scale=%d:%d:force_original_aspect_ratio=decrease[%s]",
				i, w, h, fgLabel))
		filters = append(filters,
			fmt.Sprintf("[%d:v]setpts=PTS-STARTPTS,scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d%s[%s]",
				i, w, h, w, h, blur, bgLabel))
		filters = append(filters,
			fmt.Sprintf("[%s][%s]overlay=(W-w)/2:(H-h)/2[%s]", bgLabel, fgLabel, contentLabel))

		// Audio
		hookALabel := fmt.Sprintf("hook_a_%d", i)
		contentALabel := fmt.Sprintf("content_a_%d", i)

		filters = append(filters,
			fmt.Sprintf("aevalsrc=0:d=%s[%s]", formatFloat(hookDur), hookALabel))
		filters = append(filters,
			fmt.Sprintf("[%d:a]asetpts=PTS-STARTPTS[%s]", i, contentALabel))
	}

	// Concat — expects interleaved order: seg0_v, seg0_a, seg1_v, seg1_a, ...
	concatInputs := []string{}
	for i := 0; i < len(p.Clips); i++ {
		concatInputs = append(concatInputs,
			fmt.Sprintf("[hook_v_%d]", i), fmt.Sprintf("[hook_a_%d]", i),
			fmt.Sprintf("[content_v_%d]", i), fmt.Sprintf("[content_a_%d]", i))
	}

	filters = append(filters,
		fmt.Sprintf("%sconcat=n=%d:v=1:a=1[out]", strings.Join(concatInputs, ""), segments))

	args = append(args, "-filter_complex", strings.Join(filters, ";"))
	args = append(args, "-map", "[out]")
	args = append(args, "-c:v", "libx264", "-preset", "fast", "-crf", "22")
	args = append(args, "-c:a", "aac", "-b:a", "128k")
	args = append(args, "-y", p.OutputPath)

	return args
}

func escapeDrawText(s string) string {
	return strings.ReplaceAll(s, "'", "'\\\\''")
}

func formatFloat(f float64) string {
	s := fmt.Sprintf("%.1f", f)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}
