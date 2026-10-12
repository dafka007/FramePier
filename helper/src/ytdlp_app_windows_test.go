package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func decodeFrames(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	var frames []map[string]any
	reader := bytes.NewReader(data)
	for reader.Len() > 0 {
		var size uint32
		if err := binary.Read(reader, binary.LittleEndian, &size); err != nil {
			t.Fatal(err)
		}
		payload := make([]byte, size)
		if _, err := io.ReadFull(reader, payload); err != nil {
			t.Fatal(err)
		}
		var frame map[string]any
		if err := json.Unmarshal(payload, &frame); err != nil {
			t.Fatal(err)
		}
		frames = append(frames, frame)
	}
	return frames
}

// newRuntimeTestApp builds an app whose root holds a placeholder yt-dlp.exe
// (never a valid executable), no FFmpeg and no JavaScript runtime. toolPath
// falls back to PATH, so PATH is emptied for the test: a developer machine with
// FFmpeg installed must not change what these tests see.
func newRuntimeTestApp(t *testing.T, out *bytes.Buffer) *app {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "tools"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tools", "yt-dlp.exe"), []byte("not an executable"), 0o700); err != nil {
		t.Fatal(err)
	}
	fakeRuntimeProbe(t, denoVersionOutput, nil)
	a := &app{
		jobs:        map[string]*job{},
		root:        root,
		dataDir:     t.TempDir(),
		logger:      log.New(io.Discard, "", 0),
		out:         out,
		runtimePath: filepath.Join(t.TempDir(), "runtime-state.json"),
	}
	// If a test ever does start a download, stop it and wait for its goroutine
	// before the temporary directories are removed.
	t.Cleanup(func() {
		a.mu.RLock()
		for _, j := range a.jobs {
			if j.cancel != nil {
				j.cancel()
			}
		}
		a.mu.RUnlock()
		a.jobsWG.Wait()
	})
	return a
}

func TestRuntimeTestAppHasNoFFmpegEvenIfInstalledOnPath(t *testing.T) {
	var out bytes.Buffer
	a := newRuntimeTestApp(t, &out)
	if path, err := a.toolPath("ffmpeg.exe"); err == nil {
		t.Fatalf("fixture must not find FFmpeg, found %q", path)
	}
	if _, err := a.toolPath("yt-dlp.exe"); err != nil {
		t.Fatalf("fixture must still provide its placeholder yt-dlp.exe: %v", err)
	}
}

func TestYtdlpCommandPutsFixedOptionsFirstAndKeepsHiddenWindow(t *testing.T) {
	runtime := `C:\Program Files\FramePier\tools\deno.exe`
	cmd := newYtdlpCommandContext(context.Background(), `C:\Program Files\FramePier\tools\yt-dlp.exe`, runtime, sourceYouTube, "--dump-single-json", "https://www.youtube.com/watch?v=dQw4w9WgXcQ")
	want := ytdlpPrefixArgs(runtime)
	for i, arg := range want {
		if cmd.Args[1+i] != arg {
			t.Fatalf("arg %d = %q, want %q (all: %q)", i, cmd.Args[1+i], arg, cmd.Args)
		}
	}
	if cmd.Args[len(cmd.Args)-1] != "https://www.youtube.com/watch?v=dQw4w9WgXcQ" || cmd.Args[len(cmd.Args)-2] != "--dump-single-json" {
		t.Fatalf("caller arguments changed: %q", cmd.Args)
	}
	if cmd.SysProcAttr == nil || cmd.SysProcAttr.CreationFlags&createNoWindow == 0 || !cmd.SysProcAttr.HideWindow {
		t.Fatal("yt-dlp must still run without a console window")
	}
}

func TestYtdlpSelfUpdateIsNotReachable(t *testing.T) {
	// The bundled yt-dlp.exe is a pinned, verified build. No request may make the helper run its -U updater.
	for _, payload := range []string{
		`{"id":"u1","command":"update_ytdlp"}`,
		`{"id":"u2","command":"update_ytdlp","url":"https://github.com/yt-dlp/yt-dlp"}`,
	} {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal([]byte(payload), &raw); err != nil {
			t.Fatal(err)
		}
		if _, err := decodeRequest([]byte(payload), raw); err == nil {
			t.Fatalf("update_ytdlp must be rejected: %s", payload)
		}
	}
	for _, name := range []string{"main.go", "formats.go", "ytdlp_command.go"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		source := string(data)
		for _, forbidden := range []string{"update_ytdlp", "updateYTDLP", "newYtdlpCommand(", `"-U"`, `"--update"`, "--update-to"} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s still contains a yt-dlp self-update path: %s", name, forbidden)
			}
		}
	}
}

