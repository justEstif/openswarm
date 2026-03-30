package wezterm_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/justEstif/openswarm/internal/pane"
	wezterm "github.com/justEstif/openswarm/internal/pane/wezterm"
)

// skipIfNoWezterm skips the test if `wezterm` is not found in PATH or if we
// are not running inside a WezTerm session ($WEZTERM_PANE unset).
// Spawn and other pane-creating operations require an active WezTerm mux
// connection (the $WEZTERM_PANE env var provides the parent pane context).
func skipIfNoWezterm(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("wezterm"); err != nil {
		t.Skip("wezterm not found in PATH; skipping integration test")
	}
	if os.Getenv("WEZTERM_PANE") == "" {
		t.Skip("$WEZTERM_PANE not set; must run inside a WezTerm session to spawn panes")
	}
}

// TestRegistered verifies the "wezterm" driver was registered via init().
func TestRegistered(t *testing.T) {
	b, err := pane.New("wezterm")
	if err != nil {
		t.Fatalf("pane.New(\"wezterm\") returned error: %v", err)
	}
	if b.Name() != "wezterm" {
		t.Fatalf("Name() = %q, want %q", b.Name(), "wezterm")
	}
}

// TestName verifies Name() returns "wezterm".
func TestName(t *testing.T) {
	b, _ := pane.New("wezterm")
	if got := b.Name(); got != "wezterm" {
		t.Errorf("Name() = %q, want %q", got, "wezterm")
	}
}

// TestParseWeztermList_Valid exercises the JSON parsing with a realistic fixture.
func TestParseWeztermList_Valid(t *testing.T) {
	// Use the package-internal helper via the exported test-hook path.
	// Since parseWeztermList is unexported, we call List() via backend, but
	// we test it through the exported API by invoking the wezterm driver's
	// package-level test helper.  Because the package doesn't export it, we
	// instead validate behaviour through a thin exported wrapper created below.
	// In practice we test via the exported parseWeztermListForTest.
	fixture := `[
		{"window_id":0,"tab_id":0,"pane_id":1,"workspace":"default","title":"bash",
		 "tab_title":"my-tab","is_active":true,"is_zoomed":false},
		{"window_id":0,"tab_id":1,"pane_id":2,"workspace":"default","title":"vim",
		 "tab_title":"","is_active":false,"is_zoomed":false}
	]`

	infos, err := wezterm.ParseWeztermListForTest(fixture)
	if err != nil {
		t.Fatalf("parseWeztermList: unexpected error: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("got %d panes, want 2", len(infos))
	}

	// Pane 1: tab_title present → use tab_title as Name.
	if infos[0].ID != "1" {
		t.Errorf("infos[0].ID = %q, want %q", infos[0].ID, "1")
	}
	if infos[0].Name != "my-tab" {
		t.Errorf("infos[0].Name = %q, want %q", infos[0].Name, "my-tab")
	}
	if !infos[0].Running {
		t.Errorf("infos[0].Running = false, want true")
	}

	// Pane 2: tab_title empty → fall back to title.
	if infos[1].ID != "2" {
		t.Errorf("infos[1].ID = %q, want %q", infos[1].ID, "2")
	}
	if infos[1].Name != "vim" {
		t.Errorf("infos[1].Name = %q, want %q", infos[1].Name, "vim")
	}
}

// TestParseWeztermList_Empty checks that an empty JSON array returns no panes.
func TestParseWeztermList_Empty(t *testing.T) {
	infos, err := wezterm.ParseWeztermListForTest(`[]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(infos) != 0 {
		t.Errorf("got %d panes, want 0", len(infos))
	}
}

// TestParseWeztermList_Invalid checks that malformed JSON returns an error.
func TestParseWeztermList_Invalid(t *testing.T) {
	_, err := wezterm.ParseWeztermListForTest(`not json`)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

// ---- integration tests (skipped unless wezterm is in PATH) ----

// TestSpawnAndClose is an integration smoke-test: spawn a no-op pane, close it.
func TestSpawnAndClose(t *testing.T) {
	skipIfNoWezterm(t)

	b, err := pane.New("wezterm")
	if err != nil {
		t.Fatalf("pane.New: %v", err)
	}

	id, err := b.Spawn("test-pane", "true", nil)
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	if id == "" {
		t.Fatal("Spawn returned empty ID")
	}

	if err := b.Close(id); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestList is an integration smoke-test for List().
func TestList(t *testing.T) {
	skipIfNoWezterm(t)

	b, err := pane.New("wezterm")
	if err != nil {
		t.Fatalf("pane.New: %v", err)
	}
	infos, err := b.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// At minimum there should be the current pane.
	if len(infos) == 0 {
		t.Error("List returned empty slice; expected at least one pane")
	}
}

// TestSubscribeContextCancel verifies Subscribe honours ctx cancellation.
func TestSubscribeContextCancel(t *testing.T) {
	skipIfNoWezterm(t)

	b, err := pane.New("wezterm")
	if err != nil {
		t.Fatalf("pane.New: %v", err)
	}

	id, err := b.Spawn("sub-test", "sleep 60", nil)
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	defer b.Close(id) //nolint:errcheck

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := b.Subscribe(ctx, id)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	// Cancel immediately; channel should drain and close.
	cancel()
	for range ch {
		// drain
	}
	// If we reach here, Subscribe respected the cancellation.
}

// TestErrNotFound verifies that operating on a nonexistent pane yields an error
// (WezTerm returns non-zero exit for unknown pane IDs).
func TestErrNotFound(t *testing.T) {
	skipIfNoWezterm(t)

	b, err := pane.New("wezterm")
	if err != nil {
		t.Fatalf("pane.New: %v", err)
	}
	// Use a large pane ID that almost certainly doesn't exist.
	err = b.Send("999999999", "hello")
	if err == nil {
		t.Error("expected error when sending to non-existent pane, got nil")
	}
}

// TestCloseIdempotent verifies that closing a non-existent pane does not error.
func TestCloseIdempotent(t *testing.T) {
	skipIfNoWezterm(t)

	b, err := pane.New("wezterm")
	if err != nil {
		t.Fatalf("pane.New: %v", err)
	}
	// Close a pane ID that doesn't exist — must not error.
	if err := b.Close("999999999"); err != nil {
		t.Errorf("Close on non-existent pane returned error: %v", err)
	}
}

// TestBackendNotRegisteredDirectly verifies wrong names still fail.
func TestBackendNotRegisteredDirectly(t *testing.T) {
	_, err := pane.New("definitely-not-wezterm")
	if err == nil {
		t.Fatal("expected ErrNoBackend, got nil")
	}
}

// TestSubscribeExitEvent verifies that Subscribe emits Exited=true after the
// pane disappears.
func TestSubscribeExitEvent(t *testing.T) {
	skipIfNoWezterm(t)

	b, err := pane.New("wezterm")
	if err != nil {
		t.Fatalf("pane.New: %v", err)
	}

	// Spawn a pane that exits quickly.
	id, err := b.Spawn("exit-test", "sleep 0.1", nil)
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := b.Subscribe(ctx, id)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	var gotExit bool
	for ev := range ch {
		if ev.Exited {
			gotExit = true
			if ev.Code != -1 {
				t.Errorf("expected Code=-1 (unavailable), got %d", ev.Code)
			}
			break
		}
	}
	if !gotExit {
		t.Error("Subscribe did not emit an exit event before timeout")
	}
}

// Ensure errors package is used for the blank-import test.
var _ = errors.New
