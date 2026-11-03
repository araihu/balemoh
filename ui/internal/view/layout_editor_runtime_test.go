package view

import (
	"os/exec"
	"testing"
)

func TestLayoutDraftStateMachine(t *testing.T) {
	node, e := exec.LookPath("node")
	if e != nil {
		t.Skip("node unavailable")
	}
	command := exec.CommandContext(t.Context(), node, "--test", "static/layout-editor.test.cjs")
	if out, e := command.CombinedOutput(); e != nil {
		t.Fatalf("layout draft tests: %v\n%s", e, out)
	}
}
