package main

import (
	"bytes"
	"reflect"
	"testing"
)

// dailymotionHLSFormats mirrors the verified combined HLS formats of a public
// Dailymotion video: three renditions, each one stream with audio and video.
func dailymotionHLSFormats() []mediaFormat {
	return normalizeFormats([]mediaFormat{
		{ID: "hls-380", Ext: "mp4", VideoExt: "mp4", AudioExt: "m4a", Width: 512, Height: 288, VCodec: "avc1.4d401f", ACodec: "mp4a.40.2", Protocol: "m3u8_native", TBR: 380},
		{ID: "hls-480", Ext: "mp4", VideoExt: "mp4", AudioExt: "m4a", Width: 848, Height: 480, VCodec: "avc1.4d401f", ACodec: "mp4a.40.2", Protocol: "m3u8_native", TBR: 780},
		{ID: "hls-720", Ext: "mp4", VideoExt: "mp4", AudioExt: "m4a", Width: 1280, Height: 720, VCodec: "avc1.64001f", ACodec: "mp4a.40.2", Protocol: "m3u8_native", TBR: 1500},
	})
}

func TestDailymotionQualitiesComeFromTheCombinedHLSFormats(t *testing.T) {
	qualities := availableQualitiesForDuration(dailymotionHLSFormats(), 60)
	// 848x480 is not exactly 16:9, so the existing labeling shows its
	// dimensions instead of "480p".
	want := []string{"720p", "848×480", "288p"}
	if len(qualities) != len(want) {
		t.Fatalf("qualities = %#v", qualities)
	}
	for i, label := range want {
		if qualities[i].Label != label {
			t.Errorf("quality %d = %q, want %q", i, qualities[i].Label, label)
		}
	}
	if qualities[0].ApproxBytes <= 0 {
		t.Fatal("size estimate should come from the playlist bitrate")
	}
}

func TestDailymotionExplicitQualitySelectsOneCombinedStreamWithoutMergingAudio(t *testing.T) {
	for _, container := range []string{"mp4", "mkv"} {
		for _, test := range []struct {
			width, height int
			id            string
		}{{512, 288, "hls-380"}, {848, 480, "hls-480"}, {1280, 720, "hls-720"}} {
			selected, err := selectVideoFormatByResolutionAndFPS(dailymotionHLSFormats(), test.width, test.height, 0, container)
			if err != nil || selected.ID != test.id {
				t.Fatalf("%s %dx%d = %#v, %v", container, test.width, test.height, selected, err)
			}
			expression, err := selectedFormatExpressionForSource(selected, mediaFormat{}, sourceDailymotion)
			if err != nil || expression != test.id {
				t.Fatalf("%s expression = %q, %v (a combined stream must not add an audio part)", container, expression, err)
			}
		}
	}
	selected, err := selectVideoFormat(dailymotionHLSFormats(), 480, "mp4")
	if err != nil || selected.ID != "hls-480" {
		t.Fatalf("tier selection = %#v, %v", selected, err)
	}
}

func TestDailymotionSelectionToleratesMissingCodecMetadata(t *testing.T) {
	// If the playlist omits CODECS, yt-dlp reports no vcodec/acodec. The
	// combined stream must still be selectable and must not be paired with a
	// separately selected audio stream.
	formats := normalizeFormats([]mediaFormat{{ID: "hls-480", Ext: "mp4", VideoExt: "mp4", Width: 848, Height: 480, Protocol: "m3u8_native"}})
	selected, err := selectVideoFormatByResolutionAndFPS(formats, 848, 480, 0, "mp4")
	if err != nil || selected.ID != "hls-480" {
		t.Fatalf("selection = %#v, %v", selected, err)
	}
	expression, err := selectedFormatExpressionForSource(selected, mediaFormat{}, sourceDailymotion)
	if err != nil || expression != "hls-480" {
		t.Fatalf("expression = %q, %v", expression, err)
	}
	if !hasCombinedFormats(sourceDailymotion) {
		t.Fatal("Dailymotion streams are combined")
	}
}

func TestDailymotionMP4RefusesIncompatibleCodecsInsteadOfTranscoding(t *testing.T) {
	formats := normalizeFormats([]mediaFormat{{ID: "hls-480", Ext: "webm", VideoExt: "webm", Width: 848, Height: 480, VCodec: "vp9", ACodec: "opus", Protocol: "m3u8_native"}})
	if _, err := selectVideoFormatByResolutionAndFPS(formats, 848, 480, 0, "mp4"); err == nil {
		t.Fatal("MP4 must not be promised for a stream that would need transcoding")
	}
	if selected, err := selectVideoFormatByResolutionAndFPS(formats, 848, 480, 0, "mkv"); err != nil || selected.ID != "hls-480" {
		t.Fatalf("MKV should accept the stream: %#v, %v", selected, err)
	}
}

