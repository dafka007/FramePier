package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const jsRuntimeFileName = "deno.exe"

// msgJSRuntimeUnavailable is shown to the user when YouTube is requested and
// the bundled JavaScript runtime cannot be used. It deliberately contains no
// filesystem paths.
const msgJSRuntimeUnavailable = "FramePier's built-in JavaScript runtime (Deno) is missing or not working, so YouTube downloads are unavailable. Reinstall FramePier to restore it. Twitch and Dailymotion downloads are not affected."

var errJSRuntimeUnavailable = errors.New(msgJSRuntimeUnavailable)

// ytdlpPrefixArgs returns the fixed options placed before every yt-dlp
// invocation. They keep user-level yt-dlp configuration, plugins and remotely
// fetched components out of the process and make JavaScript runtime selection
// deterministic: --no-js-runtimes clears yt-dlp's defaults (including a Deno
// found on PATH), then at most the bundled runtime is enabled by absolute path.
func ytdlpPrefixArgs(runtimePath string) []string {
	args := []string{"--ignore-config", "--no-plugin-dirs", "--no-remote-components", "--no-js-runtimes"}
	if runtimePath != "" {
		args = append(args, "--js-runtimes", "deno:"+runtimePath)
	}
	return args
}

// ytdlpExtractorFor returns the yt-dlp extractor name a source is restricted
// to, or "" when no restriction is added (YouTube and Twitch keep their
// existing command lines). --use-extractors only selects which extractor may
// handle the URL; it is not a network sandbox. yt-dlp still makes whatever
// requests that extractor and its media playlists require, and it does not
// restrict redirects, DNS resolution or secondary hosts.
func ytdlpExtractorFor(source mediaSource) string {
	if source == sourceDailymotion {
		return "dailymotion"
	}
	return ""
}

// newYtdlpCommandContext is the only constructor for yt-dlp processes that run
// extraction or downloads. runtimePath is empty when no JavaScript runtime is
// needed (for example Twitch and Dailymotion).
func newYtdlpCommandContext(ctx context.Context, ytdlpPath, runtimePath string, source mediaSource, args ...string) *exec.Cmd {
	full := ytdlpPrefixArgs(runtimePath)
	if extractor := ytdlpExtractorFor(source); extractor != "" {
		full = append(full, "--use-extractors", extractor)
	}
	full = append(full, args...)
	return newHiddenCommandContext(ctx, ytdlpPath, full...)
}

// newYtdlpCommand builds a non-extraction yt-dlp command such as -U.
func newYtdlpCommand(ytdlpPath string, args ...string) *exec.Cmd {
	full := append(ytdlpPrefixArgs(""), args...)
	return newHiddenCommand(ytdlpPath, full...)
}

// probeJSRuntimeVersion runs the bundled runtime's --version. It is a variable
// so tests can substitute a fake.
var probeJSRuntimeVersion = func(path string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := newHiddenCommandContext(ctx, path, "--version").Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

type jsRuntimeInfo struct {
	Path    string
	Version string
}

// inspectJSRuntime validates the bundled runtime at root\tools\deno.exe. Only
// that exact location is considered; PATH is never searched.
func inspectJSRuntime(root string) (jsRuntimeInfo, os.FileInfo, error) {
	candidate := filepath.Join(root, "tools", jsRuntimeFileName)
	info, err := os.Lstat(candidate)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return jsRuntimeInfo{}, nil, errJSRuntimeUnavailable
	}
	absolute, err := filepath.Abs(candidate)
	if err != nil {
		return jsRuntimeInfo{}, nil, errJSRuntimeUnavailable
	}
	output, err := probeJSRuntimeVersion(absolute)
	if err != nil {
		return jsRuntimeInfo{}, nil, errJSRuntimeUnavailable
	}
	fields := strings.Fields(firstLine(output))
	if len(fields) < 2 || fields[0] != "deno" {
		return jsRuntimeInfo{}, nil, errJSRuntimeUnavailable
	}
	return jsRuntimeInfo{Path: absolute, Version: fields[1]}, info, nil
}

// jsRuntimeCache remembers a successful inspection until the file changes, so
// the version probe does not run for every metadata request.
type jsRuntimeCache struct {
	mu   sync.Mutex
	info jsRuntimeInfo
	size int64
	mod  time.Time
}

func (c *jsRuntimeCache) verified(root string) (jsRuntimeInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.info.Path != "" {
		if stat, err := os.Lstat(c.info.Path); err == nil && stat.Mode().IsRegular() && stat.Size() == c.size && stat.ModTime().Equal(c.mod) {
			return c.info, nil
		}
		c.info = jsRuntimeInfo{}
	}
	info, stat, err := inspectJSRuntime(root)
	if err != nil {
		return jsRuntimeInfo{}, err
	}
	c.info, c.size, c.mod = info, stat.Size(), stat.ModTime()
	return info, nil
}

// runtimeFor returns the runtime path required for the media source. Only
// YouTube needs a JavaScript runtime; other sources get "" and never fail here.
func (c *jsRuntimeCache) runtimeFor(root string, source mediaSource) (string, error) {
	if source != sourceYouTube {
		return "", nil
	}
	info, err := c.verified(root)
	if err != nil {
		return "", err
	}
	return info.Path, nil
}

// status describes the runtime for the Settings page.
func (c *jsRuntimeCache) status(root string) response {
	candidate := filepath.Join(root, "tools", jsRuntimeFileName)
	_, statErr := os.Lstat(candidate)
	info, err := c.verified(root)
	return response{"installed": statErr == nil, "usable": err == nil, "version": info.Version, "name": "Deno"}
}
