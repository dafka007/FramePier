package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	qualityNotePattern = regexp.MustCompile(`(?i)^\s*([0-9]{2,4})p(?:[0-9]+)?(?:\s|$)`)
	formatIDPattern    = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
)

type mediaFormat struct {
	ID                 string  `json:"format_id"`
	Ext                string  `json:"ext"`
	VideoExt           string  `json:"video_ext"`
	AudioExt           string  `json:"audio_ext"`
	Width              int     `json:"width"`
	Height             int     `json:"height"`
	AspectRatio        float64 `json:"aspect_ratio"`
	FormatNote         string  `json:"format_note"`
	VCodec             string  `json:"vcodec"`
	ACodec             string  `json:"acodec"`
	Protocol           string  `json:"protocol"`
	FPS                float64 `json:"fps"`
	TBR                float64 `json:"tbr"`
	Filesize           int64   `json:"filesize"`
	ApproxSize         int64   `json:"filesize_approx"`
	LanguagePreference int     `json:"language_preference"`
}

type videoMetadata struct {
	ID       string        `json:"id"`
	Duration float64       `json:"duration"`
	Formats  []mediaFormat `json:"formats"`
}

type videoResolution struct {
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	FPS         int    `json:"fps,omitempty"`
	Label       string `json:"label"`
	ApproxBytes int64  `json:"approxBytes,omitempty"`
}

func loadVideoMetadata(ctx context.Context, ytdlp, workingDirectory, canonicalURL string) (videoMetadata, error) {
	cmd := newHiddenCommandContext(ctx, ytdlp, "--dump-single-json", "--no-playlist", "--no-warnings", "--skip-download", canonicalURL)
	cmd.Dir = workingDirectory
	output, err := cmd.Output()
	if err != nil {
		return videoMetadata{}, err
	}
	var metadata videoMetadata
	if err := json.Unmarshal(output, &metadata); err != nil || metadata.ID == "" {
		return videoMetadata{}, errors.New("downloader returned invalid metadata")
	}
	metadata.Formats = normalizeFormats(metadata.Formats)
	return metadata, nil
}

func normalizeFormats(formats []mediaFormat) []mediaFormat {
	normalized := append([]mediaFormat(nil), formats...)
	for index := range normalized {
		format := &normalized[index]
		if format.Width == 0 && format.Height > 0 && format.AspectRatio > 0 {
			width := int(float64(format.Height)*format.AspectRatio + 0.5)
			if width >= 16 && width <= 16384 {
				format.Width = width
			}
		}
	}
	return normalized
}

func availableQualities(formats []mediaFormat) []videoResolution {
	return availableQualitiesForDuration(formats, 0)
}

func availableQualitiesForDuration(formats []mediaFormat, duration float64) []videoResolution {
	unique := make(map[string]videoResolution)
	for _, format := range formats {
		if !usableVideoFormat(format) {
			continue
		}
		fps := qualityFPS(format.FPS)
		key := resolutionKey(format.Width, format.Height) + "@" + strconv.Itoa(fps)
		source := strings.Contains(strings.ToLower(format.FormatNote), "source")
		candidate := videoResolution{Width: format.Width, Height: format.Height, FPS: fps, Label: resolutionLabelWithFPS(format.Width, format.Height, fps, source), ApproxBytes: estimatedFormatBytes(format, duration)}
		if previous, ok := unique[key]; ok {
			if strings.HasPrefix(previous.Label, "Source (") {
				candidate.Label = previous.Label
			}
			if candidate.ApproxBytes < previous.ApproxBytes {
				candidate.ApproxBytes = previous.ApproxBytes
			}
		}
		unique[key] = candidate
	}
	qualities := make([]videoResolution, 0, len(unique))
	for _, quality := range unique {
		qualities = append(qualities, quality)
	}
	sort.Slice(qualities, func(i, j int) bool {
		leftPixels := int64(qualities[i].Width) * int64(qualities[i].Height)
		rightPixels := int64(qualities[j].Width) * int64(qualities[j].Height)
		if leftPixels != rightPixels {
			return leftPixels > rightPixels
		}
		if qualities[i].Width != qualities[j].Width {
			return qualities[i].Width > qualities[j].Width
		}
		if qualities[i].Height != qualities[j].Height {
			return qualities[i].Height > qualities[j].Height
		}
		return qualities[i].FPS > qualities[j].FPS
	})
	return qualities
}

func resolutionKey(width, height int) string { return strconv.Itoa(width) + "x" + strconv.Itoa(height) }

func resolutionLabel(width, height int) string {
	return resolutionLabelWithFPS(width, height, 0, false)
}