func TestCombinedFormatSourcesAreExactlyTwitchClipsAndDailymotion(t *testing.T) {
	for source, want := range map[mediaSource]bool{sourceYouTube: false, sourceTwitchClip: true, sourceTwitchVOD: false, sourceDailymotion: true} {
		if got := hasCombinedFormats(source); got != want {
			t.Errorf("hasCombinedFormats(%s) = %v, want %v", source, got, want)
		}
	}
	// YouTube still merges a separate audio stream.
	expression, err := selectedFormatExpressionForSource(mediaFormat{ID: "137", ACodec: "none"}, mediaFormat{ID: "140"}, sourceYouTube)
	if err != nil || expression != "137+140" {
		t.Fatalf("YouTube expression = %q, %v", expression, err)
	}
}

func TestBaseVideoSelectorKeepsExistingSourcesAndTreatsDailymotionAsCombined(t *testing.T) {
	const defaultSelector = "bestvideo*+bestaudio/best"
	const mp4Best = "bestvideo*[ext=mp4][vcodec^=avc1]+bestaudio[ext=m4a]/bestvideo*[ext=mp4][vcodec^=av01]+bestaudio[ext=m4a]/bestvideo*[ext=mp4][vcodec^=hvc1]+bestaudio[ext=m4a]/bestvideo*[ext=mp4][vcodec^=hev1]+bestaudio[ext=m4a]/best[ext=mp4]/best"
	tests := []struct {
		source      mediaSource
		container   string
		quality     string
		selector    string
		needsFormat bool
	}{
		{sourceYouTube, "mp4", "best", mp4Best, false},
		{sourceYouTube, "mkv", "best", defaultSelector, false},
		{sourceYouTube, "mp4", "1920x1080", defaultSelector, true},
		{sourceTwitchClip, "mp4", "best", "best", false},
		{sourceTwitchVOD, "mkv", "best", "best", false},
		{sourceTwitchVOD, "mp4", "1280x720", defaultSelector, true},
		{sourceDailymotion, "mp4", "best", "best", false},
		{sourceDailymotion, "mkv", "best", "best", false},
		{sourceDailymotion, "mp4", "848x480", defaultSelector, true},
		{sourceDailymotion, "mkv", "1280x720", defaultSelector, true},
	}
	for _, test := range tests {
		selector, needs := baseVideoSelector(test.source, test.container, test.quality)
		if selector != test.selector || needs != test.needsFormat {
			t.Errorf("%s/%s/%s = (%q, %v), want (%q, %v)", test.source, test.container, test.quality, selector, needs, test.selector, test.needsFormat)
		}
	}
}

func TestAudioDownloadArgsPerSource(t *testing.T) {
	tests := []struct {
		source mediaSource
		format string
		want   []string
	}{
		{sourceYouTube, "best", []string{"--format", "bestaudio"}},
		{sourceYouTube, "m4a", []string{"--format", "bestaudio[ext=m4a]/bestaudio", "--extract-audio", "--audio-format", "m4a"}},
		{sourceYouTube, "mp3", []string{"--extract-audio", "--audio-format", "mp3", "--audio-quality", "0"}},
		{sourceTwitchVOD, "best", []string{"--format", "bestaudio"}},
		{sourceTwitchVOD, "m4a", []string{"--format", "bestaudio", "--extract-audio", "--audio-format", "m4a"}},
		{sourceTwitchClip, "best", []string{"--format", "best", "--extract-audio", "--audio-format", "m4a"}},
		{sourceTwitchClip, "mp3", []string{"--format", "best", "--extract-audio", "--audio-format", "mp3", "--audio-quality", "0"}},
		// Dailymotion has no audio-only stream: audio is always extracted from
		// the combined stream, even for the "best" (native) choice.
		{sourceDailymotion, "best", []string{"--format", "best", "--extract-audio", "--audio-format", "m4a"}},
		{sourceDailymotion, "m4a", []string{"--format", "best", "--extract-audio", "--audio-format", "m4a"}},
		{sourceDailymotion, "mp3", []string{"--format", "best", "--extract-audio", "--audio-format", "mp3", "--audio-quality", "0"}},
	}
	for _, test := range tests {
		if got := audioDownloadArgs(test.source, test.format); !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s/%s = %q, want %q", test.source, test.format, got, test.want)
		}
	}
}

func TestOnlyDailymotionIsRestrictedToAnExtractor(t *testing.T) {
	for source, want := range map[mediaSource]string{sourceYouTube: "", sourceTwitchClip: "", sourceTwitchVOD: "", sourceDailymotion: "dailymotion"} {
		if got := ytdlpExtractorFor(source); got != want {
			t.Errorf("ytdlpExtractorFor(%s) = %q, want %q", source, got, want)
		}
	}
}

