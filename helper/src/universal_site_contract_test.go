package main

import "testing"

// First-slice contract: the user explicitly opens a public video page and
// clicks FramePier. A different supported site must reach the yt-dlp metadata
// probe rather than being refused solely because it is not one of our three
// original integrations. These tests do not perform network requests.
func TestUniversalSiteAcceptsPublicHTTPSPages(t *testing.T) {
    urls := []string{
        "https://vimeo.com/123456789",
        "https://www.reddit.com/r/videos/comments/abc123/example/",
        "https://www.tiktok.com/@creator/video/123456789",
    }
    for _, raw := range urls {
        t.Run(raw, func(t *testing.T) {
            got, err := validateMediaURL(raw, "")
            if err != nil {
                t.Fatalf("public HTTPS page rejected: %v", err)
            }
            if got.Source != mediaSource("generic") {
                t.Fatalf("source = %q; want generic", got.Source)
            }
            if got.CanonicalURL != raw {
                t.Fatalf("canonical URL = %q; want %q", got.CanonicalURL, raw)
            }
            // Generic sites have no trusted site-specific ID until yt-dlp
            // returns validated metadata.
            if got.ID != "" {
                t.Fatalf("generic ID must not be guessed: %q", got.ID)
            }
        })
    }
}

func TestUniversalSiteRejectsUnsafeEntryURLs(t *testing.T) {
    cases := []string{
        "http://vimeo.com/123456789",
        "https://localhost/watch",
        "https://nas.local/watch",
        "https://intranet/watch",
        "https://metadata.google.internal/latest/meta-data/",
        "https://127.0.0.1/watch",
        "https://10.0.0.1/watch",
        "https://172.16.1.1/watch",
        "https://192.168.1.1/watch",
        "https://169.254.169.254/latest/meta-data/",
        "https://[::1]/watch",
        "https://[fc00::1]/watch",
        "https://[fe80::1]/watch",
        "https://user:password@vimeo.com/123456789",
        "https://vimeo.com:8443/123456789",
        "file:///C:/Windows/System32/",
        "https://vimeo.com/\nInjected: header",
    }
    for _, raw := range cases {
        t.Run(raw, func(t *testing.T) {
            if got, err := validateMediaURL(raw, ""); err == nil {
                t.Fatalf("unsafe entry URL accepted: %#v", got)
            }
        })
    }
}

func TestUniversalSiteDoesNotTrustArbitrarySuppliedVideoID(t *testing.T) {
    if got, err := validateMediaURL("https://vimeo.com/123456789", "forged-id"); err == nil {
        t.Fatalf("untrusted supplied ID accepted: %#v", got)
    }
}
