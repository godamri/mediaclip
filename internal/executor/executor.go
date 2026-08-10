package executor

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type Result struct {
	Status     string   `json:"status"`
	Output     string   `json:"output,omitempty"`
	Thumbnail  string   `json:"thumbnail,omitempty"`
	Thumbnails []string `json:"thumbnails,omitempty"`
	Error      string   `json:"error,omitempty"`
}

func Success(outputPath string) Result {
	return Result{Status: "success", Output: outputPath}
}

func Failure(errMsg string) Result {
	return Result{Status: "error", Error: errMsg}
}

func Execute(ctx context.Context, name string, args ...string) Result {
	cmd := exec.CommandContext(ctx, name, args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrStr := stderr.String()

		if ctx.Err() != nil {
			return Failure(fmt.Sprintf("execution timed out: %s", stderrStr))
		}

		msg := err.Error()
		if stderrStr != "" {
			msg = stderrStr
		}
		return Failure(msg)
	}

	return Result{Status: "success"}
}

func ProbeDuration(ctx context.Context, file string) (float64, error) {
	if d, err := probeDurationFFprobe(ctx, file); err == nil {
		return d, nil
	}
	return probeDurationFFmpeg(ctx, file)
}

func probeDurationFFprobe(ctx context.Context, file string) (float64, error) {
	cmd := exec.CommandContext(ctx, "ffprobe", "-v", "error",
		"-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", file)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("ffprobe duration: %s: %s", err, stderr.String())
	}
	s := strings.TrimSpace(out.String())
	d, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %s", s, err)
	}
	return d, nil
}

// probeDurationFFmpeg parses the duration from `ffmpeg -i` stderr, used when
// ffprobe is not installed. Returns an error if the duration cannot be found.
func probeDurationFFmpeg(ctx context.Context, file string) (float64, error) {
	cmd := exec.CommandContext(ctx, "ffmpeg", "-hide_banner", "-i", file)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	_ = cmd.Run() // ffmpeg -i exits non-zero; we only care about stderr

	re := regexp.MustCompile(`Duration:\s*(\d+):(\d+):(\d+(?:\.\d+)?)`)
	m := re.FindStringSubmatch(stderr.String())
	if m == nil {
		return 0, fmt.Errorf("ffmpeg duration: no Duration line in stderr")
	}
	hours, _ := strconv.ParseFloat(m[1], 64)
	mins, _ := strconv.ParseFloat(m[2], 64)
	secs, _ := strconv.ParseFloat(m[3], 64)
	return hours*3600 + mins*60 + secs, nil
}
