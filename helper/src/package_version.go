package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

var packageVersionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

func installedPackageVersion(root string) string {
	data, err := os.ReadFile(filepath.Join(root, "package-version.json"))
	if err != nil {
		return ""
	}
	var metadata struct {
		Product        string `json:"product"`
		PackageVersion string `json:"packageVersion"`
		ExtensionID    string `json:"extensionId"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&metadata) != nil || decoder.Decode(&struct{}{}) != io.EOF || metadata.Product != "VidDock" || metadata.ExtensionID != "kclnooibijmfenaldmpkffdbednfipkk" || !packageVersionPattern.MatchString(metadata.PackageVersion) {
		return ""
	}
	return metadata.PackageVersion
}