func TestVideoInfoForYouTubeWithoutRuntimeFailsBeforeLaunchingYtdlp(t *testing.T) {
	var out bytes.Buffer
	a := newRuntimeTestApp(t, &out)
	req := request{ID: "1", Command: "get_video_info", URL: "https://www.youtube.com/watch?v=dQw4w9WgXcQ"}
	a.videoInfo(req, response{"id": "1", "command": "get_video_info"})
	frames := decodeFrames(t, out.Bytes())
	if len(frames) != 1 || frames[0]["ok"] != false || frames[0]["error"] != msgJSRuntimeUnavailable {
		t.Fatalf("frames = %v", frames)
	}
}

func TestVideoInfoForTwitchDoesNotRequireTheRuntime(t *testing.T) {
	var out bytes.Buffer
	a := newRuntimeTestApp(t, &out)
	req := request{ID: "2", Command: "get_video_info", URL: "https://clips.twitch.tv/AbCdEf123456"}
	a.videoInfo(req, response{"id": "2", "command": "get_video_info"})
	frames := decodeFrames(t, out.Bytes())
	if len(frames) != 1 || frames[0]["ok"] != false {
		t.Fatalf("frames = %v", frames)
	}
	if frames[0]["error"] == msgJSRuntimeUnavailable {
		t.Fatal("Twitch must reach the downloader even when the runtime is missing")
	}
}

func TestStartDownloadForYouTubeWithoutRuntimeFailsTheJobWithoutLaunching(t *testing.T) {
	var out bytes.Buffer
	a := newRuntimeTestApp(t, &out)
	req := request{ID: "job1", Command: "start_download", URL: "https://www.youtube.com/watch?v=dQw4w9WgXcQ", Mode: "video", Quality: "best", Container: "mp4", AudioFormat: "best"}
	a.startDownload(req, response{"id": "job1", "command": "start_download"})
	var final map[string]any
	for _, frame := range decodeFrames(t, out.Bytes()) {
		if frame["id"] == "job1" {
			final = frame
		}
	}
	if final == nil || final["ok"] != false || final["error"] != msgJSRuntimeUnavailable {
		t.Fatalf("response = %v", final)
	}
	j := a.jobs["job1"]
	if j == nil || j.State != "failed" || j.Error != msgJSRuntimeUnavailable {
		t.Fatalf("job = %+v", j)
	}
}

func TestVersionsReportsJavaScriptRuntimeStatus(t *testing.T) {
	var out bytes.Buffer
	a := newRuntimeTestApp(t, &out)
	status, ok := a.versions()["jsRuntime"].(response)
	if !ok || status["installed"] != false || status["usable"] != false {
		t.Fatalf("jsRuntime = %v", a.versions()["jsRuntime"])
	}
}

func TestEveryYtdlpExtractionGoesThroughTheCentralBuilder(t *testing.T) {
	for _, name := range []string{"main.go", "formats.go"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		source := string(data)
		for _, forbidden := range []string{"newHiddenCommandContext(ctx, ytdlp", "newHiddenCommand(path, \"-U\")"} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s builds a yt-dlp command outside ytdlp_command.go: %s", name, forbidden)
			}
		}
	}
}

func TestYtdlpCommandRestrictsOnlyDailymotionToItsExtractor(t *testing.T) {
	const exe = `C:\Program Files\FramePier\tools\yt-dlp.exe`
	const deno = `C:\Program Files\FramePier\tools\deno.exe`
	url := "https://www.dailymotion.com/video/xmo8jh"

	cmd := newYtdlpCommandContext(context.Background(), exe, "", sourceDailymotion, "--dump-single-json", url)
	want := append(ytdlpPrefixArgs(""), "--use-extractors", "dailymotion", "--dump-single-json", url)
	if strings.Join(cmd.Args[1:], "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("Dailymotion args = %q, want %q", cmd.Args[1:], want)
	}
	for _, arg := range cmd.Args {
		if arg == "--js-runtimes" {
			t.Fatal("Dailymotion must not enable a JavaScript runtime")
		}
	}
	if cmd.SysProcAttr == nil || cmd.SysProcAttr.CreationFlags&createNoWindow == 0 || !cmd.SysProcAttr.HideWindow {
		t.Fatal("yt-dlp must still run without a console window")
	}

	// YouTube and Twitch command lines are exactly the fixed prefix plus the
	// caller's arguments, as before the Dailymotion work.
	for _, test := range []struct {
		source  mediaSource
		runtime string
	}{{sourceYouTube, deno}, {sourceTwitchClip, ""}, {sourceTwitchVOD, ""}} {
		cmd := newYtdlpCommandContext(context.Background(), exe, test.runtime, test.source, "--dump-single-json", "x")
		want := append(ytdlpPrefixArgs(test.runtime), "--dump-single-json", "x")
		if strings.Join(cmd.Args[1:], "\x00") != strings.Join(want, "\x00") {
			t.Fatalf("%s args changed: %q", test.source, cmd.Args[1:])
		}
		for _, arg := range cmd.Args {
			if arg == "--use-extractors" {
				t.Fatalf("%s must not be restricted to an extractor", test.source)
			}
		}
	}
}

