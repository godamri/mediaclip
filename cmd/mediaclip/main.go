package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/godamri/mediaclip/internal/builder"
	"github.com/godamri/mediaclip/internal/config"
	"github.com/godamri/mediaclip/internal/executor"
)

const defaultPort = ":8080"
const execTimeout = 10 * time.Minute

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "server" {
		addr := defaultPort
		if len(os.Args) >= 3 {
			addr = os.Args[2]
		}
		startServer(addr)
		return
	}

	if len(os.Args) >= 3 && os.Args[1] == "run" {
		runCLI(os.Args[2])
		return
	}

	if len(os.Args) == 2 && os.Args[1] == "run" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			printJSON(executor.Failure(fmt.Sprintf("failed to read stdin: %s", err)))
			os.Exit(1)
		}
		runCLI(string(data))
		return
	}

	if len(os.Args) == 1 {
		addr := os.Getenv("PORT")
		if addr == "" {
			addr = defaultPort
		}
		if addr[0] != ':' {
			addr = ":" + addr
		}
		startServer(addr)
		return
	}

	fmt.Fprintf(os.Stderr, "Usage:\n")
	fmt.Fprintf(os.Stderr, "  mediaclip                         HTTP server on :8080 (or $PORT)\n")
	fmt.Fprintf(os.Stderr, "  mediaclip server                  HTTP server on :8080\n")
	fmt.Fprintf(os.Stderr, "  mediaclip server :9090            HTTP server on :9090\n")
	fmt.Fprintf(os.Stderr, "  mediaclip run '<json>'            Process one job and exit\n")
	fmt.Fprintf(os.Stderr, "  mediaclip run < file.json         Process JSON from stdin\n")
	os.Exit(2)
}

func startServer(addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/render", handleRender)
	mux.HandleFunc("/health", handleHealth)

	server := &http.Server{
		Addr:    addr,
		Handler: withLogging(mux),
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("MediaClip server listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %s", err)
		}
	}()

	<-quit
	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("forced shutdown: %s", err)
	}
	log.Println("server stopped")
}

