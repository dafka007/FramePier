package main

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	version          = "0.2.2"
	maxMessageSize   = 1024 * 1024
	maxJobs          = 2
	maxCompletedJobs = 100
	extensionOrigin  = "chrome-extension://kclnooibijmfenaldmpkffdbednfipkk/"
)

var (
	videoIDPattern   = regexp.MustCompile(`^[A-Za-z0-9_-]{6,20}$`)
	percentPattern   = regexp.MustCompile(`(?i)([0-9]{1,3}(?:\.[0-9]+)?)%`)
	speedPattern     = regexp.MustCompile(`(?i)at\s+([^\s]+/s)`)
	etaPattern       = regexp.MustCompile(`(?i)ETA\s+([0-9:]+)`)
	totalSizePattern = regexp.MustCompile(`(?i)\bof\s+~?\s*([0-9]+(?:\.[0-9]+)?)\s*([KMGT]?i?B)\b`)
)

type request struct {
	ID             string          `json:"id"`
	Command        string          `json:"command"`
	URL            string          `json:"url,omitempty"`
	VideoID        string          `json:"videoId,omitempty"`
	Mode           string          `json:"mode,omitempty"`
	Quality        string          `json:"quality,omitempty"`
	Container      string          `json:"container,omitempty"`
	AudioFormat    string          `json:"audioFormat,omitempty"`
	DownloadPath   string          `json:"downloadPath,omitempty"`
	AutoOpen       *bool           `json:"autoOpen,omitempty"`
	RunningVersion string          `json:"runningVersion,omitempty"`
	TargetVersion  string          `json:"targetVersion,omitempty"`
	UpdateEvent    string          `json:"updateEvent,omitempty"`
	Browser        string          `json:"browser,omitempty"`
	Extra          json.RawMessage `json:"-"`
}

type response map[string]any

type settings struct {
	DownloadPath string `json:"downloadPath"`
	AutoOpen     bool   `json:"autoOpen"`
}

type job struct {
	ID              string  `json:"id"`
	State           string  `json:"state"`
	Percent         float64 `json:"percent"`
	Speed           string  `json:"speed,omitempty"`
	ETA             string  `json:"eta,omitempty"`
	DownloadedBytes int64   `json:"downloadedBytes,omitempty"`
	TotalBytes      int64   `json:"totalBytes,omitempty"`
	OutputPath      string  `json:"outputPath,omitempty"`
	Error           string  `json:"error,omitempty"`
	cancelRequested bool
	cancel          context.CancelFunc
	processJob      *processJob
	source          mediaSource
	completedAt     time.Time
}

