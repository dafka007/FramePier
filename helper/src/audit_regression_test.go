package main

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"
)

func TestDuplicateDownloadIDPreservesExistingJob(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	original := &job{ID: "existing", State: "downloading", cancel: cancel}
	var output bytes.Buffer
	a := &app{jobs: map[string]*job{"existing": original}, out: &output}
	a.startDownload(request{ID: "existing", URL: "https://www.youtube.com/watch?v=2PuFyjAs7JA", Mode: "video", Quality: "best", Container: "mp4", AudioFormat: "best"}, response{})
	if a.jobs["existing"] != original || ctx.Err() != nil {
		t.Fatal("duplicate request replaced or cancelled the existing job")
	}
	if !bytes.Contains(output.Bytes(), []byte("already in use")) {
		t.Fatalf("missing duplicate request error: %q", output.Bytes())
	}
}

func TestCompletedHistoryIsBoundedWithoutRemovingActiveJobs(t *testing.T) {
	a := &app{jobs: map[string]*job{"active": {ID: "active", State: "downloading"}}}
	for i := 0; i < 150; i++ {
		id := fmt.Sprint(i)
		a.jobs[id] = &job{ID: id, State: "finished", completedAt: time.Unix(int64(i+1), 0)}
	}
	a.pruneCompletedJobs()
	if len(a.jobs) != maxCompletedJobs+1 || a.jobs["active"] == nil || a.jobs["0"] != nil {
		t.Fatal("history pruning removed active work or retained too many jobs")
	}
}

func TestCancelFinishedJobDoesNotAlterCompletion(t *testing.T) {
	var output bytes.Buffer
	j := &job{ID: "done", State: "finished"}
	a := &app{jobs: map[string]*job{"done": j}, out: &output}
	a.cancelDownload(request{ID: "done"}, response{})
	if j.cancelRequested || j.State != "finished" {
		t.Fatal("late cancellation changed a finished job")
	}
}

func TestDiagnosticDoesNotRegressFinalizingProgress(t *testing.T) {
	var output bytes.Buffer
	a := &app{out: &output}
	j := &job{State: "finalizing", Percent: 100}
	a.parseProgress(j, "WARNING: unrelated diagnostic")
	if j.State != "finalizing" || output.Len() != 0 {
		t.Fatal("diagnostic changed state or emitted spurious progress")
	}
	a.parseProgress(j, "[download] 25.0% of 4.00MiB at 1MiB/s ETA 00:03")
	if j.State != "downloading" || j.Percent != 25 || j.TotalBytes != 4194304 {
		t.Fatalf("real progress lost: %+v", j)
	}
}

func TestByteSizeRejectsNonFiniteAndOverflow(t *testing.T) {
	for _, value := range []string{"NaN", "+Inf", "-Inf", "9223372036854775808", "9223372036854775807", "1e99"} {
		if n, ok := parseByteSize(value, "B"); ok {
			t.Errorf("accepted unrepresentable byte count %s as %d", value, n)
		}
	}
	if n, ok := parseByteSize("1024", "B"); !ok || n != 1024 {
		t.Fatal("valid byte count rejected")
	}
}
