# MediaClip

Stateless Go worker that renders short-form vertical videos using FFmpeg. Takes a JSON payload, builds an FFmpeg `-filter_complex` command, executes it, and returns success/fail.

Zero external Go dependencies. Single binary.

## Quick Start

```bash
go build ./cmd/mediaclip/

# CLI — process one job
./mediaclip run "$(cat samples/editorial.json)"

# Server — HTTP on :8080
./mediaclip

curl -X POST http://localhost:8080/render \
  -H "Content-Type: application/json" \
  -d @samples/editorial.json
```

## Dependencies

- **Go** 1.21+ (stdlib only — zero external packages)
- **FFmpeg** with `--enable-libfreetype` (drawtext) and `--enable-libass` (ASS subtitles)

```bash
brew install homebrew-ffmpeg/ffmpeg/ffmpeg
ffmpeg -filters 2>/dev/null | grep -E "drawtext|ass"
```

## Architecture

```
JSON payload
  ↓
internal/config     parse + validate
  ↓
internal/builder    construct FFmpeg args (-filter_complex)
  ↓
internal/executor   os/exec with context timeout
  ↓
FFmpeg → output.mp4
```

### Packages

| Package | Responsibility |
|---|---|
| `cmd/mediaclip` | Entrypoint — HTTP server (`:8080`) or CLI worker |
| `internal/config` | JSON parsing, structs, file-existence validation |
| `internal/builder` | FFmpeg `-filter_complex` string construction |
| `internal/executor` | `os/exec` wrapper with context timeout + stderr capture |
| `pkg/fs` | File-existence checks |

### Dual-mode entrypoint

```
./mediaclip                   HTTP server on :8080 (or $PORT)
./mediaclip server            HTTP server on :8080
./mediaclip server :9090      HTTP server on :9090
PORT=9090 ./mediaclip         HTTP server on :9090
./mediaclip run '<json>'       One-shot CLI, exits 0/1
./mediaclip run < file.json    Read JSON from stdin
```

## Payload API

### Single clip

```json
{
  "job_id": "clip_007",
  "callback_url": "https://api.example.com/webhook",
  "assets": {
    "source_video": "/path/to/video.mp4",
    "subtitle_file": "/path/to/sub.ass"
  },
  "config": {
    "trim": { "start_sec": 345.5, "end_sec": 385.2 },
    "canvas": { "width": 1080, "height": 1920, "bg_mode": "blur" },
    "hook": {
      "lines": ["KENAPA DIA", "MAU DIPUJI?"],
      "font_file": "/path/to/font.ttf",
      "font_size": 80,
      "font_color": "white",
      "duration": 1.5
    },
    "thumbnail": {
      "position": "auto",
      "text_color": "#FFFFFF",
      "highlight_color": "#FFD166"
    }
  },
  "output_path": "/path/to/output.mp4"
}
```

