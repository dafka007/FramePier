package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func fakeRuntimeProbe(t *testing.T, output string, err error) *int {
	t.Helper()
	calls := 0
	previous := probeJSRuntimeVersion
	probeJSRuntimeVersion = func(string) (string, error) {
		calls++
		return output, err
	}
	t.Cleanup(func() { probeJSRuntimeVersion = previous })
	return &calls
}

func writeFakeRuntime(t *testing.T, root, content string) string {
	t.Helper()
	dir := filepath.Join(root, "tools")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, jsRuntimeFileName)
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

const denoVersionOutput = "deno 2.9.7 (stable, release, x86_64-pc-windows-msvc)\nv8 14.0.0\ntypescript 5.9.0\n"

func TestYtdlpPrefixArgsSelectBundledRuntimeDeterministically(t *testing.T) {
	path := `C:\Users\Test User\AppData\Local\Programs\FramePier\tools\deno.exe`
	want := []string{"--ignore-config", "--no-plugin-dirs", "--no-remote-components", "--no-js-runtimes", "--js-runtimes", "deno:" + path}
	got := ytdlpPrefixArgs(path)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("prefix args = %q, want %q", got, want)
	}
}

func TestYtdlpPrefixArgsWithoutRuntimeStillClearDefaults(t *testing.T) {
	want := []string{"--ignore-config", "--no-plugin-dirs", "--no-remote-components", "--no-js-runtimes"}
	got := ytdlpPrefixArgs("")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("prefix args = %q, want %q", got, want)
	}
	for _, arg := range got {
		if arg == "--js-runtimes" {
			t.Fatal("no runtime may be enabled when none is required")
		}
	}
}

func TestInspectJSRuntimeAcceptsOnlyBundledDeno(t *testing.T) {
	root := t.TempDir()
	path := writeFakeRuntime(t, root, "MZ fake")
	calls := fakeRuntimeProbe(t, denoVersionOutput, nil)
	info, _, err := inspectJSRuntime(root)
	if err != nil {
		t.Fatal(err)
	}
	if info.Version != "2.9.7" {
		t.Fatalf("version = %q", info.Version)
	}
	absolute, _ := filepath.Abs(path)
	if info.Path != absolute || !filepath.IsAbs(info.Path) {
		t.Fatalf("path = %q, want absolute %q", info.Path, absolute)
	}
	if *calls != 1 {
		t.Fatalf("probe calls = %d", *calls)
	}
}

