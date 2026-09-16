package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAvailableQualitiesKeepsDistinctActualDimensions(t *testing.T) {
	formats := []mediaFormat{
		{ID: "401", Width: 3840, Height: 1644, FormatNote: "2160p", VCodec: "av01.0.12M.08", ACodec: "none", Ext: "mp4", Protocol: "https"},
		{ID: "400", Width: 2560, Height: 1096, FormatNote: "1440p", VCodec: "av01.0.12M.08", ACodec: "none", Ext: "mp4", Protocol: "https"},
		{ID: "399", Width: 1920, Height: 822, FormatNote: "1080p", VCodec: "av01.0.08M.08", ACodec: "none", Ext: "mp4", Protocol: "https"},
		{ID: "v401", Width: 2160, Height: 3840, FormatNote: "2160p", VCodec: "av01.0.12M.08", ACodec: "none", Ext: "mp4", Protocol: "https"},
		{ID: "270", Width: 608, Height: 1080, VCodec: "avc1.64001F", ACodec: "none", Ext: "mp4", Protocol: "m3u8_native"},
		{ID: "story", Width: 160, Height: 90, FormatNote: "storyboard", VCodec: "none", ACodec: "none", Ext: "mhtml", Protocol: "mhtml"},
	}
	want := []videoResolution{
		{Width: 2160, Height: 3840, Label: "2160×3840"},
		{Width: 3840, Height: 1644, Label: "3840×1644"},
		{Width: 2560, Height: 1096, Label: "2560×1096"},
		{Width: 1920, Height: 822, Label: "1920×822"},
		{Width: 608, Height: 1080, Label: "608×1080"},
	}
	got := availableQualities(formats)
	if len(got) != len(want) {
		t.Fatalf("availableQualities() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("availableQualities() = %v, want %v", got, want)
		}
	}
}

