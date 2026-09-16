package main

import (
	"bytes"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"
)

func TestFolderLaunchValidatesDirectoryAndReportsFailure(t *testing.T) {
	var logs bytes.Buffer
	a := &app{logger: log.New(&logs, "", 0)}
	folder := filepath.Join(t.TempDir(), "My videos (עברית 日本語)")
	called := ""
	err := a.launchFixedFolderWith(folder, "open_download_folder", func(path string) error {
		called = path
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			t.Fatal("directory not prepared before launch")
		}
		return errors.New("simulated Windows access denied")
	})
	if err == nil || called != folder || !strings.Contains(logs.String(), "folder_open_failed") || !strings.Contains(logs.String(), "simulated Windows access denied") {
		t.Fatalf("error was lost: called=%q err=%v log=%s", called, err, logs.String())
	}
	file := filepath.Join(t.TempDir(), "not-a-folder.mp4")
	if err := os.WriteFile(file, []byte("media"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{file, `relative`, `C:\`, `\\server\share`, `C:\safe\..\Windows`, "C:\\bad\x00path"} {
		if err := a.launchFixedFolderWith(path, "open_download_folder", func(string) error {
			t.Fatalf("unsafe/non-directory target reached shell: %q", path)
			return nil
		}); err == nil {
			t.Errorf("accepted %q", path)
		}
	}
}

func TestFolderLaunchRechecksReparsePoint(t *testing.T) {
	var logs bytes.Buffer
	a := &app{logger: log.New(&logs, "", 0)}
	root := t.TempDir()
	link := filepath.Join(root, "redirect")
	if err := os.Symlink(root, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := a.launchFixedFolderWith(link, "open_download_folder", func(string) error {
		t.Fatal("reparse target reached shell")
		return nil
	}); err == nil {
		t.Fatal("reparse path accepted")
	}
}

func TestShellExecuteInfoWindowsX64Layout(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("Windows x64 target")
	}
	var info shellExecuteInfo
	if unsafe.Sizeof(info) != 112 || unsafe.Offsetof(info.Show) != 48 || unsafe.Offsetof(info.Process) != 104 {
		t.Fatalf("invalid Windows ABI layout: size=%d show=%d process=%d", unsafe.Sizeof(info), unsafe.Offsetof(info.Show), unsafe.Offsetof(info.Process))
	}
}