type app struct {
	mu          sync.RWMutex
	writeMu     sync.Mutex
	jobs        map[string]*job
	jobsWG      sync.WaitGroup
	settings    settings
	root        string
	dataDir     string
	logger      *log.Logger
	out         io.Writer
	runtimePath string
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--lifecycle-status":
			if len(os.Args) != 3 {
				os.Exit(1)
			}
			status, err := lifecycleStatus(os.Args[2])
			if err != nil {
				_, _ = fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			os.Exit(status)
		case "--lifecycle-stop":
			if len(os.Args) != 3 {
				os.Exit(1)
			}
			if err := stopInstalledHelpers(os.Args[2]); err != nil {
				_, _ = fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			return
		case "--lifecycle-resume":
			if err := resumeInstalledHelpers(); err != nil {
				_, _ = fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			return
		}
	}
	root, err := executableDir()
	if err != nil {
		fatalNative(err)
	}
	if len(os.Args) > 1 && (os.Args[1] == "--register" || os.Args[1] == "--repair" || os.Args[1] == "--repair-ui" || os.Args[1] == "--open-browser-setup" || os.Args[1] == "--unregister") {
		if os.Args[1] == "--open-browser-setup" {
			openBrowserSetup(root)
			return
		}
		install := os.Args[1] != "--unregister"
		if err := manageRegistration(root, install); err != nil {
			if os.Args[1] == "--repair-ui" {
				showInformation("VidDock Browser Integration", "Repair failed: "+err.Error())
			}
			_, _ = fmt.Fprintln(os.Stderr, "VidDock registration failed:", err)
			os.Exit(1)
		}
		if os.Args[1] == "--repair-ui" {
			showInformation("VidDock Browser Integration", browserIntegrationSummary(root))
			openBrowserSetup(root)
		}
		return
	}
	if originArgument() != extensionOrigin && originArgument() != "stdio-test" {
		fatalNative(errors.New("unauthorized extension origin"))
	}
	shutdownGate, err := openShutdownEvent()
	if err != nil {
		fatalNative(err)
	}
	defer shutdownGate.close()
	if shutdownGate.signaled() || lifecycleGateActive() {
		return
	}
	dataDir := filepath.Join(localAppData(), "VidDock")
	if err := os.MkdirAll(filepath.Join(dataDir, "logs"), 0700); err != nil {
		fatalNative(err)
	}
	logPath := filepath.Join(dataDir, "logs", "viddock-helper.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		fatalNative(err)
	}
	defer logFile.Close()

	a := &app{jobs: make(map[string]*job), root: root, dataDir: dataDir, logger: log.New(logFile, "", log.LstdFlags|log.LUTC), out: os.Stdout, runtimePath: runtimeStatePath(os.Getpid())}
	a.settings = a.loadSettings()
	a.writeRuntimeState()
	defer os.Remove(a.runtimePath)
	go func() {
		if shutdownGate.wait() {
			_ = os.Stdin.Close()
		}
	}()
	// Dependency versions are queried by the versions command, not on every
	// Native Messaging connection before its first request can be answered.
	a.logger.Printf("helper_started version=%s origin=%q", version, originArgument())
	if err := a.run(os.Stdin); !errors.Is(err, io.EOF) {
		a.logger.Printf("helper_stopped error=%q", err)
	}
	a.shutdown()
}

func (a *app) run(input io.Reader) error {
	reader := bufio.NewReader(input)
	for {
		payload, err := readNativeMessage(reader)
		if err != nil {
			return err
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(payload, &raw); err != nil {
			a.send(response{"ok": false, "error": "Malformed JSON message."})
			continue
		}
		req, err := decodeRequest(payload, raw)
		if err != nil {
			a.send(response{"ok": false, "error": err.Error()})
			continue
		}
		a.handle(req)
	}
}

func readNativeMessage(r io.Reader) ([]byte, error) {
	var size uint32
	if err := binary.Read(r, binary.LittleEndian, &size); err != nil {
		return nil, err
	}
	if size == 0 || size > maxMessageSize {
		return nil, fmt.Errorf("native message size %d is not allowed", size)
	}
	payload := make([]byte, size)
	_, err := io.ReadFull(r, payload)
	return payload, err
}

func decodeRequest(payload []byte, raw map[string]json.RawMessage) (request, error) {
	allowed := map[string]bool{"id": true, "command": true, "url": true, "videoId": true, "mode": true, "quality": true, "container": true, "audioFormat": true, "downloadPath": true, "autoOpen": true, "runningVersion": true, "targetVersion": true, "updateEvent": true, "browser": true}
	for key := range raw {
		if !allowed[key] {
			return request{}, fmt.Errorf("unknown field: %s", key)
		}
	}
	var req request
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		return request{}, errors.New("message fields have invalid types")
	}
	if len(req.ID) > 80 || !regexp.MustCompile(`^[A-Za-z0-9_-]{1,80}$`).MatchString(req.ID) {
		return request{}, errors.New("invalid request id")
	}
	commands := map[string]bool{"ping": true, "get_video_info": true, "get_formats": true, "start_download": true, "cancel_download": true, "get_status": true, "check_dependencies": true, "get_versions": true, "choose_download_folder": true, "open_download_folder": true, "open_logs_folder": true, "update_ytdlp": true, "get_settings": true, "set_settings": true, "record_update_event": true}
	if !commands[req.Command] {
		return request{}, errors.New("unknown command")
	}
	fieldsByCommand := map[string]map[string]bool{
		"get_video_info":      {"url": true, "videoId": true},
		"get_formats":         {"url": true, "videoId": true},
		"start_download":      {"url": true, "videoId": true, "mode": true, "quality": true, "container": true, "audioFormat": true},
		"set_settings":        {"downloadPath": true, "autoOpen": true},
		"record_update_event": {"runningVersion": true, "targetVersion": true, "updateEvent": true, "browser": true},
	}
	commandFields := fieldsByCommand[req.Command]
	for key := range raw {
		if key != "id" && key != "command" && !commandFields[key] {
			return request{}, fmt.Errorf("field %s is not allowed for command %s", key, req.Command)
		}
	}
	return req, nil
}

func (a *app) handle(req request) {
	base := response{"id": req.ID, "command": req.Command}
	switch req.Command {
	case "ping":
		base["ok"] = true
		base["version"] = version
		base["platform"] = runtime.GOOS
		if packageVersion := installedPackageVersion(a.root); packageVersion != "" {
			base["installedPackageVersion"] = packageVersion
			// Kept for the 0.1.5 worker during its one-time transition to 0.1.6.
			base["installedExtensionVersion"] = packageVersion
		}
		a.send(base)
	case "get_video_info", "get_formats":
		go a.videoInfo(req, base)
	case "start_download":
		a.startDownload(req, base)
	case "cancel_download":
		a.cancelDownload(req, base)
	case "get_status":
		a.getStatus(req, base)
	case "check_dependencies", "get_versions":
		base["ok"] = true
		base["versions"] = a.versions()
		a.send(base)
	case "choose_download_folder":
		owner := foregroundChromiumWindow()
		go a.chooseDownloadFolder(base, owner)
	case "record_update_event":
		a.recordUpdateEvent(req, base)
	case "open_download_folder":
		a.mu.RLock()
		folder := a.settings.DownloadPath
		a.mu.RUnlock()
		a.openFixedFolder(folder, base)
	case "open_logs_folder":
		a.openFixedFolder(filepath.Join(a.dataDir, "logs"), base)
	case "update_ytdlp":
		go a.updateYTDLP(base)
	case "get_settings":
		a.mu.RLock()
		current := a.settings
		a.mu.RUnlock()
		base["ok"] = true
		base["settings"] = current
		a.send(base)
	case "set_settings":
		a.setSettings(req, base)
	}
}

func (a *app) recordUpdateEvent(req request, base response) {
	if !validUpdateEvent(req) {
		base["ok"], base["error"] = false, "Invalid update diagnostic event."
		a.send(base)
		return
	}
	a.logger.Printf("extension_update event=%s browser=%s running=%s target=%s", req.UpdateEvent, req.Browser, req.RunningVersion, req.TargetVersion)
	base["ok"] = true
	a.send(base)
}

func validUpdateEvent(req request) bool {
	validEvent := req.UpdateEvent == "mismatch_detected" || req.UpdateEvent == "reload_requested" || req.UpdateEvent == "activated" || req.UpdateEvent == "reload_guarded"
	validBrowser := req.Browser == "Brave" || req.Browser == "Chrome" || req.Browser == "Edge" || req.Browser == "Chromium"
	return validEvent && validBrowser && packageVersionPattern.MatchString(req.RunningVersion) && packageVersionPattern.MatchString(req.TargetVersion)
}

func (a *app) videoInfo(req request, base response) {
	target, err := validateMediaURL(req.URL, req.VideoID)
	if err != nil {
		base["ok"], base["error"] = false, err.Error()
		a.send(base)
		return
	}
	ytdlp, err := a.toolPath("yt-dlp.exe")
	if err != nil {
		base["ok"], base["error"] = false, err.Error()
		a.send(base)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := newHiddenCommandContext(ctx, ytdlp, "--dump-single-json", "--no-playlist", "--no-warnings", "--skip-download", target.CanonicalURL)
	cmd.Dir = a.root
	output, err := cmd.Output()
	if err != nil {
		base["ok"], base["error"] = false, friendlyMediaError("Could not retrieve video information", err, target.Source)
		a.logger.Printf("video_info_failed source=%s media_id=%s error=%q diagnostic=%q", target.Source, target.ID, err, exitErrorDiagnostic(err, target.CanonicalURL))
		a.send(base)
		return
	}
	var info struct {
		ID         string        `json:"id"`
		DisplayID  string        `json:"display_id"`
		Title      string        `json:"title"`
		Uploader   string        `json:"uploader"`
		Channel    string        `json:"channel"`
		Creator    string        `json:"creator"`
		Duration   float64       `json:"duration"`
		Thumbnail  string        `json:"thumbnail"`
		UploadDate string        `json:"upload_date"`
		Formats    []mediaFormat `json:"formats"`
	}
	if err := json.Unmarshal(output, &info); err != nil || !metadataMatchesTarget(info.ID, info.DisplayID, target) {
		base["ok"], base["error"] = false, "The downloader returned invalid metadata."
		a.send(base)
		return
	}
	info.Formats = normalizeFormats(info.Formats)
	audioExt := map[string]bool{}
	var approx int64
	for _, f := range info.Formats {
		if f.ACodec != "" && f.ACodec != "none" && f.VCodec == "none" {
			if f.Ext == "m4a" || f.Ext == "mp4" || strings.HasPrefix(strings.ToLower(f.ACodec), "mp4a") || strings.HasPrefix(strings.ToLower(f.ACodec), "aac") {
				audioExt["m4a"] = true
			} else if f.Ext == "webm" {
				audioExt["webm"] = true
			}
		}
		if f.Filesize > approx {
			approx = f.Filesize
		} else if f.ApproxSize > approx {
			approx = f.ApproxSize
		}
	}
	q := availableQualitiesForDuration(info.Formats, info.Duration)
	channel := info.Channel
	if channel == "" {
		channel = info.Uploader
	}
	base["ok"] = true
	for _, quality := range q {
		if quality.ApproxBytes > approx {
			approx = quality.ApproxBytes
		}
	}
	base["video"] = response{"id": target.ID, "title": info.Title, "channel": channel, "creator": info.Creator, "duration": info.Duration, "thumbnail": info.Thumbnail, "uploadDate": info.UploadDate, "url": target.CanonicalURL, "source": string(target.Source), "sourceLabel": sourceLabel(target.Source), "qualities": q, "audioFormats": audioExt, "approxBytes": approx}
	a.send(base)
}

func (a *app) startDownload(req request, base response) {
	target, err := validateMediaURL(req.URL, req.VideoID)
	if err != nil {
		base["ok"], base["error"] = false, err.Error()
		a.send(base)
		return
	}
	if req.Mode != "video" && req.Mode != "audio" {
		base["ok"], base["error"] = false, "Invalid download mode."
		a.send(base)
		return
	}
	if !validQuality(req.Quality) || !validChoice(req.Container, []string{"mp4", "mkv"}) || !validChoice(req.AudioFormat, []string{"best", "m4a", "mp3"}) {
		base["ok"], base["error"] = false, "Invalid format selection."
		a.send(base)
		return
	}
	a.mu.Lock()
	if _, exists := a.jobs[req.ID]; exists {
		a.mu.Unlock()
		base["ok"], base["error"] = false, "Download job ID is already in use."
		a.send(base)
		return
	}
	active := 0
	for _, existing := range a.jobs {
		if existing.State != "finished" && existing.State != "failed" && existing.State != "cancelled" {
			active++
		}
	}
	if active >= maxJobs {
		a.mu.Unlock()
		base["ok"], base["error"] = false, "VidDock is already running the maximum number of downloads."
		a.send(base)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	j := &job{ID: req.ID, State: "preparing", cancel: cancel, source: target.Source}
	a.jobs[req.ID] = j
	a.mu.Unlock()
	a.writeRuntimeState()

	ytdlp, err := a.toolPath("yt-dlp.exe")
	if err != nil {
		cancel()
		a.finishJob(j, "failed", "", err.Error())
		base["ok"], base["error"] = false, err.Error()
		a.send(base)
		return
	}
	if req.Mode == "video" || (req.Mode == "audio" && (req.AudioFormat != "best" || target.Source == sourceTwitchClip)) {
		if _, err := a.toolPath("ffmpeg.exe"); err != nil {
			cancel()
			a.finishJob(j, "failed", "", "ffmpeg is required for this format.")
			base["ok"], base["error"] = false, "ffmpeg is required for this format."
			a.send(base)
			return
		}
	}
	base["ok"], base["jobId"] = true, req.ID
	a.send(base)
	a.jobsWG.Add(1)
	go func() {
		defer a.jobsWG.Done()
		a.runDownload(ctx, j, ytdlp, target, req)
	}()
}

func (a *app) runDownload(ctx context.Context, j *job, ytdlp string, target mediaTarget, req request) {
	defer j.cancel()
	a.mu.RLock()
	currentSettings := a.settings
	a.mu.RUnlock()
	if err := os.MkdirAll(currentSettings.DownloadPath, 0700); err != nil {
		a.finishJob(j, "failed", "", "Could not create the download folder.")
		return
	}
	tempRoot := filepath.Join(a.dataDir, "temp")
	if err := os.MkdirAll(tempRoot, 0700); err != nil {
		a.finishJob(j, "failed", "", "Could not create a private temporary folder.")
		return
	}
	// Separate browser connections can reuse request IDs. Never share their
	// partial files or cleanup directory.
	jobTemp, err := os.MkdirTemp(tempRoot, "job-")
	if err != nil {
		a.finishJob(j, "failed", "", "Could not create a private temporary folder.")
		return
	}
	cleaned := false
	cleanupTemp := func() {
		if cleaned {
			return
		}
		var lastErr error
		for attempt := 0; attempt < 50; attempt++ {
			if err := removePrivateTree(jobTemp, 0); err == nil {
				cleaned = true
				return
			} else {
				lastErr = err
			}
			time.Sleep(200 * time.Millisecond)
		}
		a.logger.Printf("temp_cleanup_failed job=%s error=%q", j.ID, lastErr)
	}
	defer cleanupTemp()
	var sourceMetadata videoMetadata
	metadataLoaded := false
	if target.Source == sourceTwitchVOD {
		a.mu.Lock()
		j.State = "fetching_formats"
		snapshot := *j
		a.mu.Unlock()
		a.send(response{"event": "progress", "job": snapshot})
		var metadataErr error
		sourceMetadata, metadataErr = loadVideoMetadata(ctx, ytdlp, a.root, target.CanonicalURL)
		if metadataErr != nil {
			a.finishJob(j, "failed", "", friendlyMediaError("Could not retrieve Twitch VOD formats", metadataErr, target.Source))
			return
		}
		metadataLoaded = true
		if err := ensureEstimatedDiskSpace(currentSettings.DownloadPath, jobTemp, estimatedDownloadBytes(sourceMetadata, req.Quality, req.Mode)); err != nil {
			a.finishJob(j, "failed", "", err.Error())
			return
		}
	}
	outputTemplate := "%(title).150B [" + target.ID + "] [audio].%(ext)s"
	if req.Mode == "video" {
		outputTemplate = "%(title).130B [" + target.ID + "] [%(height)sp].%(ext)s"
	}
	args := []string{"--no-playlist", "--newline", "--progress", "--windows-filenames", "--trim-filenames", "180", "--no-overwrites", "--paths", currentSettings.DownloadPath, "--paths", "temp:" + jobTemp, "--output", outputTemplate, "--print", "post_process:viddock_state::finalizing", "--print", "after_move:viddock_path::%(filepath)s"}
	if ffmpegPath, err := a.toolPath("ffmpeg.exe"); err == nil {
		args = append(args, "--ffmpeg-location", filepath.Dir(ffmpegPath))
	}
	if req.Mode == "audio" {
		switch req.AudioFormat {
		case "mp3":
			if target.Source == sourceTwitchClip {
				args = append(args, "--format", "best")
			}
			args = append(args, "--extract-audio", "--audio-format", "mp3", "--audio-quality", "0")
		case "m4a":
			if target.Source == sourceTwitchClip {
				args = append(args, "--format", "best")
			} else if target.Source == sourceYouTube {
				args = append(args, "--format", "bestaudio[ext=m4a]/bestaudio")
			} else {
				args = append(args, "--format", "bestaudio")
			}
			args = append(args, "--extract-audio", "--audio-format", "m4a")
		default:
			if target.Source == sourceTwitchClip {
				args = append(args, "--format", "best", "--extract-audio", "--audio-format", "m4a")
			} else {
				args = append(args, "--format", "bestaudio")
			}
		}
	} else {
		selector := "bestvideo*+bestaudio/best"
		if target.Source != sourceYouTube && req.Quality == "best" {
			selector = "best"
		} else if req.Container == "mp4" && req.Quality == "best" {
			selector = "bestvideo*[ext=mp4][vcodec^=avc1]+bestaudio[ext=m4a]/bestvideo*[ext=mp4][vcodec^=av01]+bestaudio[ext=m4a]/bestvideo*[ext=mp4][vcodec^=hvc1]+bestaudio[ext=m4a]/bestvideo*[ext=mp4][vcodec^=hev1]+bestaudio[ext=m4a]/best[ext=mp4]/best"
		} else if req.Quality != "best" {
			metadata := sourceMetadata
			var err error
			if !metadataLoaded {
				metadata, err = loadVideoMetadata(ctx, ytdlp, a.root, target.CanonicalURL)
			}
			if err != nil {
				a.finishJob(j, "failed", "", friendlyMediaError("Could not retrieve formats for the selected quality", err, target.Source))
				return
			}
			requestedWidth, requestedHeight, requestedFPS, exactResolution := parseResolutionAndFPS(req.Quality)
			var selected mediaFormat
			if exactResolution {
				selected, err = selectVideoFormatByResolutionAndFPS(metadata.Formats, requestedWidth, requestedHeight, requestedFPS, req.Container)
			} else {
				requestedTier, _ := strconv.Atoi(req.Quality)
				selected, err = selectVideoFormat(metadata.Formats, requestedTier, req.Container)
			}
			if err != nil {
				a.finishJob(j, "failed", "", err.Error()+".")
				return
			}
			selectedAudio := mediaFormat{}
			if target.Source != sourceTwitchClip && (selected.ACodec == "" || selected.ACodec == "none") {
				selectedAudio, err = selectAudioFormat(metadata.Formats, req.Container)
				if err != nil {
					a.finishJob(j, "failed", "", err.Error()+".")
					return
				}
			}
			selector, err = selectedFormatExpressionForSource(selected, selectedAudio, target.Source)
			if err != nil {
				a.finishJob(j, "failed", "", "The selected source format was invalid.")
				return
			}
			a.logger.Printf("format_selected job=%s requested=%s video_format_id=%s audio_format_id=%s dimensions=%dx%d ext=%s vcodec=%s acodec=%s", j.ID, req.Quality, selected.ID, selectedAudio.ID, selected.Width, selected.Height, selected.Ext, selected.VCodec, selected.ACodec)
		}
		args = append(args, "--format", selector)
		if req.Container == "mp4" {
			args = append(args, "--merge-output-format", "mp4", "--remux-video", "mp4")
		} else {
			args = append(args, "--merge-output-format", "mkv", "--remux-video", "mkv")
		}
	}
	args = append(args, target.CanonicalURL)
	cmd := newHiddenCommandContext(ctx, ytdlp, args...)
	cmd.Dir = a.root
	cmd.Env = append(os.Environ(), "PYTHONUTF8=1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		a.finishJob(j, "failed", "", "Could not start the downloader.")
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		a.finishJob(j, "failed", "", "Could not start the downloader.")
		return
	}
	processStartedAt := time.Now()
	if err := cmd.Start(); err != nil {
		a.finishJob(j, "failed", "", "Could not start the downloader.")
		return
	}
	ownedJob, err := attachKillOnCloseJob(cmd.Process.Pid)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		a.logger.Printf("download_job_setup_failed job=%s error=%q", j.ID, err)
		a.finishJob(j, "failed", "", "Could not safely manage the downloader process.")
		return
	}
	a.mu.Lock()
	j.processJob = ownedJob
	cancelledDuringStart := j.cancelRequested
	a.mu.Unlock()
	defer ownedJob.close()
	if cancelledDuringStart || ctx.Err() != nil {
		ownedJob.close()
	}
	a.logger.Printf("download_started job=%s source=%s media_id=%s mode=%s quality=%s", j.ID, target.Source, target.ID, req.Mode, req.Quality)
	var outputPath string
	var diagnostics []string
	var scanMu sync.Mutex
	var scanners sync.WaitGroup
	scan := func(reader io.Reader) {
		defer scanners.Done()
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			scanMu.Lock()
			if line != "" {
				diagnostics = append(diagnostics, safeDiagnosticLine(line, target.CanonicalURL, currentSettings.DownloadPath))
				if len(diagnostics) > 20 {
					diagnostics = diagnostics[1:]
				}
			}
			if strings.HasPrefix(line, "viddock_path::") {
				outputPath = strings.TrimPrefix(line, "viddock_path::")
				scanMu.Unlock()
				continue
			}
			if strings.HasPrefix(line, "viddock_state::") {
				state := strings.TrimPrefix(line, "viddock_state::")
				scanMu.Unlock()
				a.mu.Lock()
				j.State = state
				snapshot := *j
				a.mu.Unlock()
				a.send(response{"event": "progress", "job": snapshot})
				continue
			}
			scanMu.Unlock()
			a.parseProgress(j, line)
		}
	}
	scanners.Add(2)
	go scan(stdout)
	go scan(stderr)
	err = cmd.Wait()
	scanners.Wait()
	a.mu.RLock()
	cancelRequested := j.cancelRequested
	a.mu.RUnlock()
	if cancelRequested || ctx.Err() == context.Canceled {
		ownedJob.close()
		cleanupTemp()
		a.finishJob(j, "cancelled", "", "")
		return
	}
	if err != nil {
		a.logger.Printf("download_process_failed job=%s error=%q diagnostic=%q", j.ID, err, strings.Join(diagnostics, " | "))
		friendly := friendlyMediaDiagnostic("Download failed", strings.Join(diagnostics, " "), target.Source)
		if friendly == "" {
			friendly = friendlyMediaError("Download failed", err, target.Source)
		}
		a.finishJob(j, "failed", "", friendly)
		return
	}
	verified, err := verifyOutput(currentSettings.DownloadPath, outputPath)
	if err != nil {
		fallbackPath, discoveryErr := findRecentOutput(currentSettings.DownloadPath, target.ID, processStartedAt)
		if discoveryErr == nil {
			outputPath = fallbackPath
			verified, err = verifyOutput(currentSettings.DownloadPath, outputPath)
		} else {
			a.logger.Printf("output_discovery_failed job=%s error=%q", j.ID, discoveryErr)
		}
	}
	if err != nil {
		a.logger.Printf("output_verification_failed job=%s candidate=%q error=%q", j.ID, filepath.Base(outputPath), err)
		a.finishJob(j, "failed", "", "The downloader exited, but the final output file could not be verified.")
		return
	}
	a.finishJob(j, "finished", verified, "")
	if currentSettings.AutoOpen {
		_ = a.launchFixedFolder(currentSettings.DownloadPath, "auto_open_download_folder")
	}
}

func removePrivateTree(path string, depth int) error {
	if depth > 32 {
		return errors.New("temporary directory nesting is too deep")
	}
	entries, err := os.ReadDir(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		child := filepath.Join(path, entry.Name())
		info, err := os.Lstat(child)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if err := os.Remove(child); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			continue
		}
		if info.IsDir() {
			if err := removePrivateTree(child, depth+1); err != nil {
				return err
			}
		} else if err := os.Remove(child); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (a *app) parseProgress(j *job, line string) {
	state := ""
	lower := strings.ToLower(line)
	if strings.Contains(lower, "merging formats") || strings.Contains(lower, "merger") {
		state = "merging"
	} else if strings.Contains(lower, "extractaudio") || strings.Contains(lower, "converting audio") {
		state = "converting_audio"
	} else if strings.Contains(lower, "destination") || strings.HasPrefix(lower, "[download]") {
		state = "downloading"
	}
	if state == "" {
		return
	}
	a.mu.Lock()
	j.State = state
	if m := percentPattern.FindStringSubmatch(line); len(m) == 2 {
		if p, err := strconv.ParseFloat(m[1], 64); err == nil && p >= 0 && p <= 100 {
			if j.source != sourceTwitchVOD || p >= j.Percent {
				j.Percent = p
			}
		}
	}
	if m := speedPattern.FindStringSubmatch(line); len(m) == 2 {
		j.Speed = m[1]
	}
	if m := etaPattern.FindStringSubmatch(line); len(m) == 2 {
		j.ETA = m[1]
	}
	if m := totalSizePattern.FindStringSubmatch(line); len(m) == 3 {
		if total, ok := parseByteSize(m[1], m[2]); ok {
			if j.source != sourceTwitchVOD || total > j.TotalBytes {
				j.TotalBytes = total
			}
			j.DownloadedBytes = int64(float64(j.TotalBytes) * j.Percent / 100)
		}
	}
	snapshot := *j
	a.mu.Unlock()
	a.send(response{"event": "progress", "job": snapshot})
}

func parseByteSize(number, unit string) (int64, bool) {
	value, err := strconv.ParseFloat(number, 64)
	if err != nil || value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	multipliers := map[string]float64{"b": 1, "kb": 1000, "kib": 1024, "mb": 1000 * 1000, "mib": 1024 * 1024, "gb": 1000 * 1000 * 1000, "gib": 1024 * 1024 * 1024, "tb": 1000 * 1000 * 1000 * 1000, "tib": 1024 * 1024 * 1024 * 1024}
	multiplier, ok := multipliers[strings.ToLower(unit)]
	bytes := value * multiplier
	if !ok || bytes < 0 || bytes >= float64(math.MaxInt64) {
		return 0, false
	}
	return int64(bytes), true
}

func (a *app) finishJob(j *job, state, outputPath, errorMessage string) {
	a.mu.Lock()
	if j.cancelRequested && state == "failed" {
		state, errorMessage = "cancelled", ""
	}
	j.State, j.OutputPath, j.Error = state, outputPath, errorMessage
	j.completedAt = time.Now()
	a.pruneCompletedJobs()
	if state == "finished" {
		j.Percent = 100
	}
	snapshot := *j
	a.mu.Unlock()
	a.writeRuntimeState()
	a.logger.Printf("download_finished job=%s state=%s output=%q error=%q", j.ID, state, filepath.Base(outputPath), errorMessage)
	a.send(response{"event": "progress", "job": snapshot})
}

// Called with a.mu held. Keep recent status queries useful without retaining
// every completed job for the lifetime of a browser connection.
func (a *app) pruneCompletedJobs() {
	for {
		count := 0
		var oldest *job
		for _, candidate := range a.jobs {
			if candidate.completedAt.IsZero() {
				continue
			}
			count++
			if oldest == nil || candidate.completedAt.Before(oldest.completedAt) || (candidate.completedAt.Equal(oldest.completedAt) && candidate.ID < oldest.ID) {
				oldest = candidate
			}
		}
		if count <= maxCompletedJobs {
			return
		}
		delete(a.jobs, oldest.ID)
	}
}

func (a *app) cancelDownload(req request, base response) {
	a.mu.Lock()
	j, ok := a.jobs[req.ID]
	var ownedJob *processJob
	var cancel context.CancelFunc
	if ok && j.State != "finished" && j.State != "failed" && j.State != "cancelled" {
		j.cancelRequested = true
		ownedJob, cancel = j.processJob, j.cancel
	}
	a.mu.Unlock()
	if !ok {
		base["ok"], base["error"] = false, "Download job was not found."
		a.send(base)
		return
	}
	if ownedJob != nil {
		ownedJob.close()
	}
	if cancel != nil {
		cancel()
	}
	base["ok"] = true
	a.send(base)
}

func (a *app) shutdown() {
	type activeJob struct {
		cancel     context.CancelFunc
		processJob *processJob
	}
	a.mu.RLock()
	active := make([]activeJob, 0, len(a.jobs))
	for _, j := range a.jobs {
		if j.State != "finished" && j.State != "failed" && j.State != "cancelled" {
			active = append(active, activeJob{cancel: j.cancel, processJob: j.processJob})
		}
	}
	a.mu.RUnlock()
	for _, item := range active {
		if item.processJob != nil {
			item.processJob.close()
		}
		item.cancel()
	}
	done := make(chan struct{})
	go func() {
		a.jobsWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(8 * time.Second):
		a.logger.Printf("shutdown_timeout active_jobs=%d", len(active))
	}
}

func (a *app) writeRuntimeState() {
	a.mu.RLock()
	active := 0
	for _, j := range a.jobs {
		if j.State != "finished" && j.State != "failed" && j.State != "cancelled" {
			active++
		}
	}
	a.mu.RUnlock()
	_ = os.MkdirAll(runtimeDirectory(), 0700)
	data, _ := json.Marshal(runtimeState{PID: os.Getpid(), ActiveJobs: active})
	temp := a.runtimePath + ".tmp"
	if os.WriteFile(temp, data, 0600) == nil {
		_ = os.Rename(temp, a.runtimePath)
	}
}

func (a *app) getStatus(req request, base response) {
	a.mu.RLock()
	j, ok := a.jobs[req.ID]
	if ok {
		snapshot := *j
		base["job"] = snapshot
	}
	a.mu.RUnlock()
	base["ok"] = ok
	if !ok {
		base["error"] = "Download job was not found."
	}
	a.send(base)
}

func (a *app) versions() response {
	ytdlpPath, ytdlpErr := a.toolPath("yt-dlp.exe")
	ffmpegPath, ffmpegErr := a.toolPath("ffmpeg.exe")
	result := response{
		"helper": response{"installed": true, "version": version},
		"ytDlp":  response{"installed": ytdlpErr == nil, "version": commandVersion(ytdlpPath, "--version")},
		"ffmpeg": response{"installed": ffmpegErr == nil, "version": firstLine(commandVersion(ffmpegPath, "-version"))},
	}
	if packageVersion := installedPackageVersion(a.root); packageVersion != "" {
		result["extensionPackage"] = response{"installed": true, "version": packageVersion}
	}
	return result
}

func (a *app) updateYTDLP(base response) {
	path, err := a.toolPath("yt-dlp.exe")
	if err == nil {
		cmd := newHiddenCommand(path, "-U")
		cmd.Dir = filepath.Dir(path)
		var out []byte
		out, err = cmd.CombinedOutput()
		if err != nil {
			a.logger.Printf("ytdlp_update_failed error=%q output=%q", err, truncate(string(out), 1000))
		}
	}
	base["ok"] = err == nil
	if err != nil {
		base["error"] = "yt-dlp could not be updated. Check the log for details."
	} else {
		a.logger.Printf("ytdlp_update_succeeded version=%q", commandVersion(path, "--version"))
		base["versions"] = a.versions()
	}
	a.send(base)
}

func (a *app) setSettings(req request, base response) {
	path, err := prepareDownloadPath(req.DownloadPath)
	if err != nil {
		base["ok"], base["error"] = false, err.Error()
		a.send(base)
		return
	}
	if req.AutoOpen == nil {
		base["ok"], base["error"] = false, "autoOpen must be a boolean."
		a.send(base)
		return
	}
	newSettings := settings{DownloadPath: path, AutoOpen: *req.AutoOpen}
	data, _ := json.MarshalIndent(newSettings, "", "  ")
	if err := os.MkdirAll(filepath.Dir(a.settingsPath()), 0700); err != nil {
		base["ok"], base["error"] = false, "Could not save settings."
		a.send(base)
		return
	}
	if err := writeSettingsAtomically(a.settingsPath(), data); err != nil {
		base["ok"], base["error"] = false, "Could not save settings."
		a.send(base)
		return
	}
	a.mu.Lock()
	a.settings = newSettings
	a.mu.Unlock()
	base["ok"], base["settings"] = true, newSettings
	a.send(base)
}

func (a *app) chooseDownloadFolder(base response, owner uintptr) {
	a.mu.RLock()
	currentPath := a.settings.DownloadPath
	a.mu.RUnlock()
	a.logger.Printf("folder_picker_started browser_owner=%t", owner != 0)
	selectedPath, cancelled, err := chooseWindowsFolder(currentPath, owner)
	if cancelled {
		a.logger.Printf("folder_picker_cancelled browser_owner=%t", owner != 0)
		base["ok"], base["cancelled"] = true, true
		a.send(base)
		return
	}
	if err != nil {
		a.logger.Printf("folder_picker_failed error=%q", err)
		base["ok"], base["error"] = false, "The Windows folder picker could not be opened."
		a.send(base)
		return
	}
	selectedPath, err = prepareDownloadPath(selectedPath)
	if err != nil {
		base["ok"], base["error"] = false, err.Error()
		a.send(base)
		return
	}
	base["ok"], base["cancelled"], base["path"] = true, false, selectedPath
	a.send(base)
}

func (a *app) loadSettings() settings {
	fallback := settings{DownloadPath: filepath.Join(userDownloads(), "VidDock"), AutoOpen: false}
	data, err := os.ReadFile(a.settingsPath())
	if err != nil {
		return fallback
	}
	var loaded settings
	if json.Unmarshal(data, &loaded) != nil {
		return fallback
	}
	if path, err := validateDownloadPath(loaded.DownloadPath); err == nil {
		loaded.DownloadPath = path
		return loaded
	}
	return fallback
}

func (a *app) settingsPath() string { return filepath.Join(a.dataDir, "settings.json") }

func (a *app) openFixedFolder(path string, base response) {
	if err := a.launchFixedFolder(path, fmt.Sprint(base["command"])); err != nil {
		message := "VidDock couldn't open the download folder. Check the log for details."
		if base["command"] == "open_logs_folder" {
			message = "VidDock couldn't open the logs folder."
		}
		base["ok"], base["error"] = false, message
	} else {
		base["ok"] = true
	}
	a.send(base)
}

func (a *app) toolPath(name string) (string, error) {
	local := filepath.Join(a.root, "tools", name)
	if info, err := os.Stat(local); err == nil && !info.IsDir() {
		return local, nil
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%s is not installed", strings.TrimSuffix(name, ".exe"))
	}
	return filepath.Clean(path), nil
}

func (a *app) send(message response) {
	data, err := json.Marshal(message)
	if err != nil || len(data) > maxMessageSize {
		return
	}
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	_ = binary.Write(a.out, binary.LittleEndian, uint32(len(data)))
	_, _ = a.out.Write(data)
}

func commandVersion(path string, arg string) string {
	if path == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := newHiddenCommandContext(ctx, path, arg).CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func friendlyProcessError(prefix string, err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return prefix + ": operation timed out."
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return prefix + ". The video may be unavailable, private, or unsupported."
	}
	return prefix + "."
}

func metadataMatchesTarget(id, displayID string, target mediaTarget) bool {
	switch target.Source {
	case sourceTwitchClip:
		return displayID == target.ID
	case sourceTwitchVOD:
		return id == target.ID || id == "v"+target.ID || displayID == target.ID || displayID == "v"+target.ID
	default:
		return id == target.ID
	}
}

func friendlyMediaError(prefix string, err error, source mediaSource) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return prefix + ": operation timed out."
	}
	diagnostic := exitErrorDiagnostic(err, "")
	if friendly := friendlyMediaDiagnostic(prefix, diagnostic, source); friendly != "" {
		return friendly
	}
	return friendlyProcessError(prefix, err)
}