func TestFindRecentOutputUsesVideoMarkerAndRejectsPartFiles(t *testing.T) {
	root := t.TempDir()
	started := time.Now()
	part := filepath.Join(root, "Video [abc123xyz] [1080p].mp4.part")
	final := filepath.Join(root, "Video [abc123xyz] [1080p].mp4")
	if err := os.WriteFile(part, []byte("partial"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(final, []byte("complete"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := findRecentOutput(root, "abc123xyz", started)
	if err != nil || got != final {
		t.Fatalf("findRecentOutput() = %q, %v; want %q", got, err, final)
	}
}

func TestAvailableQualitiesIncludesVideoOnlyAndDeduplicates(t *testing.T) {
	formats := []mediaFormat{
		{ID: "315", Width: 3840, Height: 2160, FormatNote: "2160p60", VCodec: "vp9", ACodec: "none", Ext: "webm", Protocol: "https"},
		{ID: "401", Width: 3840, Height: 2160, FormatNote: "2160p60", VCodec: "av01", ACodec: "none", Ext: "mp4", Protocol: "https"},
	}
	got := availableQualities(formats)
	if len(got) != 1 || got[0] != (videoResolution{Width: 3840, Height: 2160, Label: "2160p (4K)"}) {
		t.Fatalf("availableQualities() = %v, want one 3840x2160 resolution", got)
	}
}

func TestResolutionLabels(t *testing.T) {
	tests := []struct {
		width, height int
		want          string
	}{
		{3840, 2160, "2160p (4K)"}, {2560, 1440, "1440p"}, {1920, 1080, "1080p"}, {1280, 720, "720p"},
		{854, 480, "480p"}, {640, 360, "360p"}, {426, 240, "240p"}, {256, 144, "144p"},
		{3840, 1920, "3840×1920"}, {2560, 1280, "2560×1280"}, {1920, 960, "1920×960"},
		{3440, 1440, "3440×1440"}, {1080, 1920, "1080×1920"}, {2160, 3840, "2160×3840"},
		{1080, 1080, "1080×1080"}, {1440, 1440, "1440×1440"},
	}
	for _, test := range tests {
		if got := resolutionLabel(test.width, test.height); got != test.want {
			t.Errorf("resolutionLabel(%d, %d) = %q, want %q", test.width, test.height, got, test.want)
		}
	}
}

func TestSelectVideoFormatUsesExactDimensions(t *testing.T) {
	formats := []mediaFormat{
		{ID: "401", Width: 3840, Height: 1920, FormatNote: "2160p", VCodec: "av01", ACodec: "none", Ext: "mp4", Protocol: "https", TBR: 4000},
		{ID: "true4k", Width: 3840, Height: 2160, FormatNote: "2160p", VCodec: "av01", ACodec: "none", Ext: "mp4", Protocol: "https", TBR: 3000},
	}
	got, err := selectVideoFormatByResolution(formats, 3840, 1920, "mp4")
	if err != nil || got.ID != "401" {
		t.Fatalf("exact selection = %#v, %v", got, err)
	}
}

func TestAvailableQualitiesDoesNotMergeSameWidthDifferentHeights(t *testing.T) {
	formats := []mediaFormat{
		{ID: "wide", Width: 3840, Height: 1920, FormatNote: "2160p", VCodec: "av01", ACodec: "none", Ext: "mp4", Protocol: "https"},
		{ID: "standard", Width: 3840, Height: 2160, FormatNote: "2160p", VCodec: "av01", ACodec: "none", Ext: "mp4", Protocol: "https"},
	}
	got := availableQualities(formats)
	want := []videoResolution{{Width: 3840, Height: 2160, Label: "2160p (4K)"}, {Width: 3840, Height: 1920, Label: "3840×1920"}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("availableQualities() = %#v, want %#v", got, want)
	}
}

func TestSelectVideoFormatPrefersCompatibleMP4AtExactTier(t *testing.T) {
	formats := []mediaFormat{
		{ID: "315", Width: 3840, Height: 2160, FormatNote: "2160p60", VCodec: "vp9", ACodec: "none", Ext: "webm", Protocol: "https", TBR: 17000},
		{ID: "401", Width: 3840, Height: 2160, FormatNote: "2160p60", VCodec: "av01", ACodec: "none", Ext: "mp4", Protocol: "https", TBR: 9000},
		{ID: "140", VCodec: "none", ACodec: "mp4a.40.2", Ext: "m4a", Protocol: "https", TBR: 129},
	}
	got, err := selectVideoFormat(formats, 2160, "mp4")
	if err != nil || got.ID != "401" {
		t.Fatalf("selectVideoFormat() = %#v, %v; want format 401", got, err)
	}
	audio, err := selectAudioFormat(formats, "mp4")
	if err != nil || audio.ID != "140" {
		t.Fatalf("selectAudioFormat() = %#v, %v", audio, err)
	}
	expression, err := selectedFormatExpression(got, audio)
	if err != nil || expression != "401+140" {
		t.Fatalf("selectedFormatExpression() = %q, %v", expression, err)
	}
}

func TestSelectVideoFormatRequiresMKVForVP9WebM(t *testing.T) {
	formats := []mediaFormat{{ID: "315", Width: 3840, Height: 2160, FormatNote: "2160p", VCodec: "vp9", ACodec: "none", Ext: "webm", Protocol: "https"}}
	_, err := selectVideoFormat(formats, 2160, "mp4")
	if err == nil || !strings.Contains(err.Error(), "Choose MKV") {
		t.Fatalf("selectVideoFormat() error = %v, want MKV recommendation", err)
	}
	got, err := selectVideoFormat(formats, 2160, "mkv")
	if err != nil || got.ID != "315" {
		t.Fatalf("MKV selectVideoFormat() = %#v, %v", got, err)
	}
}

func TestTwitchQualitiesUseAspectRatioFPSAndSource(t *testing.T) {
	formats := normalizeFormats([]mediaFormat{
		{ID: "portrait-1080", Height: 1080, AspectRatio: 0.5625, FPS: 60, Ext: "mp4", VideoExt: "mp4", Protocol: "https"},
		{ID: "1080p60", Height: 1080, AspectRatio: 16.0 / 9.0, FPS: 60, Ext: "mp4", VideoExt: "mp4", ACodec: "mp4a.40.2", FormatNote: "Source", Protocol: "m3u8_native", TBR: 7000},
		{ID: "1080p30", Width: 1920, Height: 1080, FPS: 30, Ext: "mp4", VideoExt: "mp4", Protocol: "m3u8_native", TBR: 4000},
	})
	qualities := availableQualitiesForDuration(formats, 3600)
	want := []videoResolution{
		{Width: 1920, Height: 1080, FPS: 60, Label: "Source (1080p60)", ApproxBytes: 3150000000},
		{Width: 1920, Height: 1080, Label: "1080p", ApproxBytes: 1800000000},
		{Width: 608, Height: 1080, FPS: 60, Label: "608×1080 · 60 fps"},
	}
	if len(qualities) != len(want) {
		t.Fatalf("Twitch qualities = %#v, want %#v", qualities, want)
	}
	for index := range want {
		if qualities[index] != want[index] {
			t.Fatalf("Twitch qualities = %#v, want %#v", qualities, want)
		}
	}
}

func TestTwitchClipDirectFormatSelection(t *testing.T) {
	formats := normalizeFormats([]mediaFormat{{ID: "1080", Height: 1080, AspectRatio: 16.0 / 9.0, FPS: 60, Ext: "mp4", VideoExt: "mp4", Protocol: "https"}})
	selected, err := selectVideoFormatByResolutionAndFPS(formats, 1920, 1080, 60, "mp4")
	if err != nil || selected.ID != "1080" {
		t.Fatalf("Twitch clip selection = %#v, %v", selected, err)
	}
	expression, err := selectedFormatExpressionForSource(selected, mediaFormat{}, sourceTwitchClip)
	if err != nil || expression != "1080" {
		t.Fatalf("Twitch clip expression = %q, %v", expression, err)
	}
}

func TestFPSQualityParsing(t *testing.T) {
	width, height, fps, ok := parseResolutionAndFPS("1920x1080@60")
	if !ok || width != 1920 || height != 1080 || fps != 60 || !validQuality("1920x1080@60") {
		t.Fatalf("FPS quality parsed as %dx%d@%d, %v", width, height, fps, ok)
	}
	for _, invalid := range []string{"1920x1080@", "1920x1080@30", "1920x1080@999", "1920x1080@60@60"} {
		if validQuality(invalid) {
			t.Errorf("invalid FPS quality accepted: %q", invalid)
		}
	}
}

func TestLongVODProgressSizesUseInt64(t *testing.T) {
	bytes, ok := parseByteSize("12.5", "GiB")
	if !ok || bytes != 13421772800 {
		t.Fatalf("parseByteSize() = %d, %v", bytes, ok)
	}
	if _, ok := parseByteSize("1e99", "GiB"); ok {
		t.Fatal("overflowing progress size was accepted")
	}
}

func TestTwitchFriendlyErrors(t *testing.T) {
	tests := []struct {
		diagnostic string
		source     mediaSource
		want       string
	}{
		{"ERROR: This clip is no longer available", sourceTwitchClip, "This Twitch clip is no longer available."},
		{"ERROR: This video is only available to subscribers", sourceTwitchVOD, "This Twitch video is not publicly available without authentication."},
		{"ERROR: HTTP Error 429: Too Many Requests", sourceTwitchVOD, "Twitch temporarily rate-limited this request. Try again later."},
	}
	for _, test := range tests {
		if got := friendlyMediaDiagnostic("Download failed", test.diagnostic, test.source); got != test.want {
			t.Errorf("friendly error = %q, want %q", got, test.want)
		}
	}
}
