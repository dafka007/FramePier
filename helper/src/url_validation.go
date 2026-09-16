package main

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

type mediaSource string

const (
	sourceYouTube    mediaSource = "youtube"
	sourceTwitchClip mediaSource = "twitch_clip"
	sourceTwitchVOD  mediaSource = "twitch_vod"
)

type mediaTarget struct {
	CanonicalURL string
	ID           string
	Source       mediaSource
}

var (
	twitchClipIDPattern  = regexp.MustCompile(`^[A-Za-z0-9_-]{6,100}$`)
	twitchChannelPattern = regexp.MustCompile(`^[A-Za-z0-9_]{1,25}$`)
	twitchVODIDPattern   = regexp.MustCompile(`^[0-9]{1,20}$`)
)

func validateMediaURL(raw, suppliedID string) (mediaTarget, error) {
	if raw == "" || len(raw) > 2048 || strings.IndexFunc(raw, func(r rune) bool { return r < 32 }) >= 0 {
		return mediaTarget{}, errors.New("Invalid video URL.")
	}
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.Port() != "" {
		return mediaTarget{}, errors.New("Only valid HTTPS YouTube or Twitch media URLs are supported.")
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	var target mediaTarget
	switch host {
	case "youtu.be", "youtube.com", "www.youtube.com", "m.youtube.com", "music.youtube.com":
		target, err = validateParsedYouTubeURL(parsed, host)
	case "clips.twitch.tv", "twitch.tv", "www.twitch.tv", "m.twitch.tv":
		target, err = validateParsedTwitchURL(parsed, host)
	default:
		err = errors.New("Only official YouTube and Twitch domains are supported.")
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
	default:
		return "YouTube"
	}
}
