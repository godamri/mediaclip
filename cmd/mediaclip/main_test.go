package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/godamri/mediaclip/internal/config"
	"github.com/godamri/mediaclip/internal/executor"
)

func TestHealthEndpoint(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "ok" {
		t.Errorf("expected status=ok, got %s", resp["status"])
	}
}

func TestRender_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/render", nil)
	w := httptest.NewRecorder()

	handleRender(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "error" {
		t.Errorf("expected error status, got %s", resp["status"])
	}
}

func TestRender_InvalidJSON(t *testing.T) {
	body := strings.NewReader(`{bad json}`)
	req := httptest.NewRequest(http.MethodPost, "/render", body)
	w := httptest.NewRecorder()

	handleRender(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestRender_MissingFields(t *testing.T) {
	body := strings.NewReader(`{"job_id":"test"}`)
	req := httptest.NewRequest(http.MethodPost, "/render", body)
	w := httptest.NewRecorder()

	handleRender(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}

func TestRender_InvalidJSONViaProcess(t *testing.T) {
	payload, err := config.ParsePayload([]byte(`{bad}`))
	if err == nil {
		t.Fatal("expected parse error")
	}
	var result executor.Result
	if config.IsMerge(payload) {
		result = executor.Failure("merge invalid")
	} else if err := config.ValidatePayload(payload); err != nil {
		result = executor.Failure(err.Error())
	}
	if result.Status != "" && result.Status != "error" {
		t.Fatal("expected error")
	}
}

func TestRender_MissingFieldsViaValidation(t *testing.T) {
	payload, _ := config.ParsePayload([]byte(`{"job_id":"test"}`))
	err := config.ValidatePayload(payload)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestRender_AssetNotFound(t *testing.T) {
	data := `{
		"job_id": "test",
		"assets": {
			"source_video": "/nonexistent/video.mp4",
			"subtitle_file": "/nonexistent/sub.ass"
		},
		"config": {
			"trim": {"start_sec": 0, "end_sec": 10},
			"canvas": {"width": 1080, "height": 1920, "bg_mode": "blur"},
			"hook": {
				"text": "test",
				"font_file": "",
				"font_size": 72,
				"font_color": "white",
				"y_position": 200
			}
		},
		"output_path": "/tmp/out.mp4"
	}`
	payload, _ := config.ParsePayload([]byte(data))
	err := config.ValidatePayload(payload)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "source_video not found") {
		t.Errorf("expected source_video error, got: %s", err)
	}
}

func TestRender_CallbackReturns202(t *testing.T) {
	dir := t.TempDir()
	video := dir + "/video.mp4"
	sub := dir + "/sub.ass"
	if err := os.WriteFile(video, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sub, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	body := strings.NewReader(`{"job_id":"test","callback_url":"http://localhost:9999/cb","assets":{"source_video":"` + video + `","subtitle_file":"` + sub + `"},"config":{"trim":{"start_sec":0,"end_sec":10},"canvas":{"width":1080,"height":1920,"bg_mode":"blur"}},"output_path":"` + dir + `/out.mp4"}`)
	req := httptest.NewRequest(http.MethodPost, "/render", body)
	w := httptest.NewRecorder()

	handleRender(w, req)
	if w.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d", w.Code)
	}
}