func resolutionLabelWithFPS(width, height, fps int, source bool) string {
	var label string
	if width > height && width > 0 && height > 0 && absInt(width*9-height*16) <= 9 {
		if width == 3840 && height == 2160 {
			label = "2160p"
			if fps > 30 {
				label += strconv.Itoa(fps)
			}
			label += " (4K)"
		} else {
			label = strconv.Itoa(height) + "p"
			if fps > 30 {
				label += strconv.Itoa(fps)
			}
		}
	} else {
		label = strconv.Itoa(width) + "×" + strconv.Itoa(height)
		if fps > 30 {
			label += " · " + strconv.Itoa(fps) + " fps"
		}
	}
	if source {
		return "Source (" + label + ")"
	}
	return label
}

func qualityFPS(value float64) int {
	if value < 45 || value > 240 {
		return 0
	}
	return int(value + 0.5)
}

func estimatedFormatBytes(format mediaFormat, duration float64) int64 {
	if format.Filesize > 0 {
		return format.Filesize
	}
	if format.ApproxSize > 0 {
		return format.ApproxSize
	}
	if duration > 0 && format.TBR > 0 {
		estimate := duration * format.TBR * 125
		if estimate > 0 && estimate < float64(^uint64(0)>>1) {
			return int64(estimate)
		}
	}
	return 0
}

func estimatedDownloadBytes(metadata videoMetadata, quality, mode string) int64 {
	var estimate int64
	if mode == "audio" {
		for _, format := range metadata.Formats {
			if strings.EqualFold(format.VCodec, "none") || (format.VideoExt == "none" && format.AudioExt != "none") {
				if value := estimatedFormatBytes(format, metadata.Duration); value > estimate {
					estimate = value
				}
			}
		}
		return estimate
	}
	qualities := availableQualitiesForDuration(metadata.Formats, metadata.Duration)
	if quality == "best" && len(qualities) > 0 {
		return qualities[0].ApproxBytes
	}
	width, height, fps, ok := parseResolutionAndFPS(quality)
	if !ok {
		return 0
	}
	for _, candidate := range qualities {
		if candidate.Width == width && candidate.Height == height && (fps == 0 || candidate.FPS == fps) && candidate.ApproxBytes > estimate {
			estimate = candidate.ApproxBytes
		}
	}
	return estimate
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func formatQuality(format mediaFormat) int {
	if !usableVideoFormat(format) {
		return 0
	}
	if quality := qualityFromNote(format.FormatNote); quality > 0 {
		return quality
	}

	// YouTube quality labels follow the 16:9 width ladder for cropped ultrawide
	// sources and the short edge for ordinary landscape/portrait sources.
	quality := format.Height
	if format.Width < format.Height {
		quality = format.Width
	} else if format.Width*9 > format.Height*17 {
		quality = int(float64(format.Width)*9.0/16.0 + 0.5)
	}
	return quality
}

func qualityFromNote(note string) int {
	if match := qualityNotePattern.FindStringSubmatch(note); len(match) == 2 {
		quality, err := strconv.Atoi(match[1])
		if err == nil && quality > 0 && quality <= 8640 {
			return quality
		}
	}
	return 0
}

func usableVideoFormat(format mediaFormat) bool {
	vcodec := strings.ToLower(strings.TrimSpace(format.VCodec))
	videoExt := strings.ToLower(strings.TrimSpace(format.VideoExt))
	protocol := strings.ToLower(strings.TrimSpace(format.Protocol))
	note := strings.ToLower(format.FormatNote)
	return format.ID != "" && format.Width > 0 && format.Height > 0 &&
		((vcodec != "" && vcodec != "none") || (videoExt != "" && videoExt != "none")) &&
		!strings.Contains(protocol, "mhtml") && !strings.Contains(protocol, "image") &&
		!strings.Contains(note, "storyboard")
}

func selectVideoFormat(formats []mediaFormat, requested int, container string) (mediaFormat, error) {
	candidates := make([]mediaFormat, 0)
	for _, format := range formats {
		if formatQuality(format) == requested && formatIDPattern.MatchString(format.ID) {
			if container != "mp4" || mp4MuxSuitable(format) {
				candidates = append(candidates, format)
			}
		}
	}
	if len(candidates) == 0 {
		if container == "mp4" {
			for _, format := range formats {
				if formatQuality(format) == requested {
					return mediaFormat{}, fmt.Errorf("%dp is available, but its source codecs are not suitable for a compatible MP4 without video transcoding. Choose MKV for this quality", requested)
				}
			}
		}
		return mediaFormat{}, fmt.Errorf("%dp is no longer available for this video", requested)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := formatScore(candidates[i], container), formatScore(candidates[j], container)
		if left != right {
			return left > right
		}
		return candidates[i].ID < candidates[j].ID
	})
	return candidates[0], nil
}

func selectVideoFormatByResolution(formats []mediaFormat, width, height int, container string) (mediaFormat, error) {
	return selectVideoFormatByResolutionAndFPS(formats, width, height, 0, container)
}

