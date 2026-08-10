# Plan: Mode `sections`

New entrypoint mode that renders a sequence of sections — each section is an
image + audio pair — into ONE concatenated video. A "section" is a faceless
slideshow-style segment: the image becomes the video background, the audio
drives the duration, subtitles + hook text render on top.

## Decisions (locked)

| Question | Decision |
|---|---|
| Output | One concatenated video (all sections → single file) |
| Image role | Video background (full canvas) |
| Duration | Follows audio duration (probe each audio file) |
| Subtitle | Per-section ASS file, timestamps 0-based relative to that section's audio; burned before concat |
| Thumbnail | From the section image, random position (reuse thumbnail pipeline) |
| Render approach | Per-section to temp mp4, then concat (2-pass) |
| Entrypoint | New top-level `mode: "sections"` field |

## Why per-section ASS (the "jebakan")

The video is **built**, not cropped. So each section's timeline starts at 0.
Give every section its own `subtitle_file` whose ASS timestamps are relative
to that section's audio (0-based). When rendering section `i` standalone, the
ASS is applied to a stream reset to `PTS-STARTPTS`, so timestamps align
naturally — **no offset shifting, no cumulative duration math**. Concatenating
pre-rendered segments makes this trivial because each segment already carries
its burned-in subtitles and hook.

## Payload schema

```json
{
  "job_id": "sections_01",
  "mode": "sections",
  "callback_url": "https://.../cb",        // optional, same as other modes
  "output_path": "/out/final.mp4",
  "canvas": { "width": 1080, "height": 1920, "bg_mode": "blur" },
  "thumbnail": { "position": "auto", "text_color": "#FFFFFF", "highlight_color": "#2563EB" },
  "sections": [
    {
      "image": "/assets/slide1.jpg",
      "audio": "/assets/voice1.mp3",
      "subtitle_file": "/assets/voice1.ass",
      "bg_fill": "cover",
      "hook": {
        "text": "donal trum makan cobek",
        "font_file": "/fonts/Verdana Bold.ttf",
        "font_size": 80,
        "duration": 1.5
      }
    },
    {
      "image": "/assets/slide2.jpg",
      "audio": "/assets/voice2.mp3",
      "subtitle_file": "/assets/voice2.ass",
      "bg_fill": "blur",
      "hook": { "lines": ["baris satu", "baris dua"], "font_file": "/fonts/Verdana Bold.ttf", "font_size": 80 }
    }
  ]
}
```

## New structs (`internal/config`)

```go
type SectionConfig struct {
    Image         string     `json:"image"`
    Audio         string     `json:"audio"`
    SubtitleFile  string     `json:"subtitle_file"`
    BgFill        string     `json:"bg_fill"`      // "cover" (default) | "contain" | "blur"
    Hook          HookConfig `json:"hook"`
}

// JobPayload gains:
Mode     string          `json:"mode,omitempty"`   // "single" | "merge" | "sections"
Sections []SectionConfig `json:"sections,omitempty"`
```

- `IsSections(p)` → `p.Mode == "sections"`.
- `ValidateSectionsPayload(p)`:
  - canvas W/H positive, bg_mode in `blur|""`
  - each section: `image` + `audio` required, `fs.FileExists` both
  - `subtitle_file` required (it drives the sync) — `FileExists`
  - `bg_fill` in `cover|contain|blur` (empty → cover)
  - hook: font_size > 0 if text/lines present; font_file exists if set
  - same single-quote rejection as other modes for all injected paths

## Rendering: 2-pass per-section → concat

### Pass 1 — one ffmpeg command per section (temp segment)

```bash
ffmpeg -loop 1 -t <audio_dur> -i slide.jpg \
       -i voice.mp3 \
       -filter_complex "
  [0:v]scale=<W>:<H>:force_original_aspect_ratio=increase,crop=<W>:<H><blur>[bg];
  [0:v]scale=<W>:<H>:force_original_aspect_ratio=decrease[fg];
  [bg][fg]overlay=(W-w)/2:(H-h)/2[v1];
  [v1]ass=filename=voice1.ass[sub];
  [sub]drawtext=<hook>[out]"
       -map [out] -map 1:a \
       -t <audio_dur> -c:v libx264 -c:a aac <tmp>/sec_0.mp4
```

- `<audio_dur>` comes from `executor.ProbeDuration(audio)`.
- `bg_fill`:
  - `cover` (default) → scale increase + crop (existing bg pattern)
  - `contain` → scale decrease only, no crop; pad via canvas bg (blur or solid)
  - `blur` → cover + `boxblur` on bg layer
- ASS burned here, hook drawtext on top (existing `buildDrawtext`).
- Temp dir: `os.MkdirTemp("", "mediaclip_sections_*")`, cleaned after.

### Pass 2 — concat segments

```bash
ffmpeg -f concat -safe 0 -i concat.txt -c copy final.mp4
```

- `concat.txt`: `file '/tmp/.../sec_0.mp4'` lines.
- Use `-c copy` (segments already H.264+AAC, same params) — fast, lossless.

### Thumbnails (per section)

- Reuse `generateThumbnail`/`BuildThumbnailArgs` — but source is now the
  section **image**, not a video. Adaptation needed: for an image source,
  `BuildThumbnailArgs` should skip `-ss` probing (or use `-i image` directly
  with `-loop 1 -frames:v 1`). Frame timestamp concept doesn't apply — render
  the image directly.
- Output: `<output>_N_thumb.jpg` (idx per section), response `thumbnails`.
- Position/colors: reuse `thumbnail.*` config (random default).

## Package changes

| Package | Change |
|---|---|
| `internal/config` | `SectionConfig`, `Mode`, `Sections`, `IsSections`, `ValidateSectionsPayload` |
| `internal/builder` | `BuildSectionSegmentArgs(section, canvas, tmpAss, out, dur)`; `BuildSectionConcatArgs(segments, out)`; reuse `buildDrawtext`, `escapeFilterPath` |
| `internal/executor` | reuse `ProbeDuration` (already has ffprobe + ffmpeg fallback) |
| `cmd/mediaclip/main.go` | `executeSections(payload)` — per-section temp ASS write, pass-1 render, pass-2 concat, thumbnails, cleanup; branch in `handleRender` + `runCLI` |
| `README.md` / `AGENTS.md` | document mode + filter chain order + section flow |

## Response format

Same as other modes:

```json
{ "status": "success", "output": "/out/final.mp4", "thumbnails": ["/out/final_0_thumb.jpg", "/out/final_1_thumb.jpg"] }
{ "status": "error", "error": "..." }
```

## Tests

- `config`: sections validation happy path + each error (missing image/audio/sub,
  bad bg_fill, single-quote path)
- `builder`: segment args contain `-loop 1`, `ass=filename=`, drawtext, correct
  `-t`; concat args contain `-f concat -c copy`; bg_fill variants produce
  correct scale/crop/blur
- `executor`: ProbeDuration already covered
- E2E: 2-section payload with real image+audio, assert output exists + thumbnails
- Corrupt/nonexistent audio → clean error, temp files removed

## Task order

1. `config` — structs + `IsSections` + `ValidateSectionsPayload` + tests
2. `builder` — `BuildSectionSegmentArgs` + `BuildSectionConcatArgs` + tests
3. `main.go` — `executeSections` (temp ASS + pass1 + pass2 + thumbnail + cleanup) + branches
4. `samples/payload_sections.json` + E2E verification
5. README + AGENTS.md
6. `go build` + `go vet` + `go test ./... -count=1` + commit + push