func TestThumbnailsAreOmittedForDailymotionOnly(t *testing.T) {
	const url = "https://s1.dmcdn.net/v/abc/x240"
	if got := thumbnailForSource(sourceDailymotion, url); got != "" {
		t.Fatalf("Dailymotion thumbnail = %q", got)
	}
	for _, source := range []mediaSource{sourceYouTube, sourceTwitchClip, sourceTwitchVOD} {
		if got := thumbnailForSource(source, "https://i.ytimg.com/vi/x/hq.jpg"); got != "https://i.ytimg.com/vi/x/hq.jpg" {
			t.Errorf("%s thumbnail changed: %q", source, got)
		}
	}
}

func TestDailymotionMetadataMustMatchTheRequestedID(t *testing.T) {
	target := mediaTarget{ID: "xmo8jh", Source: sourceDailymotion}
	if !metadataMatchesTarget("xmo8jh", "", target) {
		t.Fatal("matching ID rejected")
	}
	for _, id := range []string{"", "x8jmo", "XMO8JH", "xmo8jh_extra"} {
		if metadataMatchesTarget(id, "xmo8jh", target) {
			t.Errorf("metadata ID %q accepted", id)
		}
	}
}

func TestDailymotionProgressNeverMovesBackwards(t *testing.T) {
	var output bytes.Buffer
	a := &app{out: &output}
	j := &job{State: "preparing", source: sourceDailymotion}
	a.parseProgress(j, "[download]  50.0% of ~   8.00MiB at    1.00MiB/s ETA 00:03 (frag 6/11)")
	a.parseProgress(j, "[download]  45.0% of ~   7.00MiB at    1.00MiB/s ETA 00:03 (frag 6/12)")
	if j.State != "downloading" || j.Percent != 50 || j.TotalBytes != 8*1024*1024 {
		t.Fatalf("HLS estimate moved backwards: %+v", j)
	}
	a.parseProgress(j, "[download]  60.0% of ~   9.00MiB at    1.00MiB/s ETA 00:02 (frag 7/12)")
	if j.Percent != 60 || j.TotalBytes != 9*1024*1024 {
		t.Fatalf("forward progress lost: %+v", j)
	}
}

func TestYouTubeProgressKeepsItsExistingSemantics(t *testing.T) {
	var output bytes.Buffer
	a := &app{out: &output}
	j := &job{State: "preparing", source: sourceYouTube}
	a.parseProgress(j, "[download]  50.0% of 8.00MiB at 1.00MiB/s ETA 00:03")
	a.parseProgress(j, "[download]  10.0% of 4.00MiB at 1.00MiB/s ETA 00:03")
	if j.Percent != 10 || j.TotalBytes != 4*1024*1024 {
		t.Fatalf("YouTube progress is reported as yt-dlp prints it: %+v", j)
	}
}

func TestDailymotionFriendlyErrors(t *testing.T) {
	tests := []struct {
		diagnostic string
		want       string
	}{
		{"ERROR: [dailymotion] xmo8jh: Video gone", "This Dailymotion video is no longer available."},
		{"ERROR: [dailymotion] xmo8jh: This video is geo-restricted", "This Dailymotion video is not available in your region."},
		{"ERROR: [dailymotion] xmo8jh: This video is password protected", "This Dailymotion video is not publicly available without signing in."},
		{"ERROR: [dailymotion] xmo8jh: Private video; please log in", "This Dailymotion video is not publicly available without signing in."},
		{"ERROR: This video is DRM protected", "This Dailymotion video is protected and cannot be downloaded."},
		{"ERROR: Requested format is not available", "No downloadable format is available for this Dailymotion video."},
		{"ERROR: unable to download video data: HTTP Error 429: Too Many Requests", "Dailymotion temporarily rate-limited this request. Try again later."},
		{"ERROR: unable to download video data: HTTP Error 403: Forbidden", "Dailymotion declined this request. Try again later."},
		{"ERROR: Unable to download webpage: timed out", "Could not reach Dailymotion. Check the network connection and try again."},
	}
	for _, test := range tests {
		if got := friendlyMediaDiagnostic("Download failed", test.diagnostic, sourceDailymotion); got != test.want {
			t.Errorf("%q => %q, want %q", test.diagnostic, got, test.want)
		}
	}
	// A throughput figure such as 403KiB/s must not look like an HTTP error.
	if got := friendlyMediaDiagnostic("Download failed", "[download] 40.0% of ~ 7.00MiB at 403.20KiB/s", sourceDailymotion); got != "" {
		t.Fatalf("progress line was rewritten: %q", got)
	}
	// Dailymotion wording never leaks into other sources.
	if got := friendlyMediaDiagnostic("Download failed", "ERROR: Video gone", sourceYouTube); got != "" {
		t.Fatalf("YouTube rewritten: %q", got)
	}
}

func TestDailymotionSafeMessagesContainNoPathsOrURLs(t *testing.T) {
	for _, message := range []string{msgDailymotionLive, friendlyMediaDiagnostic("x", "video gone", sourceDailymotion)} {
		for _, bad := range []string{`\`, "http", "C:"} {
			if bytes.Contains([]byte(message), []byte(bad)) {
				t.Errorf("message %q contains %q", message, bad)
			}
		}
	}
}
