package ghostty_test

import (
	"context"
	"errors"
	"testing"

	"github.com/justEstif/openswarm/internal/pane"
	_ "github.com/justEstif/openswarm/internal/pane/ghostty"
)

// TestRegistered verifies the "ghostty" driver was registered via init().
func TestRegistered(t *testing.T) {
	b, err := pane.New("ghostty")
	if err != nil {
		t.Fatalf("pane.New(\"ghostty\") returned error: %v", err)
	}
	if b == nil {
		t.Fatal("pane.New returned nil backend")
	}
}

// TestName verifies Name() returns "ghostty".
func TestName(t *testing.T) {
	b, _ := pane.New("ghostty")
	if got := b.Name(); got != "ghostty" {
		t.Errorf("Name() = %q, want %q", got, "ghostty")
	}
}

// TestSpawnNotSupported verifies Spawn returns ErrNotSupported.
func TestSpawnNotSupported(t *testing.T) {
	b, _ := pane.New("ghostty")
	_, err := b.Spawn("test", "echo hi", nil)
	assertNotSupported(t, "Spawn", err)
}

// TestSendNotSupported verifies Send returns ErrNotSupported.
func TestSendNotSupported(t *testing.T) {
	b, _ := pane.New("ghostty")
	err := b.Send("1", "hello")
	assertNotSupported(t, "Send", err)
}

// TestCaptureNotSupported verifies Capture returns ErrNotSupported.
func TestCaptureNotSupported(t *testing.T) {
	b, _ := pane.New("ghostty")
	_, err := b.Capture("1")
	assertNotSupported(t, "Capture", err)
}

// TestSubscribeNotSupported verifies Subscribe returns ErrNotSupported.
func TestSubscribeNotSupported(t *testing.T) {
	b, _ := pane.New("ghostty")
	ctx := context.Background()
	ch, err := b.Subscribe(ctx, "1")
	assertNotSupported(t, "Subscribe", err)
	if ch != nil {
		t.Error("Subscribe returned non-nil channel; expected nil")
	}
}

// TestListNotSupported verifies List returns ErrNotSupported.
func TestListNotSupported(t *testing.T) {
	b, _ := pane.New("ghostty")
	_, err := b.List()
	assertNotSupported(t, "List", err)
}

// TestCloseNotSupported verifies Close returns ErrNotSupported.
func TestCloseNotSupported(t *testing.T) {
	b, _ := pane.New("ghostty")
	err := b.Close("1")
	assertNotSupported(t, "Close", err)
}

// TestWaitNotSupported verifies Wait returns ErrNotSupported.
func TestWaitNotSupported(t *testing.T) {
	b, _ := pane.New("ghostty")
	code, err := b.Wait("1")
	assertNotSupported(t, "Wait", err)
	if code != -1 {
		t.Errorf("Wait returned code %d, want -1", code)
	}
}

// assertNotSupported fails the test if err is nil or does not wrap ErrNotSupported.
func assertNotSupported(t *testing.T, method string, err error) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: expected error wrapping pane.ErrNotSupported, got nil", method)
		return
	}
	if !errors.Is(err, pane.ErrNotSupported) {
		t.Errorf("%s: error %q does not wrap pane.ErrNotSupported", method, err)
	}
}