func selectVideoFormatByResolutionAndFPS(formats []mediaFormat, width, height, fps int, container string) (mediaFormat, error) {
	candidates := make([]mediaFormat, 0)
	for _, format := range formats {
		if usableVideoFormat(format) && format.Width == width && format.Height == height && (fps == 0 || qualityFPS(format.FPS) == fps) && formatIDPattern.MatchString(format.ID) {
			if container != "mp4" || mp4MuxSuitable(format) {
				candidates = append(candidates, format)
			}
		}
	}
	label := resolutionLabel(width, height)
	if len(candidates) == 0 {
		if container == "mp4" {
			for _, format := range formats {
				if usableVideoFormat(format) && format.Width == width && format.Height == height && (fps == 0 || qualityFPS(format.FPS) == fps) {
					return mediaFormat{}, fmt.Errorf("%s is available, but its source codecs are not suitable for a compatible MP4 without video transcoding. Choose MKV for this resolution", label)
				}
			}
		}
		return mediaFormat{}, fmt.Errorf("%s is no longer available for this video", label)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := formatScore(candidates[i], container), formatScore(candidates[j], container)
		if left != right {
			return left > right
		}
		return candidates[i].ID < candidates[j].ID
	})
	return candidates[0], nil
}

func selectAudioFormat(formats []mediaFormat, container string, source mediaSource) (mediaFormat, error) {
	candidates := make([]mediaFormat, 0)
	for _, format := range formats {
		codec := strings.ToLower(format.ACodec)
		if !formatIDPattern.MatchString(format.ID) || !strings.EqualFold(format.VCodec, "none") || codec == "" || codec == "none" {
			continue
		}
		if container == "mp4" && (!strings.EqualFold(format.Ext, "m4a") || (!strings.HasPrefix(codec, "mp4a") && !strings.HasPrefix(codec, "aac"))) {
			continue
		}
		candidates = append(candidates, format)
	}
	if len(candidates) == 0 {
		if container == "mp4" {
			return mediaFormat{}, errors.New("compatible AAC/M4A audio is unavailable; choose MKV for this video")
		}
		return mediaFormat{}, errors.New("no usable audio stream is available for this video")
	}
	// YouTube-specific: prefer original-language audio tracks.
	// yt-dlp assigns language_preference == 10 to the original track and
	// negative values to AI dubs. Only restrict to == 10 when at least one
	// such candidate exists; otherwise preserve the old TBR→ID ordering.
	// This rule is YouTube-only; Twitch VODs must keep the old TBR→ID ordering.
	if source == sourceYouTube {
		originals := make([]mediaFormat, 0)
		for _, format := range candidates {
			if format.LanguagePreference == 10 {
				originals = append(originals, format)
			}
		}
		if len(originals) > 0 {
			candidates = originals
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].TBR != candidates[j].TBR {
			return candidates[i].TBR > candidates[j].TBR
		}
		return candidates[i].ID < candidates[j].ID
	})
	return candidates[0], nil
}

func mp4MuxSuitable(format mediaFormat) bool {
	codec := strings.ToLower(format.VCodec)
	if codec == "" && strings.EqualFold(format.Ext, "mp4") && strings.EqualFold(format.VideoExt, "mp4") {
		return true
	}
	switch {
	case strings.HasPrefix(codec, "avc1"), strings.HasPrefix(codec, "h264"):
		return true
	case strings.HasPrefix(codec, "av01"):
		return true
	case strings.HasPrefix(codec, "hvc1"), strings.HasPrefix(codec, "hev1"), strings.HasPrefix(codec, "hevc"):
		return true
	default:
		return false
	}
}

func formatScore(format mediaFormat, container string) float64 {
	score := format.TBR + format.FPS
	codec := strings.ToLower(format.VCodec)
	protocol := strings.ToLower(format.Protocol)
	if protocol == "https" || protocol == "http" {
		score += 100000
	}
	if format.ACodec != "" && format.ACodec != "none" {
		score += 1000
	}
	if container == "mp4" {
		if strings.EqualFold(format.Ext, "mp4") {
			score += 1000000
		}
		switch {
		case strings.HasPrefix(codec, "avc1"), strings.HasPrefix(codec, "h264"):
			score += 300000
		case strings.HasPrefix(codec, "av01"):
			score += 200000
		case strings.HasPrefix(codec, "hvc1"), strings.HasPrefix(codec, "hev1"), strings.HasPrefix(codec, "hevc"):
			score += 100000
		}
	}
	return score
}

func selectedFormatExpression(video, audio mediaFormat) (string, error) {
	return selectedFormatExpressionForSource(video, audio, sourceYouTube)
}

func selectedFormatExpressionForSource(video, audio mediaFormat, source mediaSource) (string, error) {
	if !formatIDPattern.MatchString(video.ID) {
		return "", errors.New("downloader returned an unsafe format identifier")
	}
	if source == sourceTwitchClip {
		return video.ID, nil
	}
	if video.ACodec != "" && video.ACodec != "none" {
		return video.ID, nil
	}
	if !formatIDPattern.MatchString(audio.ID) {
		return "", errors.New("downloader returned an unsafe audio format identifier")
	}
	return video.ID + "+" + audio.ID, nil
}
