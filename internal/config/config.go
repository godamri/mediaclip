package config

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/godamri/mediaclip/pkg/fs"
)

type Assets struct {
	SourceVideo   string `json:"source_video"`
	SubtitleFile  string `json:"subtitle_file"`
}

type TrimConfig struct {
	StartSec float64 `json:"start_sec"`
	EndSec   float64 `json:"end_sec"`
}

type CanvasConfig struct {
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	BgMode  string `json:"bg_mode"`
}

type HookConfig struct {
	Text        string   `json:"text"`
	FontFile    string   `json:"font_file"`
	FontSize    float64  `json:"font_size"`
	FontColor   string   `json:"font_color"`
	YPosition   float64  `json:"y_position"`
	Position    string   `json:"position,omitempty"`
	Lines       []string `json:"lines,omitempty"`
	BoxBg       string   `json:"box_bg,omitempty"`
	BoxPadding  int      `json:"box_padding,omitempty"`
	StrokeWidth int      `json:"stroke_width,omitempty"`
	Shadow      bool     `json:"shadow,omitempty"`
	AccentColor string   `json:"accent_color,omitempty"`
	AccentText  string   `json:"accent_text,omitempty"`
	Duration    float64  `json:"duration,omitempty"`
	Image       string   `json:"image,omitempty"`
}

type Config struct {
	Trim   TrimConfig   `json:"trim"`
	Canvas CanvasConfig `json:"canvas"`
	Hook   HookConfig   `json:"hook"`
}

type ClipConfig struct {
	SourceVideo string     `json:"source_video"`
	Trim        TrimConfig `json:"trim,omitempty"`
	Hook        HookConfig `json:"hook"`
}

type JobPayload struct {
	JobID       string        `json:"job_id"`
	CallbackURL string        `json:"callback_url,omitempty"`
	Assets      Assets        `json:"assets,omitempty"`
	Config      Config        `json:"config,omitempty"`
	Clips       []ClipConfig  `json:"clips,omitempty"`
	OutputPath  string        `json:"output_path"`
}

func ParsePayload(data []byte) (JobPayload, error) {
	var p JobPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return p, fmt.Errorf("invalid JSON: %w", err)
	}
	return p, nil
}

func ValidatePayload(p JobPayload) error {
	if p.JobID == "" {
		return errors.New("job_id is required")
	}
	if p.Assets.SourceVideo == "" {
		return errors.New("source_video is required")
	}
	if p.Assets.SubtitleFile == "" {
		return errors.New("subtitle_file is required")
	}
	if p.OutputPath == "" {
		return errors.New("output_path is required")
	}
	if !fs.FileExists(p.Assets.SourceVideo) {
		return fmt.Errorf("source_video not found: %s", p.Assets.SourceVideo)
	}
	if !fs.FileExists(p.Assets.SubtitleFile) {
		return fmt.Errorf("subtitle_file not found: %s", p.Assets.SubtitleFile)
	}
	if p.Config.Hook.FontFile != "" && !fs.FileExists(p.Config.Hook.FontFile) {
		return fmt.Errorf("font_file not found: %s", p.Config.Hook.FontFile)
	}
	if p.Config.Trim.StartSec < 0 {
		return errors.New("trim.start_sec must be >= 0")
	}
	if p.Config.Trim.EndSec <= p.Config.Trim.StartSec {
		return errors.New("trim.end_sec must be greater than trim.start_sec")
	}
	if p.Config.Canvas.Width <= 0 || p.Config.Canvas.Height <= 0 {
		return errors.New("canvas dimensions must be positive")
	}
	switch p.Config.Canvas.BgMode {
	case "blur", "":
	default:
		return fmt.Errorf("unsupported canvas.bg_mode: %s", p.Config.Canvas.BgMode)
	}
	if (p.Config.Hook.Text != "" || len(p.Config.Hook.Lines) > 0) && p.Config.Hook.FontSize <= 0 {
		return errors.New("hook.font_size must be positive")
	}
	return nil
}

func IsMerge(p JobPayload) bool {
	return len(p.Clips) > 0
}

func ValidateMergePayload(p JobPayload) error {
	if p.JobID == "" {
		return errors.New("job_id is required")
	}
	if p.OutputPath == "" {
		return errors.New("output_path is required")
	}
	if p.Config.Canvas.Width <= 0 || p.Config.Canvas.Height <= 0 {
		return errors.New("canvas dimensions must be positive")
	}
	switch p.Config.Canvas.BgMode {
	case "blur", "":
	default:
		return fmt.Errorf("unsupported canvas.bg_mode: %s", p.Config.Canvas.BgMode)
	}
	if len(p.Clips) == 0 {
		return errors.New("at least one clip is required")
	}
	for i, c := range p.Clips {
		if c.SourceVideo == "" {
			return fmt.Errorf("clips[%d].source_video is required", i)
		}
		if !fs.FileExists(c.SourceVideo) {
			return fmt.Errorf("clips[%d].source_video not found: %s", i, c.SourceVideo)
		}
		if c.Trim.StartSec < 0 {
			return fmt.Errorf("clips[%d].trim.start_sec must be >= 0", i)
		}
		if c.Trim.EndSec > 0 && c.Trim.EndSec <= c.Trim.StartSec {
			return fmt.Errorf("clips[%d].trim.end_sec must be greater than start_sec", i)
		}
		if c.Hook.FontFile != "" && !fs.FileExists(c.Hook.FontFile) {
			return fmt.Errorf("clips[%d].hook.font_file not found: %s", i, c.Hook.FontFile)
		}
		if c.Hook.Image != "" && !fs.FileExists(c.Hook.Image) {
			return fmt.Errorf("clips[%d].hook.image not found: %s", i, c.Hook.Image)
		}
	}
	return nil
}
