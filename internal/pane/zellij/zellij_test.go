package zellij

import (
	"encoding/json"
	"os/exec"
	"testing"

	"github.com/justEstif/openswarm/internal/pane"
)

// zellijAvailable returns true if the zellij binary is in PATH.
func zellijAvailable() bool {
	_, err := exec.LookPath("zellij")
	return err == nil
}

// TestName verifies that ZellijBackend.Name() returns "zellij".
func TestName(t *testing.T) {
	b := &ZellijBackend{}
	if got := b.Name(); got != "zellij" {
		t.Errorf("Name() = %q; want %q", got, "zellij")
	}
}

// TestPaneInt verifies that paneInt strips the "terminal_" prefix correctly.
func TestPaneInt(t *testing.T) {
	tests := []struct {
		id   pane.PaneID
		want string
	}{
		{pane.PaneID("terminal_5"), "5"},
		{pane.PaneID("terminal_0"), "0"},
		{pane.PaneID("terminal_123"), "123"},
		// Bare integer (no prefix) — should pass through unchanged.
		{pane.PaneID("42"), "42"},
		// Empty string — no-op.
		{pane.PaneID(""), ""},
	}

	for _, tc := range tests {
		t.Run(string(tc.id), func(t *testing.T) {
			if got := paneInt(tc.id); got != tc.want {
				t.Errorf("paneInt(%q) = %q; want %q", tc.id, got, tc.want)
			}
		})
	}
}

// TestRegistration verifies that the init() function registered the "zellij" backend.
func TestRegistration(t *testing.T) {
	b, err := pane.New("zellij")
	if err != nil {
		t.Fatalf("pane.New(\"zellij\") error: %v", err)
	}
	if b.Name() != "zellij" {
		t.Errorf("registered backend Name() = %q; want %q", b.Name(), "zellij")
	}
}

// TestParsePaneList unit-tests the JSON parsing logic for List() using a hardcoded fixture.
// This test does not require a real Zellij session.
func TestParsePaneList(t *testing.T) {
	fixture := `[
		{
			"id": 1,
			"is_plugin": false,
			"is_focused": true,
			"title": "bash",
			"exited": false,
			"exit_status": null
		},
		{
			"id": 2,
			"is_plugin": true,
			"is_focused": false,
			"title": "status-bar",
			"exited": false,
			"exit_status": null
		},
		{
			"id": 3,
			"is_plugin": false,
			"is_focused": false,
			"title": "my-worker",
			"exited": true,
			"exit_status": 0
		},
		{
			"id": 4,
			"is_plugin": false,
			"is_focused": false,
			"title": "failed-task",
			"exited": true,
			"exit_status": 1
		}
	]`

	infos, err := parsePaneList([]byte(fixture))
	if err != nil {
		t.Fatalf("parsePaneList error: %v", err)
	}

	// Plugin pane (id=2) should be filtered out.
	if len(infos) != 3 {
		t.Fatalf("expected 3 panes (plugins filtered), got %d", len(infos))
	}

	// Pane 1: still running (exit_status = null).
	if infos[0].ID != pane.PaneID("terminal_1") {
		t.Errorf("infos[0].ID = %q; want %q", infos[0].ID, "terminal_1")
	}
	if infos[0].Name != "bash" {
		t.Errorf("infos[0].Name = %q; want %q", infos[0].Name, "bash")
	}
	if !infos[0].Running {
		t.Errorf("infos[0].Running = false; want true (exit_status is null)")
	}

	// Pane 3: exited with code 0 — Running should be false.
	if infos[1].ID != pane.PaneID("terminal_3") {
		t.Errorf("infos[1].ID = %q; want %q", infos[1].ID, "terminal_3")
	}
	if infos[1].Running {
		t.Errorf("infos[1].Running = true; want false (exit_status = 0)")
	}

	// Pane 4: exited with code 1 — Running should be false.
	if infos[2].ID != pane.PaneID("terminal_4") {
		t.Errorf("infos[2].ID = %q; want %q", infos[2].ID, "terminal_4")
	}
	if infos[2].Running {
		t.Errorf("infos[2].Running = true; want false (exit_status = 1)")
	}
}