func TestVideoInfoForDailymotionDoesNotRequireTheRuntime(t *testing.T) {
	var out bytes.Buffer
	a := newRuntimeTestApp(t, &out)
	req := request{ID: "3", Command: "get_video_info", URL: "https://www.dailymotion.com/video/xmo8jh"}
	a.videoInfo(req, response{"id": "3", "command": "get_video_info"})
	frames := decodeFrames(t, out.Bytes())
	if len(frames) != 1 || frames[0]["ok"] != false {
		t.Fatalf("frames = %v", frames)
	}
	if frames[0]["error"] == msgJSRuntimeUnavailable || strings.Contains(frames[0]["error"].(string), "Only") {
		t.Fatalf("Dailymotion must pass validation and reach the downloader: %v", frames[0]["error"])
	}
}

func TestVideoInfoRejectsUnsupportedDailymotionURLsBeforeLaunchingYtdlp(t *testing.T) {
	for _, value := range []string{"https://www.dailymotion.com/playlist/x7wdsj", "https://dai.ly/xmo8jh", "https://www.dailymotion.com/video/xmo8jh@evil.test"} {
		var out bytes.Buffer
		a := newRuntimeTestApp(t, &out)
		a.videoInfo(request{ID: "4", Command: "get_video_info", URL: value}, response{"id": "4"})
		frames := decodeFrames(t, out.Bytes())
		if len(frames) != 1 || frames[0]["ok"] != false || frames[0]["video"] != nil {
			t.Fatalf("%s: frames = %v", value, frames)
		}
	}
}

func TestStartDownloadForDailymotionPassesTheRuntimeGate(t *testing.T) {
	var out bytes.Buffer
	a := newRuntimeTestApp(t, &out)
	// The test root has no ffmpeg.exe, so the request stops at the FFmpeg check,
	// which comes after the runtime gate that only YouTube must pass.
	req := request{ID: "job2", Command: "start_download", URL: "https://www.dailymotion.com/video/xmo8jh", Mode: "video", Quality: "best", Container: "mp4", AudioFormat: "best"}
	a.startDownload(req, response{"id": "job2", "command": "start_download"})
	var final map[string]any
	for _, frame := range decodeFrames(t, out.Bytes()) {
		if frame["id"] == "job2" {
			final = frame
		}
	}
	if final == nil || final["error"] == msgJSRuntimeUnavailable || final["error"] != "ffmpeg is required for this format." {
		t.Fatalf("response = %v", final)
	}
	if j := a.jobs["job2"]; j == nil || j.source != sourceDailymotion || j.State != "failed" {
		t.Fatalf("job = %+v (the request must stop before any download starts)", j)
	}
}

func TestDailymotionAudioDownloadRequiresFFmpegEvenForNativeAudio(t *testing.T) {
	var out bytes.Buffer
	a := newRuntimeTestApp(t, &out)
	req := request{ID: "job3", Command: "start_download", URL: "https://www.dailymotion.com/video/xmo8jh", Mode: "audio", Quality: "best", Container: "mp4", AudioFormat: "best"}
	a.startDownload(req, response{"id": "job3", "command": "start_download"})
	var final map[string]any
	for _, frame := range decodeFrames(t, out.Bytes()) {
		if frame["id"] == "job3" {
			final = frame
		}
	}
	if final == nil || final["error"] != "ffmpeg is required for this format." {
		t.Fatalf("response = %v", final)
	}
}

func TestCancellingADailymotionJobUsesTheSharedCancellationPath(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var out bytes.Buffer
	j := &job{ID: "dm", State: "downloading", source: sourceDailymotion, cancel: cancel}
	a := &app{jobs: map[string]*job{"dm": j}, out: &out}
	a.cancelDownload(request{ID: "dm"}, response{"id": "dm"})
	if ctx.Err() == nil || !j.cancelRequested {
		t.Fatal("cancel did not reach the Dailymotion job")
	}
	frames := decodeFrames(t, out.Bytes())
	if len(frames) != 1 || frames[0]["ok"] != true {
		t.Fatalf("frames = %v", frames)
	}
}

func TestEveryDailymotionAndYtdlpDownloadPathStillUsesTheCentralBuilder(t *testing.T) {
	data, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "newYtdlpCommandContext(ctx, ytdlp, runtimePath, target.Source, args...)") {
		t.Fatal("downloads must pass the source to the central yt-dlp builder")
	}
	if !strings.Contains(string(data), `"--match-filters", "!is_live"`) {
		t.Fatal("Dailymotion downloads must refuse live streams")
	}
}