func TestInspectJSRuntimeRejectsUnusableRuntimes(t *testing.T) {
	cases := map[string]func(t *testing.T, root string){
		"missing": func(t *testing.T, root string) {},
		"empty":   func(t *testing.T, root string) { writeFakeRuntime(t, root, "") },
		"directory": func(t *testing.T, root string) {
			if err := os.MkdirAll(filepath.Join(root, "tools", jsRuntimeFileName), 0o700); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			setup(t, root)
			calls := fakeRuntimeProbe(t, denoVersionOutput, nil)
			if _, _, err := inspectJSRuntime(root); !errors.Is(err, errJSRuntimeUnavailable) {
				t.Fatalf("err = %v", err)
			}
			if *calls != 0 {
				t.Fatal("an invalid file must not be executed")
			}
		})
	}
	probeCases := map[string]struct {
		output string
		err    error
	}{
		"probe fails":     {"", errors.New("exit status 1")},
		"not deno":        {"node v24.19.0\n", nil},
		"no version":      {"deno\n", nil},
		"empty output":    {"", nil},
		"wrong first tok": {"xdeno 2.9.7\n", nil},
	}
	for name, tc := range probeCases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeFakeRuntime(t, root, "MZ fake")
			fakeRuntimeProbe(t, tc.output, tc.err)
			if _, _, err := inspectJSRuntime(root); !errors.Is(err, errJSRuntimeUnavailable) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestInspectJSRuntimeNeverUsesPATH(t *testing.T) {
	root := t.TempDir()
	onPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(onPath, jsRuntimeFileName), []byte("MZ fake"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", onPath)
	calls := fakeRuntimeProbe(t, denoVersionOutput, nil)
	if _, _, err := inspectJSRuntime(root); !errors.Is(err, errJSRuntimeUnavailable) {
		t.Fatalf("a runtime found only on PATH must not be accepted: %v", err)
	}
	if *calls != 0 {
		t.Fatal("a PATH runtime must never be executed")
	}
}

func TestRuntimeCacheProbesOnceUntilFileChanges(t *testing.T) {
	root := t.TempDir()
	path := writeFakeRuntime(t, root, "MZ fake")
	calls := fakeRuntimeProbe(t, denoVersionOutput, nil)
	var cache jsRuntimeCache
	for i := 0; i < 3; i++ {
		if _, err := cache.runtimeFor(root, sourceYouTube); err != nil {
			t.Fatal(err)
		}
	}
	if *calls != 1 {
		t.Fatalf("probe calls = %d, want 1", *calls)
	}
	if err := os.WriteFile(path, []byte("MZ fake but different size"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.runtimeFor(root, sourceYouTube); err != nil {
		t.Fatal(err)
	}
	if *calls != 2 {
		t.Fatalf("a changed runtime must be probed again, calls = %d", *calls)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.runtimeFor(root, sourceYouTube); !errors.Is(err, errJSRuntimeUnavailable) {
		t.Fatalf("a removed runtime must be reported, err = %v", err)
	}
}

func TestOnlyYouTubeRequiresTheRuntime(t *testing.T) {
	root := t.TempDir()
	calls := fakeRuntimeProbe(t, denoVersionOutput, nil)
	var cache jsRuntimeCache
	for _, source := range []mediaSource{sourceTwitchClip, sourceTwitchVOD, sourceDailymotion} {
		path, err := cache.runtimeFor(root, source)
		if path != "" || err != nil {
			t.Fatalf("%s: path=%q err=%v", source, path, err)
		}
	}
	if *calls != 0 {
		t.Fatal("Twitch and Dailymotion must not touch the runtime")
	}
	_, err := cache.runtimeFor(root, sourceYouTube)
	if !errors.Is(err, errJSRuntimeUnavailable) {
		t.Fatalf("YouTube without a runtime must fail, err = %v", err)
	}
}

func TestMissingRuntimeMessageIsActionableAndPathFree(t *testing.T) {
	root := t.TempDir()
	fakeRuntimeProbe(t, denoVersionOutput, nil)
	var cache jsRuntimeCache
	_, err := cache.runtimeFor(root, sourceYouTube)
	if err == nil {
		t.Fatal("expected an error")
	}
	message := err.Error()
	for _, want := range []string{"JavaScript runtime", "YouTube", "Reinstall FramePier", "Twitch"} {
		if !strings.Contains(message, want) {
			t.Fatalf("message %q lacks %q", message, want)
		}
	}
	if strings.Contains(message, root) || strings.Contains(message, `\`) {
		t.Fatalf("message must not expose paths: %q", message)
	}
}

func TestRuntimeStatusDistinguishesMissingFromUnusable(t *testing.T) {
	var cache jsRuntimeCache
	missing := t.TempDir()
	fakeRuntimeProbe(t, denoVersionOutput, nil)
	status := cache.status(missing)
	if status["installed"] != false || status["usable"] != false {
		t.Fatalf("missing: %v", status)
	}

	broken := t.TempDir()
	writeFakeRuntime(t, broken, "MZ fake")
	fakeRuntimeProbe(t, "", errors.New("boom"))
	var brokenCache jsRuntimeCache
	status = brokenCache.status(broken)
	if status["installed"] != true || status["usable"] != false {
		t.Fatalf("unusable: %v", status)
	}

	good := t.TempDir()
	writeFakeRuntime(t, good, "MZ fake")
	fakeRuntimeProbe(t, denoVersionOutput, nil)
	var goodCache jsRuntimeCache
	status = goodCache.status(good)
	if status["installed"] != true || status["usable"] != true || status["version"] != "2.9.7" {
		t.Fatalf("usable: %v", status)
	}
}

func TestFriendlyDiagnosticReportsRuntimeProblemsForYouTubeOnly(t *testing.T) {
	text := "WARNING: No supported JavaScript runtime could be found"
	if got := friendlyMediaDiagnostic("Download failed", text, sourceYouTube); got != msgJSRuntimeUnavailable {
		t.Fatalf("YouTube = %q", got)
	}
	if got := friendlyMediaDiagnostic("Download failed", text, sourceTwitchClip); got == msgJSRuntimeUnavailable {
		t.Fatal("Twitch errors must not be rewritten as runtime errors")
	}
	if got := friendlyMediaDiagnostic("Download failed", "HTTP Error 403", sourceYouTube); got != "" {
		t.Fatalf("unrelated YouTube error was rewritten: %q", got)
	}
}
