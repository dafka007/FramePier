package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstalledPackageVersionUsesExternalFixedMetadata(t *testing.T) {
	root := t.TempDir()
	metadata := []byte(`{"product":"VidDock","packageVersion":"0.1.6","extensionId":"kclnooibijmfenaldmpkffdbednfipkk"}`)
	if err := os.WriteFile(filepath.Join(root, "package-version.json"), metadata, 0600); err != nil {
		t.Fatal(err)
	}
	if got := installedPackageVersion(root); got != "0.1.6" {
		t.Fatalf("installedPackageVersion() = %q", got)
	}
}

func TestInstalledPackageVersionRejectsUntrustedMetadata(t *testing.T) {
	root := t.TempDir()
	cases := []string{
		`{"product":"Other","packageVersion":"0.1.6","extensionId":"kclnooibijmfenaldmpkffdbednfipkk"}`,
		`{"product":"VidDock","packageVersion":"0.1.6","extensionId":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`,
		`{"product":"VidDock","packageVersion":"1.2.3;reload","extensionId":"kclnooibijmfenaldmpkffdbednfipkk"}`,
		`{"product":"VidDock","packageVersion":"0.1.6","extensionId":"kclnooibijmfenaldmpkffdbednfipkk","extra":true}`,
		`{"product":"VidDock","packageVersion":"0.1.6","extensionId":"kclnooibijmfenaldmpkffdbednfipkk"}{}`,
	}
	for _, metadata := range cases {
		if err := os.WriteFile(filepath.Join(root, "package-version.json"), []byte(metadata), 0600); err != nil {
			t.Fatal(err)
		}
		if got := installedPackageVersion(root); got != "" {
			t.Fatalf("installedPackageVersion(%s) = %q", metadata, got)
		}
	}
}

func TestPrepareDownloadPathCreatesAndChecksWritableDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Unicode Δ תיקייה", "downloads")
	got, err := prepareDownloadPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Clean(path) {
		t.Fatalf("prepareDownloadPath() = %q, want %q", got, filepath.Clean(path))
	}
	if info, err := os.Stat(got); err != nil || !info.IsDir() {
		t.Fatalf("prepared directory is unavailable: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(got, ".viddock-write-test-*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("write probe was not cleaned up: %v, %v", matches, err)
	}
}

func TestFolderPickerCancelHRESULT(t *testing.T) {
	if failedHRESULT(0) || !failedHRESULT(uintptr(hresultCancelled)) || uint32(hresultCancelled) != hresultCancelled {
		t.Fatal("folder picker HRESULT classification is incorrect")
	}
}

func TestFolderPickerOwnerExecutableAllowlist(t *testing.T) {
	for _, name := range []string{"brave.exe", "CHROME.EXE", "msedge.exe"} {
		if !validChromiumExecutable(name) {
			t.Fatalf("supported browser executable rejected: %s", name)
		}
	}
	for _, name := range []string{"explorer.exe", "Code.exe", "VidDockHelper.exe"} {
		if validChromiumExecutable(name) {
			t.Fatalf("unrelated owner executable accepted: %s", name)
		}
	}
}

func TestCurrentProcessSharesItsWindowsSession(t *testing.T) {
	if !sameWindowsSession(uint32(os.Getpid())) {
		t.Fatal("current process was not recognized in its own Windows session")
	}
}
