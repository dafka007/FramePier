package main

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const nativeHostName = "com.viddock.helper"

type browserIntegration struct {
	Name                 string
	RegistryKey          string
	ExtensionsURL        string
	ExecutableCandidates []string
}

type nativeHostManifest struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Path           string   `json:"path"`
	Type           string   `json:"type"`
	AllowedOrigins []string `json:"allowed_origins"`
}

func browserIntegrations() []browserIntegration {
	programFiles, programFilesX86, local := os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), localAppData()
	return []browserIntegration{
		{Name: "Google Chrome", RegistryKey: `HKCU\Software\Google\Chrome\NativeMessagingHosts\` + nativeHostName, ExtensionsURL: "chrome://extensions/", ExecutableCandidates: []string{filepath.Join(programFiles, `Google\Chrome\Application\chrome.exe`), filepath.Join(programFilesX86, `Google\Chrome\Application\chrome.exe`), filepath.Join(local, `Google\Chrome\Application\chrome.exe`)}},
		{Name: "Brave Browser", RegistryKey: `HKCU\Software\BraveSoftware\Brave-Browser\NativeMessagingHosts\` + nativeHostName, ExtensionsURL: "brave://extensions/", ExecutableCandidates: []string{filepath.Join(programFiles, `BraveSoftware\Brave-Browser\Application\brave.exe`), filepath.Join(programFilesX86, `BraveSoftware\Brave-Browser\Application\brave.exe`), filepath.Join(local, `BraveSoftware\Brave-Browser\Application\brave.exe`)}},
		{Name: "Microsoft Edge", RegistryKey: `HKCU\Software\Microsoft\Edge\NativeMessagingHosts\` + nativeHostName, ExtensionsURL: "edge://extensions/", ExecutableCandidates: []string{filepath.Join(programFiles, `Microsoft\Edge\Application\msedge.exe`), filepath.Join(programFilesX86, `Microsoft\Edge\Application\msedge.exe`), filepath.Join(local, `Microsoft\Edge\Application\msedge.exe`)}},
	}
}

func detectedBrowserExecutable(browser browserIntegration) string {
	for _, candidate := range browser.ExecutableCandidates {
		if candidate != "" {
			if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
				return candidate
			}
		}
	}
	return ""
}

func manifestPath(root string) string {
	return filepath.Join(root, "native-host", nativeHostName+".json")
}

func writeNativeHostManifest(root string) error {
	helper := filepath.Join(root, "FramePierHelper.exe")
	if info, err := os.Stat(helper); err != nil || !info.Mode().IsRegular() {
		return errors.New("FramePierHelper.exe is missing")
	}
	manifest := nativeHostManifest{Name: nativeHostName, Description: "FramePier local video download helper", Path: helper, Type: "stdio", AllowedOrigins: []string{extensionOrigin}}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(manifestPath(root))
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	temp := manifestPath(root) + ".tmp"
	if err := os.WriteFile(temp, append(data, '\n'), 0600); err != nil {
		return err
	}
	if err := replaceFile(temp, manifestPath(root)); err != nil {
		_ = os.Remove(temp)
		return err
	}
	return nil
}

func validateNativeHostManifest(root string) error {
	data, err := os.ReadFile(manifestPath(root))
	if err != nil {
		return fmt.Errorf("native host manifest missing: %w", err)
	}
	var manifest nativeHostManifest
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return fmt.Errorf("native host manifest invalid: %w", err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("native host manifest contains trailing data")
	}
	if manifest.Name != nativeHostName || manifest.Type != "stdio" {
		return errors.New("native host manifest identity is invalid")
	}
	expectedHelper := filepath.Clean(filepath.Join(root, "FramePierHelper.exe"))
	if !filepath.IsAbs(manifest.Path) || !strings.EqualFold(filepath.Clean(manifest.Path), expectedHelper) {
		return errors.New("native host manifest helper path is invalid")
	}
	if len(manifest.AllowedOrigins) != 1 || manifest.AllowedOrigins[0] != extensionOrigin {
		return errors.New("native host manifest extension origin is invalid")
	}
	if info, err := os.Stat(expectedHelper); err != nil || !info.Mode().IsRegular() {
		return errors.New("native host manifest helper is missing")
	}
	return nil
}

func updateRegistry(browser browserIntegration, manifest string, install bool) error {
	for _, view := range []string{"/reg:32", "/reg:64"} {
		if install {
			output, err := newHiddenCommand("reg.exe", "add", browser.RegistryKey, "/ve", "/t", "REG_SZ", "/d", manifest, "/f", view).CombinedOutput()
			if err != nil {
				return fmt.Errorf("%s %s registration failed: %s", browser.Name, view, strings.TrimSpace(string(output)))
			}
			output, err = newHiddenCommand("reg.exe", "query", browser.RegistryKey, "/ve", view).CombinedOutput()
			if err != nil || !strings.Contains(strings.ToLower(string(output)), strings.ToLower(manifest)) {
				return fmt.Errorf("%s %s registration validation failed", browser.Name, view)
			}
		} else {
			_ = newHiddenCommand("reg.exe", "delete", browser.RegistryKey, "/f", view).Run()
		}
	}
	return nil
}

func manageRegistration(root string, install bool) error {
	if install {
		if err := writeNativeHostManifest(root); err != nil {
			return err
		}
		if err := validateNativeHostManifest(root); err != nil {
			return err
		}
	}
	var failures []string
	for _, browser := range browserIntegrations() {
		if install && detectedBrowserExecutable(browser) == "" {
			continue
		}
		if err := updateRegistry(browser, manifestPath(root), install); err != nil {
			failures = append(failures, err.Error())
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("registry update failed: %s", strings.Join(failures, "; "))
	}
	if install {
		// Only reopen browser launches after the manifest and registry have
		// been repaired and verified, not while they are being replaced.
		if err := resumeInstalledHelpers(); err != nil {
			return fmt.Errorf("resume native helper: %w", err)
		}
		if err := nativeSelfTest(root); err != nil {
			return fmt.Errorf("native helper ping failed: %w", err)
		}
	}
	return nil
}

func nativeSelfTest(root string) error {
	cmd := newHiddenCommand(filepath.Join(root, "FramePierHelper.exe"), extensionOrigin)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	defer func() {
		_ = stdin.Close()
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	payload := []byte(`{"id":"post_install_ping","command":"ping"}`)
	if err := binary.Write(stdin, binary.LittleEndian, uint32(len(payload))); err != nil {
		return err
	}
	if _, err := stdin.Write(payload); err != nil {
		return err
	}
	result := make(chan error, 1)
	go func() {
		body, readErr := readNativeMessage(bufio.NewReader(stdout))
		if readErr == nil {
			var response struct {
				OK      bool   `json:"ok"`
				Version string `json:"version"`
			}
			if json.Unmarshal(body, &response) != nil || !response.OK || response.Version != version {
				readErr = errors.New("invalid ping response")
			}
		}
		result <- readErr
	}()
	var testErr error
	select {
	case testErr = <-result:
	case <-time.After(10 * time.Second):
		testErr = errors.New("ping timed out")
	}
	_ = stdin.Close()
	if testErr != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	_ = cmd.Wait()
	return testErr
}

func browserIntegrationSummary(root string) string {
	var detected []string
	for _, browser := range browserIntegrations() {
		if detectedBrowserExecutable(browser) != "" {
			detected = append(detected, browser.Name)
		}
	}
	if len(detected) == 0 {
		return "No supported browser was detected. Install Chrome, Brave, or Edge, then run Repair Browser Integration again."
	}
	packageStatus := "The installed extension package could not be version-checked."
	if packageVersion := installedPackageVersion(root); packageVersion != "" {
		packageStatus = "Installed extension package: " + packageVersion + "."
	}
	return "Native Messaging was repaired and validated for: " + strings.Join(detected, ", ") + ".\n\nThe helper passed its Native Messaging ping. " + packageStatus + " If an open browser still reports an older extension version, restart it once; Remove/Load unpacked is not required."
}

func openBrowserSetup(root string) {
	_ = openWindowsFolder(filepath.Join(root, "Extension"))
	for _, browser := range browserIntegrations() {
		if executable := detectedBrowserExecutable(browser); executable != "" {
			_ = newHiddenCommand(executable, browser.ExtensionsURL).Start()
		}
	}
}
