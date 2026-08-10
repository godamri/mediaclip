package executor

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestExecute_Success(t *testing.T) {
	ctx := context.Background()
	result := Execute(ctx, "echo", "hello")

	if result.Status != "success" {
		t.Errorf("expected success, got %s: %s", result.Status, result.Error)
	}
}

func TestExecute_CommandNotFound(t *testing.T) {
	ctx := context.Background()
	result := Execute(ctx, "nonexistent_command_xyz")

	if result.Status != "error" {
		t.Fatal("expected error for nonexistent command")
	}
	if result.Error == "" {
		t.Fatal("expected non-empty error message")
	}
}

func TestExecute_NonZeroExit(t *testing.T) {
	ctx := context.Background()
	result := Execute(ctx, "false")

	if result.Status != "error" {
		t.Fatal("expected error for false command")
	}
}

func TestExecute_Timeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	result := Execute(ctx, "sleep", "5")

	if result.Status != "error" {
		t.Fatal("expected error for timed out command")
	}
	if !strings.Contains(result.Error, "timed out") {
		t.Errorf("expected timeout message, got: %s", result.Error)
	}
}

func TestExecute_StderrCapture(t *testing.T) {
	ctx := context.Background()
	result := Execute(ctx, "sh", "-c", "echo stderr output >&2; exit 1")

	if result.Status != "error" {
		t.Fatal("expected error")
	}
	if !strings.Contains(result.Error, "stderr output") {
		t.Errorf("expected stderr output in error, got: %s", result.Error)
	}
}

func TestSuccessResult(t *testing.T) {
	r := Success("/tmp/out.mp4")
	if r.Status != "success" {
		t.Errorf("expected success status, got %s", r.Status)
	}
	if r.Output != "/tmp/out.mp4" {
		t.Errorf("expected output path, got %s", r.Output)
	}
}

func TestFailureResult(t *testing.T) {
	r := Failure("something went wrong")
	if r.Status != "error" {
		t.Errorf("expected error status, got %s", r.Status)
	}
	if r.Error != "something went wrong" {
		t.Errorf("expected error message, got %s", r.Error)
	}
}

func TestExecute_CanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := Execute(ctx, "sleep", "5")

	if result.Status != "error" {
		t.Fatal("expected error for canceled context")
	}
}