func friendlyMediaDiagnostic(prefix, diagnostic string, source mediaSource) string {
	diagnostic = strings.ToLower(diagnostic)
	if source == sourceTwitchClip || source == sourceTwitchVOD {
		if strings.Contains(diagnostic, "subscriber") || strings.Contains(diagnostic, "authentication") || strings.Contains(diagnostic, "log in") || strings.Contains(diagnostic, "login") {
			return "This Twitch video is not publicly available without authentication."
		}
		if strings.Contains(diagnostic, "no longer available") || strings.Contains(diagnostic, "not found") || strings.Contains(diagnostic, "deleted") {
			if source == sourceTwitchClip {
				return "This Twitch clip is no longer available."
			}
			return "This Twitch VOD is no longer available."
		}
		if strings.Contains(diagnostic, "no video formats") || strings.Contains(diagnostic, "requested format is not available") {
			return "No downloadable format is available for this Twitch video."
		}
		if strings.Contains(diagnostic, "429") || strings.Contains(diagnostic, "rate limit") {
			return "Twitch temporarily rate-limited this request. Try again later."
		}
		if strings.Contains(diagnostic, "network") || strings.Contains(diagnostic, "timed out") || strings.Contains(diagnostic, "unable to download") {
			return "Could not reach Twitch. Check the network connection and try again."
		}
	}
	return ""
}

