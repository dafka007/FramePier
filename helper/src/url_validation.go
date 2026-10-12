package main

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

type mediaSource string

const (
	sourceYouTube     mediaSource = "youtube"
	sourceTwitchClip  mediaSource = "twitch_clip"
	sourceTwitchVOD   mediaSource = "twitch_vod"
	sourceDailymotion mediaSource = "dailymotion"
	sourceGeneric     mediaSource = "generic"
)

type mediaTarget struct {
	CanonicalURL string
	ID           string
	Source       mediaSource
	// Extractor is fixed by a narrowly validated site URL. It is never client-supplied.
	Extractor    string
}

var (
	twitchClipIDPattern  = regexp.MustCompile(`^[A-Za-z0-9_-]{6,100}$`)
	twitchChannelPattern = regexp.MustCompile(`^[A-Za-z0-9_]{1,25}$`)
	twitchVODIDPattern   = regexp.MustCompile(`^[0-9]{1,20}$`)
	// Dailymotion video IDs are "x" followed by lowercase base36. The optional
	// slug that older share links append after "_" is validated and discarded.
	dailymotionIDPattern   = regexp.MustCompile(`^x[0-9a-z]{4,10}$`)
	dailymotionSlugPattern = regexp.MustCompile(`^[A-Za-z0-9-]{1,200}package main

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

type mediaSource string

const (
	sourceYouTube     mediaSource = "youtube"
	sourceTwitchClip  mediaSource = "twitch_clip"
	sourceTwitchVOD   mediaSource = "twitch_vod"
	sourceDailymotion mediaSource = "dailymotion"
	sourceGeneric     mediaSource = "generic"
)

type mediaTarget struct {
	CanonicalURL string
	ID           string
	Source       mediaSource
	// Extractor is fixed by a narrowly validated site URL. It is never client-supplied.
	Extractor    string
}

var (
	twitchClipIDPattern  = regexp.MustCompile(`^[A-Za-z0-9_-]{6,100}$`)
	twitchChannelPattern = regexp.MustCompile(`^[A-Za-z0-9_]{1,25}$`)
	twitchVODIDPattern   = regexp.MustCompile(`^[0-9]{1,20}$`)
	// Dailymotion video IDs are "x" followed by lowercase base36. The optional
	// slug that older share links append after "_" is validated and discarded.
	dailymotionIDPattern   = regexp.MustCompile(`^x[0-9a-z]{4,10}$`)
)
	vimeoVideoPattern      = regexp.MustCompile(`^/[0-9]{1,20}package main

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

type mediaSource string

const (
	sourceYouTube     mediaSource = "youtube"
	sourceTwitchClip  mediaSource = "twitch_clip"
	sourceTwitchVOD   mediaSource = "twitch_vod"
	sourceDailymotion mediaSource = "dailymotion"
	sourceGeneric     mediaSource = "generic"
)

type mediaTarget struct {
	CanonicalURL string
	ID           string
	Source       mediaSource
	// Extractor is fixed by a narrowly validated site URL. It is never client-supplied.
	Extractor    string
}

var (
	twitchClipIDPattern  = regexp.MustCompile(`^[A-Za-z0-9_-]{6,100}$`)
	twitchChannelPattern = regexp.MustCompile(`^[A-Za-z0-9_]{1,25}$`)
	twitchVODIDPattern   = regexp.MustCompile(`^[0-9]{1,20}$`)
	// Dailymotion video IDs are "x" followed by lowercase base36. The optional
	// slug that older share links append after "_" is validated and discarded.
	dailymotionIDPattern   = regexp.MustCompile(`^x[0-9a-z]{4,10}$`)
)
	redditVideoPattern     = regexp.MustCompile(`^/r/[A-Za-z0-9_]{1,21}/comments/[A-Za-z0-9]{3,12}/[A-Za-z0-9_-]{1,200}/?package main

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

type mediaSource string

const (
	sourceYouTube     mediaSource = "youtube"
	sourceTwitchClip  mediaSource = "twitch_clip"
	sourceTwitchVOD   mediaSource = "twitch_vod"
	sourceDailymotion mediaSource = "dailymotion"
	sourceGeneric     mediaSource = "generic"
)

type mediaTarget struct {
	CanonicalURL string
	ID           string
	Source       mediaSource
	// Extractor is fixed by a narrowly validated site URL. It is never client-supplied.
	Extractor    string
}

var (
	twitchClipIDPattern  = regexp.MustCompile(`^[A-Za-z0-9_-]{6,100}$`)
	twitchChannelPattern = regexp.MustCompile(`^[A-Za-z0-9_]{1,25}$`)
	twitchVODIDPattern   = regexp.MustCompile(`^[0-9]{1,20}$`)
	// Dailymotion video IDs are "x" followed by lowercase base36. The optional
	// slug that older share links append after "_" is validated and discarded.
	dailymotionIDPattern   = regexp.MustCompile(`^x[0-9a-z]{4,10}$`)
)
	tiktokVideoPattern     = regexp.MustCompile(`^/@[A-Za-z0-9_.]{2,32}/video/[0-9]{6,24}package main

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

type mediaSource string

const (
	sourceYouTube     mediaSource = "youtube"
	sourceTwitchClip  mediaSource = "twitch_clip"
	sourceTwitchVOD   mediaSource = "twitch_vod"
	sourceDailymotion mediaSource = "dailymotion"
	sourceGeneric     mediaSource = "generic"
)

type mediaTarget struct {
	CanonicalURL string
	ID           string
	Source       mediaSource
	// Extractor is fixed by a narrowly validated site URL. It is never client-supplied.
	Extractor    string
}

var (
	twitchClipIDPattern  = regexp.MustCompile(`^[A-Za-z0-9_-]{6,100}$`)
	twitchChannelPattern = regexp.MustCompile(`^[A-Za-z0-9_]{1,25}$`)
	twitchVODIDPattern   = regexp.MustCompile(`^[0-9]{1,20}$`)
	// Dailymotion video IDs are "x" followed by lowercase base36. The optional
	// slug that older share links append after "_" is validated and discarded.
	dailymotionIDPattern   = regexp.MustCompile(`^x[0-9a-z]{4,10}$`)
)
)

func validateMediaURL(raw, suppliedID string) (mediaTarget, error) {
	if raw == "" || len(raw) > 2048 || strings.IndexFunc(raw, func(r rune) bool { return r < 32 }) >= 0 {
		return mediaTarget{}, errors.New("Invalid video URL.")
	}
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.Port() != "" {
		return mediaTarget{}, errors.New("Only valid HTTPS YouTube, Twitch or Dailymotion media URLs are supported.")
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	var target mediaTarget
	switch host {
	case "youtu.be", "youtube.com", "www.youtube.com", "m.youtube.com", "music.youtube.com":
		target, err = validateParsedYouTubeURL(parsed, host)
	case "clips.twitch.tv", "twitch.tv", "www.twitch.tv", "m.twitch.tv":
		target, err = validateParsedTwitchURL(parsed, host)
	case "dailymotion.com", "www.dailymotion.com":
		target, err = validateParsedDailymotionURL(parsed)
	case "vimeo.com", "www.vimeo.com", "reddit.com", "www.reddit.com", "tiktok.com", "www.tiktok.com":
		target, err = validateSupportedGenericURL(parsed, host)
	default:
		err = errors.New("This site is not yet supported by FramePier.")
	}
	if err != nil {
		return mediaTarget{}, err
	}
	if suppliedID != "" && suppliedID != target.ID {
		return mediaTarget{}, errors.New("Media ID does not match the URL.")
	}
	return target, nil
}

func validateParsedYouTubeURL(parsed *url.URL, host string) (mediaTarget, error) {
	var videoID string
	if host == "youtu.be" {
		parts := pathParts(parsed.EscapedPath())
		if len(parts) == 1 {
			videoID, _ = url.PathUnescape(parts[0])
		}
	} else if parsed.Path == "/watch" {
		videoID = parsed.Query().Get("v")
	} else {
		parts := pathParts(parsed.Path)
		if len(parts) == 2 && (parts[0] == "shorts" || parts[0] == "live" || parts[0] == "embed") {
			videoID = parts[1]
		}
	}
	if !videoIDPattern.MatchString(videoID) {
		return mediaTarget{}, errors.New("The URL does not contain a valid YouTube video ID.")
	}
	return mediaTarget{CanonicalURL: "https://www.youtube.com/watch?v=" + videoID, ID: videoID, Source: sourceYouTube}, nil
}

func validateParsedTwitchURL(parsed *url.URL, host string) (mediaTarget, error) {
	parts := pathParts(parsed.EscapedPath())
	for index, part := range parts {
		decoded, err := url.PathUnescape(part)
		if err != nil || decoded != part {
			return mediaTarget{}, errors.New("The Twitch URL contains an invalid path.")
		}
		parts[index] = decoded
	}
	if host == "clips.twitch.tv" {
		if len(parts) == 1 && twitchClipIDPattern.MatchString(parts[0]) {
			return mediaTarget{CanonicalURL: "https://clips.twitch.tv/" + parts[0], ID: parts[0], Source: sourceTwitchClip}, nil
		}
		return mediaTarget{}, errors.New("This Twitch page is not a supported clip or VOD.")
	}
	if len(parts) == 2 && strings.EqualFold(parts[0], "videos") && twitchVODIDPattern.MatchString(parts[1]) {
		return mediaTarget{CanonicalURL: "https://www.twitch.tv/videos/" + parts[1], ID: parts[1], Source: sourceTwitchVOD}, nil
	}
	if len(parts) == 3 && twitchChannelPattern.MatchString(parts[0]) && strings.EqualFold(parts[1], "clip") && twitchClipIDPattern.MatchString(parts[2]) {
		return mediaTarget{CanonicalURL: "https://clips.twitch.tv/" + parts[2], ID: parts[2], Source: sourceTwitchClip}, nil
	}
	if len(parts) == 1 && twitchChannelPattern.MatchString(parts[0]) {
		reserved := map[string]bool{"directory": true, "settings": true, "subscriptions": true, "search": true, "downloads": true, "inventory": true, "wallet": true, "jobs": true, "p": true, "turbo": true, "videos": true}
		if reserved[strings.ToLower(parts[0])] {
			return mediaTarget{}, errors.New("This Twitch page is not a supported clip or VOD.")
		}
		return mediaTarget{}, errors.New("Live Twitch streams are not supported yet. Public Twitch clips and VODs are supported.")
	}
	return mediaTarget{}, errors.New("This Twitch page is not a supported clip or VOD.")
}

// validateParsedDailymotionURL accepts only public single-video pages of the
// form /video/<id> or the legacy /video/<id>_<slug>. Query strings are ignored
// and never forwarded; the canonical URL is rebuilt from the validated ID.
func validateParsedDailymotionURL(parsed *url.URL) (mediaTarget, error) {
	parts := pathParts(parsed.EscapedPath())
	if len(parts) == 2 && parts[0] == "video" {
		id, slug, hasSlug := strings.Cut(parts[1], "_")
		if dailymotionIDPattern.MatchString(id) && (!hasSlug || dailymotionSlugPattern.MatchString(slug)) {
			return mediaTarget{CanonicalURL: "https://www.dailymotion.com/video/" + id, ID: id, Source: sourceDailymotion}, nil
		}
	}
	return mediaTarget{}, errors.New("This Dailymotion page is not a supported public video.")
}

// This is an explicitly constrained first batch, not an arbitrary-URL pass-through.
// It does not create a network sandbox: yt-dlp's approved extractor may still
// contact media/CDN hosts. Broader site support requires a separate network gate.
func validateSupportedGenericURL(parsed *url.URL, host string) (mediaTarget, error) {
	if parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Opaque != "" || parsed.ForceQuery ||
		parsed.RawPath != "" || parsed.EscapedPath() != parsed.Path {
		return mediaTarget{}, errors.New("Use the direct public video URL without extra parameters.")
	}
	var extractor string
	switch host {
	case "vimeo.com", "www.vimeo.com":
		if vimeoVideoPattern.MatchString(parsed.Path) { extractor = "Vimeo" }
	case "reddit.com", "www.reddit.com":
		if redditVideoPattern.MatchString(parsed.Path) { extractor = "Reddit" }
	case "tiktok.com", "www.tiktok.com":
		if tiktokVideoPattern.MatchString(parsed.Path) { extractor = "TikTok" }
	}
	if extractor == "" {
		return mediaTarget{}, errors.New("Open a single public video page on this site, not a feed or profile.")
	}
	return mediaTarget{CanonicalURL: parsed.String(), Source: sourceGeneric, Extractor: extractor}, nil
}

func pathParts(value string) []string {
	trimmed := strings.Trim(value, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func validateYouTubeURL(raw, suppliedID string) (string, string, error) {
	target, err := validateMediaURL(raw, suppliedID)
	if err != nil {
		return "", "", err
	}
	if target.Source != sourceYouTube {
		return "", "", errors.New("Only valid HTTPS YouTube video URLs are supported.")
	}
	return target.CanonicalURL, target.ID, nil
}

func sourceLabel(source mediaSource) string {
	switch source {
	case sourceTwitchClip:
		return "Twitch Clip"
	case sourceTwitchVOD:
		return "Twitch VOD"
	case sourceDailymotion:
		return "Dailymotion"
	case sourceGeneric:
		return "Supported site"
	default:
		return "YouTube"
	}
}
