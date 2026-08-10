package executor

import (
	"context"
	"os"
	"testing"
)

var probeVideo = "MediAlign/yt.mp4"

func probeVideoOrSkip(t *testing.T) string {
	t.Helper()
	if _, err := os.Stat(probeVideo); err != nil {
		t.Skip("probe test video not available")
	}
	return probeVideo
}

func TestProbeDurationFFmpegFallback(t *testing.T) {
	video := probeVideoOrSkip(t)
	d, err := probeDurationFFmpeg(context.Background(), video)
	if err != nil {
		t.Fatal(err)
	}
	if d <= 0 {
		t.Fatalf("expected positive duration, got %f", d)
	}
}

func TestProbeDuration(t *testing.T) {
	video := probeVideoOrSkip(t)
	d, err := ProbeDuration(context.Background(), video)
	if err != nil {
		t.Fatal(err)
	}
	if d <= 0 {
		t.Fatalf("expected positive duration, got %f", d)
	}
}