func exitErrorDiagnostic(err error, canonicalURL string) string {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return ""
	}
	value := string(exitErr.Stderr)
	if canonicalURL != "" {
		value = strings.ReplaceAll(value, canonicalURL, "[video]")
	}
	return truncate(value, 1000)
}

func verifyOutput(root, candidate string) (string, error) {
	if candidate == "" {
		return "", errors.New("missing output path")
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	fileAbs, err := filepath.Abs(candidate)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(rootAbs, fileAbs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return "", errors.New("output escaped configured folder")
	}
	info, err := os.Stat(fileAbs)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 {
		return "", errors.New("output missing or empty")
	}
	return fileAbs, nil
}

func findRecentOutput(root, videoID string, startedAt time.Time) (string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	marker := "[" + videoID + "]"
	var newestPath string
	var newestTime time.Time
	for _, entry := range entries {
		if entry.IsDir() || !strings.Contains(entry.Name(), marker) || strings.HasSuffix(strings.ToLower(entry.Name()), ".part") || strings.HasSuffix(strings.ToLower(entry.Name()), ".ytdl") {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.ModTime().Before(startedAt.Add(-2*time.Second)) {
			continue
		}
		if newestPath == "" || info.ModTime().After(newestTime) {
			newestPath = filepath.Join(root, entry.Name())
			newestTime = info.ModTime()
		}
	}
	if newestPath == "" {
		return "", errors.New("no newly created output matched the video")
	}
	return newestPath, nil
}

func validateDownloadPath(value string) (string, error) {
	if value == "" || len(value) > 240 || strings.IndexFunc(value, func(r rune) bool { return r < 32 }) >= 0 {
		return "", errors.New("Invalid download folder.")
	}
	if strings.HasPrefix(value, `\\`) || !filepath.IsAbs(value) {
		return "", errors.New("Download folder must be an absolute local Windows path.")
	}
	for _, segment := range strings.FieldsFunc(value, func(r rune) bool { return r == '\\' || r == '/' }) {
		if segment == ".." {
			return "", errors.New("Download folder traversal is not allowed.")
		}
	}
	clean := filepath.Clean(value)
	volume := filepath.VolumeName(clean)
	if strings.EqualFold(clean, volume+`\`) || hasReparsePoint(clean) {
		return "", errors.New("Download folder is not allowed.")
	}
	return clean, nil
}

func prepareDownloadPath(value string) (string, error) {
	path, err := validateDownloadPath(value)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(path, 0700); err != nil {
		return "", errors.New("The download folder could not be created.")
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return "", errors.New("The download location is not a folder.")
	}
	probe, err := os.CreateTemp(path, ".viddock-write-test-")
	if err != nil {
		return "", errors.New("The download folder is not writable.")
	}
	probePath := probe.Name()
	closeErr := probe.Close()
	removeErr := os.Remove(probePath)
	if closeErr != nil || removeErr != nil {
		return "", errors.New("The download folder is not writable.")
	}
	return path, nil
}

func validChoice(value string, choices []string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}

func validQuality(value string) bool {
	if value == "best" {
		return true
	}
	quality, err := strconv.Atoi(value)
	if err == nil && quality >= 64 && quality <= 8640 && strconv.Itoa(quality) == value {
		return true
	}
	_, _, _, ok := parseResolutionAndFPS(value)
	return ok
}

func parseResolutionAndFPS(value string) (int, int, int, bool) {
	dimensions, fpsText, hasFPS := strings.Cut(value, "@")
	if hasFPS && (fpsText == "" || strings.Contains(fpsText, "@")) {
		return 0, 0, 0, false
	}
	widthText, heightText, found := strings.Cut(dimensions, "x")
	if !found || widthText == "" || heightText == "" {
		return 0, 0, 0, false
	}
	width, widthErr := strconv.Atoi(widthText)
	height, heightErr := strconv.Atoi(heightText)
	if widthErr != nil || heightErr != nil || width < 16 || height < 16 || width > 16384 || height > 16384 || strconv.Itoa(width) != widthText || strconv.Itoa(height) != heightText {
		return 0, 0, 0, false
	}
	fps := 0
	if hasFPS {
		var fpsErr error
		fps, fpsErr = strconv.Atoi(fpsText)
		if fpsErr != nil || fps < 45 || fps > 240 || strconv.Itoa(fps) != fpsText {
			return 0, 0, 0, false
		}
	}
	return width, height, fps, true
}

func executableDir() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(path), nil
}

func localAppData() string {
	if value := os.Getenv("LOCALAPPDATA"); value != "" {
		return value
	}
	return filepath.Join(os.TempDir(), "VidDock")
}

func userDownloads() string {
	if profile := os.Getenv("USERPROFILE"); profile != "" {
		return filepath.Join(profile, "Downloads")
	}
	return os.TempDir()
}

func originArgument() string {
	if len(os.Args) > 1 && os.Args[1] == "--stdio-test" {
		return "stdio-test"
	}
	if len(os.Args) > 1 && strings.HasPrefix(os.Args[1], "chrome-extension://") {
		return os.Args[1]
	}
	return ""
}

func firstLine(value string) string {
	if i := strings.IndexAny(value, "\r\n"); i >= 0 {
		return value[:i]
	}
	return value
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}

func safeDiagnosticLine(value, canonicalURL, downloadPath string) string {
	value = strings.ReplaceAll(value, canonicalURL, "[video]")
	value = strings.ReplaceAll(value, downloadPath, "[download-folder]")
	return truncate(value, 500)
}

func fatalNative(err error) {
	_, _ = fmt.Fprintln(os.Stderr, "VidDock helper failed:", err)
	os.Exit(1)
}
