package executor

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

type Result struct {
	Status string `json:"status"`
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
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