// TestParsePaneListEmpty verifies that an empty JSON array returns no panes without error.
func TestParsePaneListEmpty(t *testing.T) {
	infos, err := parsePaneList([]byte(`[]`))
	if err != nil {
		t.Fatalf("parsePaneList([]) error: %v", err)
	}
	if len(infos) != 0 {
		t.Errorf("expected 0 panes, got %d", len(infos))
	}
}

// TestParsePaneListInvalidJSON verifies that malformed JSON returns an error.
func TestParsePaneListInvalidJSON(t *testing.T) {
	_, err := parsePaneList([]byte(`{not valid json`))
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

// TestParseSubscribeEvent_PaneUpdate tests parsing a pane_update NDJSON line.
func TestParseSubscribeEvent_PaneUpdate(t *testing.T) {
	line := []byte(`{"event":"pane_update","pane_id":"terminal_1","viewport":["line1","line2"],"is_initial":true}`)
	evt := parseSubscribeEvent(pane.PaneID("terminal_1"), line)

	if evt.PaneID != pane.PaneID("terminal_1") {
		t.Errorf("PaneID = %q; want %q", evt.PaneID, "terminal_1")
	}
	if evt.Text != "line1\nline2" {
		t.Errorf("Text = %q; want %q", evt.Text, "line1\nline2")
	}
	if evt.Exited {
		t.Error("Exited = true; want false for pane_update")
	}
}

// TestParseSubscribeEvent_PaneClosed tests parsing a pane_closed NDJSON line.
func TestParseSubscribeEvent_PaneClosed(t *testing.T) {
	line := []byte(`{"event":"pane_closed","pane_id":"terminal_1"}`)
	evt := parseSubscribeEvent(pane.PaneID("terminal_1"), line)

	if !evt.Exited {
		t.Error("Exited = false; want true for pane_closed event")
	}
}

// TestParseSubscribeEvent_WithExitCode tests parsing a pane_update with exited+exit_code.
func TestParseSubscribeEvent_WithExitCode(t *testing.T) {
	code := 2
	raw := zellijSubscribeEvent{
		Event:    "pane_update",
		PaneID:   "terminal_3",
		Viewport: []string{"done"},
		Exited:   true,
		ExitCode: &code,
	}
	data, _ := json.Marshal(raw)
	evt := parseSubscribeEvent(pane.PaneID("terminal_3"), data)

	if !evt.Exited {
		t.Error("Exited = false; want true")
	}
	if evt.Code != 2 {
		t.Errorf("Code = %d; want 2", evt.Code)
	}
}

// TestParseSubscribeEvent_Unparseable verifies that unparseable lines are returned as raw text.
func TestParseSubscribeEvent_Unparseable(t *testing.T) {
	line := []byte("not json at all")
	evt := parseSubscribeEvent(pane.PaneID("terminal_1"), line)

	if evt.Text != "not json at all" {
		t.Errorf("Text = %q; want raw line text", evt.Text)
	}
	if evt.Exited {
		t.Error("Exited = true; should be false for raw text fallback")
	}
}

// TestBuildEnvCmd verifies the env command construction helper.
func TestBuildEnvCmd(t *testing.T) {
	// No env vars — should return cmd unchanged.
	got := buildEnvCmd("go build ./...", nil)
	if got != "go build ./..." {
		t.Errorf("buildEnvCmd with nil env = %q; want %q", got, "go build ./...")
	}

	got = buildEnvCmd("go build ./...", map[string]string{})
	if got != "go build ./..." {
		t.Errorf("buildEnvCmd with empty env = %q; want %q", got, "go build ./...")
	}

	// Single env var.
	got = buildEnvCmd("myapp", map[string]string{"FOO": "bar"})
	if got != "env FOO=bar myapp" {
		t.Errorf("buildEnvCmd single var = %q; want %q", got, "env FOO=bar myapp")
	}
}

// ----- Integration tests (skipped if zellij not in PATH) -----

// TestSpawnRequiresZellij is a placeholder that ensures integration tests skip gracefully.
func TestSpawnRequiresZellij(t *testing.T) {
	if !zellijAvailable() {
		t.Skip("zellij not found in PATH")
	}
	// Integration test body would go here (requires an active Zellij session).
	t.Skip("integration test requires active Zellij session — skipping in CI")
}
