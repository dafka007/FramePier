package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type urlFixture struct {
	URL       string `json:"url"`
	Accept    bool   `json:"accept"`
	Source    string `json:"source"`
	ID        string `json:"id"`
	Canonical string `json:"canonical"`
	Why       string `json:"why"`
}

// loadURLFixtures reads the fixture file shared with
// scripts/url-fixtures.test.mjs, which runs the same cases through the
// extension's parseMediaUrl.
func loadURLFixtures(t *testing.T) []urlFixture {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "scripts", "fixtures", "url-cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []urlFixture
	if err := json.Unmarshal(data, &fixtures); err != nil || len(fixtures) == 0 {
		t.Fatalf("invalid fixtures: %v", err)
	}
	return fixtures
}

func TestSharedURLFixturesAgreeWithTheHelperValidator(t *testing.T) {
	sawDailymotionAccept, sawDailymotionReject := false, false
	for _, fixture := range loadURLFixtures(t) {
		target, err := validateMediaURL(fixture.URL, "")
		if !fixture.Accept {
			if err == nil {
				t.Errorf("accepted %q (%s) as %+v", fixture.URL, fixture.Why, target)
			}
			if strings.Contains(fixture.URL, "dailymotion") {
				sawDailymotionReject = true
			}
			continue
		}
		if err != nil {
			t.Errorf("rejected %q: %v", fixture.URL, err)
			continue
		}
		if string(target.Source) != fixture.Source || target.ID != fixture.ID || target.CanonicalURL != fixture.Canonical {
			t.Errorf("%q = %+v, want source=%s id=%s canonical=%s", fixture.URL, target, fixture.Source, fixture.ID, fixture.Canonical)
		}
		if _, err := validateMediaURL(fixture.URL, fixture.ID); err != nil {
			t.Errorf("matching supplied ID rejected for %q: %v", fixture.URL, err)
		}
		if _, err := validateMediaURL(fixture.URL, "zzzzzzzz"); err == nil {
			t.Errorf("mismatched supplied ID accepted for %q", fixture.URL)
		}
		if fixture.Source == "dailymotion" {
			sawDailymotionAccept = true
		}
	}
	if !sawDailymotionAccept || !sawDailymotionReject {
		t.Fatal("fixtures must cover accepted and rejected Dailymotion URLs")
	}
}

func TestDailymotionURLFormsOnlyTheHelperCanSee(t *testing.T) {
	invalid := []string{
		"https://www.dailymotion.com:443/video/xmo8jh",
		"https://www.dailymotion.com/video/xmo8jh#t=10",
		"https://www.dailymotion.com/video/xmo8jh\x00",
		"https://www.dailymotion.com/video/xmo8jh\n",
		"https://www.dailymotion.com/video/xmo8jh?" + strings.Repeat("a", 2100),
		"",
		"//www.dailymotion.com/video/xmo8jh",
		"www.dailymotion.com/video/xmo8jh",
		"HTTP://www.dailymotion.com/video/xmo8jh",
	}
	for _, value := range invalid {
		if _, err := validateMediaURL(value, ""); err == nil {
			t.Errorf("accepted %q", value)
		}
	}
	// A trailing dot names the same host; the existing allowlist behaviour for
	// YouTube and Twitch also trims it.
	target, err := validateMediaURL("https://www.dailymotion.com./video/xmo8jh", "")
	if err != nil || target.CanonicalURL != "https://www.dailymotion.com/video/xmo8jh" {
		t.Fatalf("trailing-dot host = %+v, %v", target, err)
	}
}

func TestDailymotionDoesNotLeakIntoTheYouTubeOnlyHelper(t *testing.T) {
	if _, _, err := validateYouTubeURL("https://www.dailymotion.com/video/xmo8jh", ""); err == nil {
		t.Fatal("validateYouTubeURL accepted a Dailymotion URL")
	}
}

func TestDailymotionCanonicalURLDropsUntrustedComponents(t *testing.T) {
	target, err := validateMediaURL("https://www.dailymotion.com/video/x5kesuj_some-slug?playlist=x1&autoplay=1&url=https://evil.test/", "x5kesuj")
	if err != nil {
		t.Fatal(err)
	}
	if target.CanonicalURL != "https://www.dailymotion.com/video/x5kesuj" || target.ID != "x5kesuj" || target.Source != sourceDailymotion {
		t.Fatalf("target = %+v", target)
	}
	if strings.ContainsAny(strings.TrimPrefix(target.CanonicalURL, "https://"), "?#&=_@:") {
		t.Fatalf("canonical URL carries extra components: %s", target.CanonicalURL)
	}
	if sourceLabel(sourceDailymotion) != "Dailymotion" {
		t.Fatalf("label = %q", sourceLabel(sourceDailymotion))
	}
}

func TestExistingSourcesKeepTheirLabelsAndCanonicalForms(t *testing.T) {
	cases := map[mediaSource]string{sourceYouTube: "YouTube", sourceTwitchClip: "Twitch Clip", sourceTwitchVOD: "Twitch VOD"}
	for source, want := range cases {
		if got := sourceLabel(source); got != want {
			t.Errorf("%s label = %q, want %q", source, got, want)
		}
	}
}
