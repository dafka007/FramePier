package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateYouTubeURL(t *testing.T) {
	valid := []string{
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		"https://youtube.com/watch?v=dQw4w9WgXcQ&list=PL123",
		"https://youtu.be/dQw4w9WgXcQ?t=12",
		"https://www.youtube.com/shorts/dQw4w9WgXcQ",
		"https://m.youtube.com/watch?v=dQw4w9WgXcQ",
	}
	for _, value := range valid {
		canonical, id, err := validateYouTubeURL(value, "")
		if err != nil || id != "dQw4w9WgXcQ" || canonical != "https://www.youtube.com/watch?v=dQw4w9WgXcQ" {
			t.Errorf("valid URL rejected: %q (%v)", value, err)
		}
	}
}

func TestRejectUnsafeURLs(t *testing.T) {
	invalid := []string{
		"http://www.youtube.com/watch?v=dQw4w9WgXcQ",
		"javascript:alert(1)",
		"file:///C:/Windows/System32",
		"data:text/html,hello",
		"https://localhost/watch?v=dQw4w9WgXcQ",
		"https://youtube.com.evil.test/watch?v=dQw4w9WgXcQ",
		"https://www.youtube.com/watch?v=;calc.exe",
		"https://www.youtube.com/playlist?list=PL123",
		"https://user:pass@www.youtube.com/watch?v=dQw4w9WgXcQ",
	}
	for _, value := range invalid {
		if _, _, err := validateYouTubeURL(value, ""); err == nil {
			t.Errorf("unsafe URL accepted: %q", value)
		}
	}
}

func TestVideoIDMustMatch(t *testing.T) {
	if _, _, err := validateYouTubeURL("https://youtu.be/dQw4w9WgXcQ", "AAAAAAAAAAA"); err == nil {
		t.Fatal("mismatched video id was accepted")
	}
}

func TestValidateTwitchURLs(t *testing.T) {
	tests := []struct {
		url, id string
		source  mediaSource
	}{
		{"https://clips.twitch.tv/AmusedMildTitanOSkomodo-hl2PV3WlBurF_8uo", "AmusedMildTitanOSkomodo-hl2PV3WlBurF_8uo", sourceTwitchClip},
		{"https://www.twitch.tv/twitch/clip/AmusedMildTitanOSkomodo-hl2PV3WlBurF_8uo", "AmusedMildTitanOSkomodo-hl2PV3WlBurF_8uo", sourceTwitchClip},
		{"https://www.twitch.tv/videos/2865128806?t=1h2m", "2865128806", sourceTwitchVOD},
	}
	for _, test := range tests {
		target, err := validateMediaURL(test.url, test.id)
		if err != nil || target.ID != test.id || target.Source != test.source {
			t.Fatalf("validateMediaURL(%q) = %#v, %v", test.url, target, err)
		}
	}
}

func TestRejectUnsupportedTwitchPages(t *testing.T) {
	unsupported := []string{
		"https://www.twitch.tv/twitch/directory",
		"https://www.twitch.tv/directory/category/games",
		"https://www.twitch.tv/settings/profile",
		"https://www.twitch.tv/subscriptions",
		"https://www.twitch.tv/search?term=test",
		"https://clips.twitch.tv/clip/extra",
		"https://twitch.tv.evil.test/videos/2865128806",
	}
	for _, value := range unsupported {
		if _, err := validateMediaURL(value, ""); err == nil {
			t.Errorf("unsupported Twitch page accepted: %q", value)
		}
	}
	if _, err := validateMediaURL("https://www.twitch.tv/twitch", ""); err == nil || !strings.Contains(err.Error(), "Live Twitch streams") {
		t.Fatalf("live channel error = %v", err)
	}
}

func TestTwitchMetadataIdentity(t *testing.T) {
	clip := mediaTarget{ID: "AmusedMildTitanOSkomodo-hl2PV3WlBurF_8uo", Source: sourceTwitchClip}
	if !metadataMatchesTarget("3238533519", clip.ID, clip) || metadataMatchesTarget("3238533519", "OtherClip", clip) {
		t.Fatal("Twitch clip display ID validation failed")
	}
	vod := mediaTarget{ID: "2865128806", Source: sourceTwitchVOD}
	if !metadataMatchesTarget("v2865128806", "v2865128806", vod) || metadataMatchesTarget("v999", "v999", vod) {
		t.Fatal("Twitch VOD ID validation failed")
	}
}

