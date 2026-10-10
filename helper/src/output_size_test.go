package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
)

func newFinishTestApp(t *testing.T, j *job) (*app, *bytes.Buffer) {
	t.Helper()
	var out bytes.Buffer
	return &app{
		jobs:        map[string]*job{j.ID: j},
		logger:      log.New(io.Discard, "", 0),
		out:         &out,
		runtimePath: filepath.Join(t.TempDir(), "runtime-state.json"),
	}, &out
}

// A finished Dailymotion job reported "73.3 MB / 73.3 MB" for a 22,822,230-byte
// file because those figures come from yt-dlp's (HLS estimate) progress text.
// The finished job must carry the real size of the saved file separately.
func TestFinishedJobReportsActualFileSizeSeparatelyFromProgressEstimates(t *testing.T) {
	const actual = 22822230
	path := filepath.Join(t.TempDir(), "video.mp4")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(actual); err != nil {
		t.Fatal(err)
	}
	file.Close()

	j := &job{ID: "dm", State: "finalizing", source: sourceDailymotion, TotalBytes: 76861645, DownloadedBytes: 76861645, Percent: 100}
	a, out := newFinishTestApp(t, j)
	a.finishJob(j, "finished", path, "")

	if j.OutputBytes != actual {
		t.Fatalf("OutputBytes = %d, want %d", j.OutputBytes, actual)
	}
	if j.TotalBytes != 76861645 || j.DownloadedBytes != 76861645 {
		t.Fatal("progress-derived counters must not be rewritten to look like the output size")
	}
	// One native-message frame: 4-byte length prefix, then the JSON event.
	var event map[string]any
	if out.Len() < 5 || json.Unmarshal(out.Bytes()[4:], &event) != nil {
		t.Fatalf("unexpected output: %q", out.Bytes())
	}
	jobFrame, _ := event["job"].(map[string]any)
	if jobFrame == nil || jobFrame["outputBytes"] != float64(actual) || jobFrame["state"] != "finished" {
		t.Fatalf("progress event = %v", event)
	}
}

func TestOutputBytesAreOnlySetForFinishedJobsWithARegularFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "x.mp4")
	if err := os.WriteFile(file, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, state, path string
	}{
		{"failed", "failed", file},
		{"cancelled", "cancelled", file},
		{"finished without a path", "finished", ""},
		{"finished with a missing file", "finished", filepath.Join(dir, "missing.mp4")},
		{"finished with a directory", "finished", dir},
	} {
		j := &job{ID: "j"}
		a, _ := newFinishTestApp(t, j)
		a.finishJob(j, test.state, test.path, "")
		if j.OutputBytes != 0 {
			t.Errorf("%s: OutputBytes = %d, want 0", test.name, j.OutputBytes)
		}
	}
	data, err := json.Marshal(job{ID: "j", State: "downloading"})
	if err != nil || bytes.Contains(data, []byte("outputBytes")) {
		t.Fatalf("running jobs must omit outputBytes: %s %v", data, err)
	}
}
