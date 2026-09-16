package main

import (
	"context"
	"testing"
)

func TestHiddenCommandPreservesPipesAndUsesNoWindow(t *testing.T) {
	cmd := newHiddenCommandContext(context.Background(), "cmd.exe", "/d", "/c", "echo", "ok")
	if cmd.SysProcAttr == nil || cmd.SysProcAttr.CreationFlags&createNoWindow == 0 || !cmd.SysProcAttr.HideWindow {
		t.Fatalf("hidden command has incorrect process attributes: %#v", cmd.SysProcAttr)
	}
	output, err := cmd.Output()
	if err != nil || string(output) != "ok\r\n" {
		t.Fatalf("hidden command output = %q, %v", output, err)
	}
}