func TestVerifyOutputRejectsTraversal(t *testing.T) {
	if _, err := verifyOutput(`C:\Users\Example\Downloads\VidDock`, `C:\Windows\System32\calc.exe`); err == nil {
		t.Fatal("escaped output path was accepted")
	}
}

func TestValidateDownloadPath(t *testing.T) {
	invalid := []string{`..\Windows\System32`, `C:\`, `\\server\share`, `relative`, "C:\\Bad\x00Path"}
	for _, value := range invalid {
		if _, err := validateDownloadPath(value); err == nil {
			t.Errorf("unsafe path accepted: %q", value)
		}
	}
}

func TestDecodeRequestRejectsUnknownFieldsAndTypes(t *testing.T) {
	cases := []string{
		`{"id":"test","command":"ping","executable":"calc.exe"}`,
		`{"id":"test","command":"ping","autoOpen":"yes"}`,
		`{"id":"test","command":"run_command"}`,
		`{"id":"; calc.exe","command":"ping"}`,
		`{"id":"test","command":"ping","url":"https://www.youtube.com/watch?v=dQw4w9WgXcQ"}`,
		`{"id":"test","command":"choose_download_folder","downloadPath":"C:\\Windows"}`,
		`{"id":"test","command":"open_download_folder","downloadPath":"C:\\Windows"}`,
		`{"id":"test","command":"open_download_folder","url":"https://example.com"}`,
		`{"id":"test","command":"ping","updateEvent":"reload_requested"}`,
		`{"id":"test","command":"record_update_event","url":"https://www.youtube.com/watch?v=dQw4w9WgXcQ"}`,
	}
	for _, payload := range cases {
		var raw map[string]json.RawMessage
		_ = json.Unmarshal([]byte(payload), &raw)
		if _, err := decodeRequest([]byte(payload), raw); err == nil {
			t.Errorf("malicious message accepted: %s", payload)
		}
	}
}

func TestUpdateDiagnosticCommandValidation(t *testing.T) {
	valid := request{
		Command:        "record_update_event",
		RunningVersion: "0.1.5",
		TargetVersion:  "0.1.6",
		UpdateEvent:    "reload_requested",
		Browser:        "Brave",
	}
	if !validUpdateEvent(valid) {
		t.Fatal("valid update event was rejected")
	}
	invalid := []request{
		{RunningVersion: "0.1.5", TargetVersion: "0.1.6", UpdateEvent: "kill_browser", Browser: "Brave"},
		{RunningVersion: "0.1.5;calc", TargetVersion: "0.1.6", UpdateEvent: "reload_requested", Browser: "Brave"},
		{RunningVersion: "0.1.5", TargetVersion: "0.1.6", UpdateEvent: "reload_requested", Browser: "Unknown"},
	}
	for _, candidate := range invalid {
		if validUpdateEvent(candidate) {
			t.Fatalf("invalid update event was accepted: %#v", candidate)
		}
	}
}

func TestFolderPickerCommandAcceptsNoFilesystemInput(t *testing.T) {
	payload := []byte(`{"id":"choose_test","command":"choose_download_folder"}`)
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(payload, &raw)
	request, err := decodeRequest(payload, raw)
	if err != nil || request.Command != "choose_download_folder" {
		t.Fatalf("fixed folder picker command rejected: %#v, %v", request, err)
	}
}

func TestReadNativeMessageRejectsOversize(t *testing.T) {
	header := []byte{1, 0, 16, 0}
	if _, err := readNativeMessage(strings.NewReader(string(header))); err == nil {
		t.Fatal("oversized Native Messaging frame was accepted")
	}
}

func TestChoicesRejectShellMetacharacters(t *testing.T) {
	attacks := []string{`; calc.exe`, `&& calc.exe`, `| calc.exe`, `$(calc.exe)`, "`calc.exe`", `1080 --exec calc.exe`}
	for _, attack := range attacks {
		if validChoice(attack, []string{"best", "1080", "720"}) {
			t.Errorf("unsafe format accepted: %q", attack)
		}
	}
}