func handleRender(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, executor.Failure("method not allowed"))
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, executor.Failure(fmt.Sprintf("cannot read body: %s", err)))
		return
	}

	payload, err := config.ParsePayload(body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, executor.Failure(fmt.Sprintf("invalid payload: %s", err)))
		return
	}

	hasCallback := payload.CallbackURL != ""

	if config.IsMerge(payload) {
		if err := config.ValidateMergePayload(payload); err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, executor.Failure(fmt.Sprintf("validation failed: %s", err)))
			return
		}
		if hasCallback {
			writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted", "job_id": payload.JobID})
			go func() {
				result := executeMerge(payload)
				postCallback(payload.CallbackURL, result)
			}()
		} else {
			result := executeMerge(payload)
			status := http.StatusOK
			if result.Status == "error" {
				status = http.StatusInternalServerError
			}
			writeJSON(w, status, result)
		}
		return
	}

	if err := config.ValidatePayload(payload); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, executor.Failure(fmt.Sprintf("validation failed: %s", err)))
		return
	}

	if hasCallback {
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted", "job_id": payload.JobID})
		go func() {
			result := executeJob(payload)
			postCallback(payload.CallbackURL, result)
		}()
	} else {
		result := executeJob(payload)
		status := http.StatusOK
		if result.Status == "error" {
			status = http.StatusInternalServerError
		}
		writeJSON(w, status, result)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func runCLI(jsonStr string) {
	payload, err := config.ParsePayload([]byte(jsonStr))
	if err != nil {
		printJSON(executor.Failure(fmt.Sprintf("invalid payload: %s", err)))
		os.Exit(1)
	}

	var result executor.Result
	if config.IsMerge(payload) {
		if err := config.ValidateMergePayload(payload); err != nil {
			printJSON(executor.Failure(fmt.Sprintf("validation failed: %s", err)))
			os.Exit(1)
		}
		result = executeMerge(payload)
	} else {
		if err := config.ValidatePayload(payload); err != nil {
			printJSON(executor.Failure(fmt.Sprintf("validation failed: %s", err)))
			os.Exit(1)
		}
		result = executeJob(payload)
	}

	printJSON(result)
	if result.Status == "error" {
		os.Exit(1)
	}
}

func executeJob(payload config.JobPayload) executor.Result {
	args := builder.BuildFFmpegArgs(payload)

	ctx, cancel := context.WithTimeout(context.Background(), execTimeout)
	defer cancel()

	result := executor.Execute(ctx, "ffmpeg", args...)
	if result.Status == "success" {
		result.Output = payload.OutputPath
		if thumb, err := generateThumbnail(payload, payload.Assets.SourceVideo, payload.Config.Trim, payload.Config.Hook, -1); err == nil {
			result.Thumbnail = thumb
		}
	}
	return result
}

func executeMerge(payload config.JobPayload) executor.Result {
	args := builder.BuildMergeFFmpegArgs(payload)

	ctx, cancel := context.WithTimeout(context.Background(), execTimeout)
	defer cancel()

		result := executor.Execute(ctx, "ffmpeg", args...)
		if result.Status == "success" {
			result.Output = payload.OutputPath
			thumbs := []string{}
			for i, c := range payload.Clips {
				if thumb, err := generateThumbnail(payload, c.SourceVideo, c.Trim, c.Hook, i); err == nil {
					thumbs = append(thumbs, thumb)
				}
			}
			if len(thumbs) > 0 {
				result.Thumbnails = thumbs
			}
		}
		return result
	}

func generateThumbnail(payload config.JobPayload, source string, trim config.TrimConfig, hook config.HookConfig, idx int) (string, error) {
	if (hook.Text == "" && len(hook.Lines) == 0) || hook.FontFile == "" {
		return "", fmt.Errorf("thumbnail skipped: hook text or font_file missing")
	}

	thumb := payload.Config.Thumbnail

	pos := thumb.Position
	if pos == "" || pos == "auto" {
		pos = builder.RandomThumbPosition()
	}
	baseColor, hlColor := builder.PickThumbColors(thumb)

	assContent, err := builder.BuildThumbnailASS(hook, payload.Config.Canvas, thumb, pos, baseColor, hlColor)
	if err != nil {
		return "", err
	}

	tmp, err := os.CreateTemp("", "mediaclip_thumb_*.ass")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.WriteString(assContent); err != nil {
		tmp.Close()
		return "", err
	}
	tmp.Close()

	ts, err := randomFrameTS(payload, source, trim)
	if err != nil {
		return "", err
	}

	thumbPath := thumb.Path
	if thumbPath == "" {
		ext := filepath.Ext(payload.OutputPath)
		base := strings.TrimSuffix(payload.OutputPath, ext)
		if idx >= 0 {
			base += fmt.Sprintf("_%d", idx)
		}
		thumbPath = base + "_thumb.jpg"
	} else if idx >= 0 {
		ext := filepath.Ext(thumbPath)
		base := strings.TrimSuffix(thumbPath, ext)
		thumbPath = base + fmt.Sprintf("_%d", idx) + ext
	}

	fontsDir := filepath.Dir(hook.FontFile)
	args := builder.BuildThumbnailArgs(source, ts, payload.Config.Canvas.Width, payload.Config.Canvas.Height, payload.Config.Canvas.BgMode, tmpPath, fontsDir, thumbPath)

	ctx, cancel := context.WithTimeout(context.Background(), execTimeout)
	defer cancel()

	result := executor.Execute(ctx, "ffmpeg", args...)
	if result.Status != "success" {
		return "", fmt.Errorf("thumbnail render failed: %s", result.Error)
	}
	return thumbPath, nil
}

func randomFrameTS(payload config.JobPayload, source string, trim config.TrimConfig) (float64, error) {
	start := trim.StartSec
	end := trim.EndSec
	if end <= start {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		dur, err := executor.ProbeDuration(ctx, source)
		if err != nil {
			return 0, err
		}
		end = dur
	}
	if end <= start {
		return start, nil
	}
	return start + rand.Float64()*(end-start), nil
}

func postCallback(url string, result executor.Result) {
	data, err := json.Marshal(result)
	if err != nil {
		log.Printf("callback marshal error: %s", err)
		return
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		log.Printf("callback POST error: %s", err)
		return
	}
	resp.Body.Close()
	log.Printf("callback sent to %s: %d", url, resp.StatusCode)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func printJSON(v interface{}) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