> Every successful render also produces a thumbnail at `<output>_thumb.jpg` (single) or `<output>_N_thumb.jpg` per clip (merge). Response includes `thumbnail` / `thumbnails` fields. See [Thumbnails](#thumbnails).

### Merge clips

```json
{
  "job_id": "merge_001",
  "callback_url": "https://api.example.com/webhook",
  "config": {
    "canvas": { "width": 1080, "height": 1920, "bg_mode": "blur" }
  },
  "clips": [
    {
      "source_video": "/path/to/clip1.mp4",
      "trim": { "start_sec": 10, "end_sec": 30 },
      "hook": {
        "text": "Clip 1 Title",
        "font_file": "/path/to/font.ttf",
        "font_size": 80,
        "font_color": "white",
        "duration": 1.0
      }
    },
    {
      "source_video": "/path/to/clip2.mp4",
      "trim": { "start_sec": 30, "end_sec": 60 },
      "hook": {
        "text": "Clip 2 Title",
        "font_file": "/path/to/font.ttf",
        "font_size": 80,
        "font_color": "white",
        "duration": 1.5
      }
    }
  ],
  "output_path": "/path/to/merged.mp4"
}
```

## Field Reference

### `assets`

| Field | Required | Description |
|---|---|---|
| `source_video` | yes | Path to input video |
| `subtitle_file` | yes | Path to `.ass` subtitle file |

### `config.trim`

| Field | Required | Description |
|---|---|---|
| `start_sec` | no | Start time (seconds) |
| `end_sec` | no | End time (seconds) |

### `config.canvas`

| Field | Required | Default | Description |
|---|---|---|---|
| `width` | yes | — | Output width (px) |
| `height` | yes | — | Output height (px) |
| `bg_mode` | no | `blur` | Background fill (`blur` or empty) |

### `config.hook`

Hook renders only when at least one of `text` or `lines` is provided AND `font_file` is provided.

| Field | Type | Default | Description |
|---|---|---|---|
| `text` | string | — | Single-line hook text |
| `lines` | []string | — | Multi-line hook (one entry per line) |
| `font_file` | string | — | Path to `.ttf` font |
| `font_size` | float | — | Font size in px |
| `font_color` | string | `white` | Text color |
| `y_position` | float | auto | Y offset from top (px) |
| `position` | string | — | `"top"`, `"center"`, `"bottom"` — overrides `y_position` |
| `duration` | float | `1.5` | Hook visible time (seconds) |
| `stroke_width` | int | `0` | Text outline width |
| `shadow` | bool | `false` | Enable drop shadow |
| `box_bg` | string | — | Background box color (e.g. `black@0.7`) |
| `box_padding` | int | `0` | Box padding (px) |
| `accent_text` | string | — | Word to highlight in accent color |
| `accent_color` | string | — | Color for accent text |
| `image` | string | — | Hook background image (merge mode) |

## Thumbnails

Every successful render (single + merge) also produces a JPG thumbnail automatically when the hook has `text`/`lines` + `font_file`.

- **Single** → 1 thumbnail, path in response `thumbnail`.
- **Merge** → 1 thumbnail per clip, path list in response `thumbnails` (`<output>_N_thumb.jpg`).
- **Frame**: random timestamp inside the clip trim range, picked from the source video before rendering.
- **Size**: same as `canvas`.
- **Position**: 9-grid, default `auto` = pure random; override via `thumbnail.position`.
- **Longest word**: rendered in `highlight_color` inline via libass `{\c}` override.

### `config.thumbnail`

| Field | Type | Default | Description |
|---|---|---|---|
| `path` | string | `<output>_thumb.jpg` | Thumbnail output path (merge: per-clip `_N_thumb.jpg` when unset) |
| `position` | string | `auto` | `auto` or `top-left`, `top-center`, `top-right`, `middle-left`, `center`, `middle-center`, `middle-right`, `bottom-left`, `bottom-center`, `bottom-right` |
| `text_color` | string | random palette | Base text color (`#RRGGBB`) |
| `highlight_color` | string | random palette | Longest-word color (`#RRGGBB`) |
| `font_size` | float | `hook.font_size` | Thumbnail font size |
| `padding` | int | `40` | Edge margin (px) |

Default palette (boring modern enterprise, picked randomly unless overridden):

| Base | Highlight |
|---|---|
| `#FFFFFF` | `#2563EB` |
| `#F8FAFC` | `#475569` |
| `#FFFFFF` | `#1E40AF` |
| `#F1F5F9` | `#334155` |
| `#FFFFFF` | `#0EA5E9` |

**Sample:** `samples/thumbnail.json`

## Callback / Webhook

Add `callback_url` to the payload:

```json
{
  "job_id": "clip_007",
  "callback_url": "https://api.example.com/webhook/mediaclip",
  ...
}
```

**API mode:** Returns `202 Accepted` immediately. After rendering completes, POSTs the result JSON to `callback_url`. HTTP client has 10s timeout.

**CLI mode:** Ignored. Result always printed to stdout.

## Subtitle Modes

Subtitle modes are driven entirely by `.ass` file formatting — no Go code changes needed.

| Mode | ASS structure | Behavior |
|---|---|---|
| **Line-by-line** | One Dialogue per phrase | Standard subtitle rendering |
| **Word-by-word** | One Dialogue per word, staggered timing | Each word appears and disappears individually |
| **Karaoke + accent** | `\K` tags per word, `SecondaryColour` = accent | Current word in yellow, past/future in cyan |
| **Dynamic** | Layer 0 = full line, Layer 1 = per-word highlight | Full text always visible, one word highlighted at a time |

### Generating ASS files

Use `validate_subtitles.py` to verify timeline correctness:

```bash
# After generating .ass files:
python3 validate_subtitles.py
```

Validates: non-negative durations, no overlapping events within a layer, max 1 active cue per layer per frame.

## Hook Styles

### Editorial

White text, bold, stroke + shadow, centered. No background.

```json
"hook": {
  "lines": ["KENAPA DIA", "MAU DIPUJI?"],
  "font_size": 80,
  "font_color": "white",
  "stroke_width": 6,
  "shadow": true,
  "duration": 1.5
}
```

**Sample:** `samples/editorial.json`

### Punch Card

Single line with semi-transparent black background box.

```json
"hook": {
  "lines": ["MAU DIPUJI?"],
  "font_size": 92,
  "box_bg": "black@0.7",
  "box_padding": 28,
  "stroke_width": 4,
  "shadow": true
}
```

**Sample:** `samples/punchcard.json`

### Highlight

Two lines: first line white, second line in accent color.

```json
"hook": {
  "lines": ["KENAPA DIA", "MAU DIPUJI?"],
  "font_size": 80,
  "stroke_width": 6,
  "shadow": true,
  "accent_text": "MAU DIPUJI?",
  "accent_color": "yellow"
}
```

**Sample:** `samples/highlight.json`

## Positioning

Hook position via `position` (semantic) or `y_position` (explicit px):

| Position | Behavior |
|---|---|
| `"top"` | Text block top at 12% of canvas height |
| `"center"` | Vertically centered |
| `"bottom"` | 350px above bottom edge |
| `y_position: 300` | Fixed Y offset from top (baseline of first line) |

If neither `position` nor `y_position` is set, defaults to `top` position.

## Filter Chain

```
[0:v] scale + crop + boxblur [bg]
[0:v] scale [fg]
[bg][fg] overlay [overlaid]
[overlaid] ass [subbed]              ← subtitles (bottom layer)
[subbed] drawtext [out]              ← hook (top layer)
```

ASS subtitles render BEFORE drawtext, so hook renders ON TOP of subtitles.

## Commands

```bash
go build ./cmd/mediaclip/      # Build binary
go test ./... -count=1         # Run all tests
go test ./... -cover           # Coverage report
go vet ./...                   # Static analysis
go test ./... -race            # Race detection
python3 validate_subtitles.py  # Subtitle timeline validation
```

## Sample Files

| File | Description |
|---|---|
| `samples/editorial.json` | Editorial hook + line subs |
| `samples/punchcard.json` | Punch card hook + line subs |
| `samples/highlight.json` | Highlight hook + line subs |
| `samples/top.json` | Hook at top position |
| `samples/center.json` | Hook at center position |
| `samples/bottom.json` | Hook at bottom position |
| `samples/sub_line.json` | Line-by-line subtitles |
| `samples/sub_word.json` | Word-by-word subtitles |
| `samples/sub_karaoke_acc.json` | Karaoke with accent color |
| `samples/sub_karaoke_dynamic.json` | Dynamic word highlight |
| `samples/thumbnail.json` | Thumbnail generation |
| `samples/merge.json` | 2-clip merge |
| `samples/merge_multi.json` | 4-clip merge |

Source video: `yt.mp4`

Subtitle source: `subtitle.json` (raw ASR transcript)

## Technical Notes

- **FFmpeg filter quoting** — Single quotes, backslashes, and colons need careful handling inside Go strings for `-filter_complex`
- **ASS layer stacking** — libass stacks vertically when multiple Dialogue events overlap in time. Ensure ≥10ms gap between consecutive events within a layer
- **`\n` in drawtext** — Use actual newline byte (`\n` in Go), not `\\n`. FFmpeg interprets real newlines in the text parameter
- **`enable` expression** — `enable='between(t,0,DURATION)'` limits drawtext visibility
- **Timeout** — 10 minute default context timeout via `exec.CommandContext`
- **Stderr capture** — Always read FFmpeg stderr for diagnostics
- **Callback timeout** — HTTP POST to `callback_url` has 10s client timeout

## Philosophy

- **"Stupid Machine"** — no AI, no transcription, no decision-making. All assets arrive pre-processed
- **Fail-fast** — validate all input files exist before any FFmpeg call
- **Stateless** — no database, no session, no persistence. Single binary
- **Minimal deps** — Go stdlib only (`os/exec`, `encoding/json`, `net/http`)
