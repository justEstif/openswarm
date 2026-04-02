package tmux

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/justEstif/openswarm/internal/pane"
)

// hasTmux reports whether tmux is available on PATH.
// Integration tests call t.Skip when this returns false.
func hasTmux() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

// ---------------------------------------------------------------------------
// Unit tests — no tmux binary required
// ---------------------------------------------------------------------------

func TestName(t *testing.T) {
	b := &TmuxBackend{}
	if got := b.Name(); got != "tmux" {
		t.Errorf("Name() = %q, want %q", got, "tmux")
	}
}

func TestRegistered(t *testing.T) {
	b, err := pane.New("tmux")
	if err != nil {
		t.Fatalf("pane.New(%q) error: %v", "tmux", err)
	}
	if b.Name() != "tmux" {
		t.Errorf("registered backend Name() = %q, want %q", b.Name(), "tmux")
	}
}

// ---------------------------------------------------------------------------
// buildEnvCmd tests
// ---------------------------------------------------------------------------

func TestBuildEnvCmd_NoEnv(t *testing.T) {
	got := buildEnvCmd("go build ./...", nil)
	want := "sh -c 'go build ./...'"
	if got != want {
		t.Errorf("buildEnvCmd with no env\n got:  %q\n want: %q", got, want)
	}
}

func TestBuildEnvCmd_WithEnv(t *testing.T) {
	env := map[string]string{
		"FOO": "bar",
		"BAZ": "qux",
	}
	got := buildEnvCmd("my-cmd", env)
	// Keys are sorted: BAZ before FOO.
	want := "env BAZ='qux' FOO='bar' sh -c 'my-cmd'"
	if got != want {
		t.Errorf("buildEnvCmd with env\n got:  %q\n want: %q", got, want)
	}
}

func TestBuildEnvCmd_SpecialCharsInValue(t *testing.T) {
	env := map[string]string{"MSG": "it's a test"}
	got := buildEnvCmd("echo $MSG", env)
	// Single quote in value must be escaped as '\''.
	if !strings.Contains(got, `'it'\''s a test'`) {
		t.Errorf("buildEnvCmd special chars: got %q, expected escaped single quote", got)
	}
}

func TestBuildEnvCmd_SpecialCharsInCmd(t *testing.T) {
	got := buildEnvCmd("echo 'hello world'", nil)
	// Inner single quotes in cmd must be escaped.
	if !strings.Contains(got, `'echo '\''hello world'\'''`) {
		t.Errorf("buildEnvCmd special chars in cmd: got %q", got)
	}
}

func TestBuildEnvCmd_EmptyEnv(t *testing.T) {
	got := buildEnvCmd("ls", map[string]string{})
	want := "sh -c 'ls'"
	if got != want {
		t.Errorf("buildEnvCmd empty env\n got:  %q\n want: %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// parseListOutput tests
// ---------------------------------------------------------------------------

func TestParseListOutput_Empty(t *testing.T) {
	infos, err := parseListOutput("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(infos) != 0 {
		t.Errorf("expected 0 panes, got %d", len(infos))
	}
}

func TestParseListOutput_Single(t *testing.T) {
	input := "%42\tmywindow\t0\tbash\n"
	infos, err := parseListOutput(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("expected 1 pane, got %d", len(infos))
	}
	p := infos[0]
	if p.ID != "%42" {
		t.Errorf("ID = %q, want %%42", p.ID)
	}
	if p.Name != "mywindow" {
		t.Errorf("Name = %q, want mywindow", p.Name)
	}
	if !p.Running {
		t.Error("Running = false, want true (pane_dead=0)")
	}
	if p.Command != "bash" {
		t.Errorf("Command = %q, want bash", p.Command)
	}
}

func TestParseListOutput_Dead(t *testing.T) {
	input := "%7\tbuild\t1\tsh\n"
	infos, err := parseListOutput(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("expected 1 pane, got %d", len(infos))
	}
	if infos[0].Running {
		t.Error("Running = true, want false (pane_dead=1)")
	}
}

func TestParseListOutput_Multiple(t *testing.T) {
	input := strings.Join([]string{
		"%1\twin-a\t0\tzsh",
		"%2\twin-b\t1\tpython3",
		"%3\twin-c\t0\tvim",
	}, "\n") + "\n"

	infos, err := parseListOutput(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(infos) != 3 {
		t.Fatalf("expected 3 panes, got %d", len(infos))
	}
	ids := []pane.PaneID{"%1", "%2", "%3"}
	for i, want := range ids {
		if infos[i].ID != want {
			t.Errorf("pane[%d].ID = %q, want %q", i, infos[i].ID, want)
		}
	}
}

func TestParseListOutput_MalformedLineSkipped(t *testing.T) {
	// A line with fewer than 4 tab-separated fields should be skipped.
	input := "%42\tmywindow\n"
	infos, err := parseListOutput(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(infos) != 0 {
		t.Errorf("expected 0 panes (malformed skipped), got %d", len(infos))
	}
}

func TestParseListOutput_CommandWithTab(t *testing.T) {
	// SplitN(..., 4) ensures tabs in the command field are preserved.
	input := "%5\twin\t0\tsh\t-c\tsomething\n"
	infos, err := parseListOutput(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("expected 1 pane, got %d", len(infos))
	}
	// The command field captures everything after the 3rd tab.
	if !strings.HasPrefix(infos[0].Command, "sh") {
		t.Errorf("Command = %q, want prefix %q", infos[0].Command, "sh")
	}
}

// ---------------------------------------------------------------------------
// singleQuote tests
// ---------------------------------------------------------------------------

func TestSingleQuote_Plain(t *testing.T) {
	if got := singleQuote("hello"); got != "'hello'" {
		t.Errorf("singleQuote(hello) = %q", got)
	}
}

func TestSingleQuote_WithSingleQuote(t *testing.T) {
	got := singleQuote("it's")
	want := `'it'\''s'`
	if got != want {
		t.Errorf("singleQuote(it's) = %q, want %q", got, want)
	}
}

func TestSingleQuote_Empty(t *testing.T) {
	if got := singleQuote(""); got != "''" {
		t.Errorf("singleQuote('') = %q, want ''", got)
	}
}

// ---------------------------------------------------------------------------
// Integration tests — require tmux binary
// ---------------------------------------------------------------------------

func TestSpawnAndClose_Integration(t *testing.T) {
	if !hasTmux() {
		t.Skip("tmux not found")
	}
	b := &TmuxBackend{}

	id, err := b.Spawn("test-pane", "sleep 60", pane.SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	t.Logf("spawned pane %s", id)
	if !strings.HasPrefix(string(id), "%") {
		t.Errorf("pane ID %q does not start with %%", id)
	}

	// Close should succeed and be idempotent.
	if err := b.Close(id); err != nil {
		t.Errorf("Close: %v", err)
	}
	if err := b.Close(id); err != nil {
		t.Errorf("Close (idempotent): %v", err)
	}
}

func TestSendAndCapture_Integration(t *testing.T) {
	if !hasTmux() {
		t.Skip("tmux not found")
	}
	b := &TmuxBackend{}

	id, err := b.Spawn("cap-test", "cat", pane.SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	t.Cleanup(func() { _ = b.Close(id) })

	// Allow the shell to settle.
	// (In production the handshake handles this; here we just sleep briefly.)
	time.Sleep(50 * time.Millisecond)

	if err := b.Send(id, "hello from test\n"); err != nil {
		t.Fatalf("Send: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	out, err := b.Capture(id)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	t.Logf("captured: %q", out)
}

func TestList_Integration(t *testing.T) {
	if !hasTmux() {
		t.Skip("tmux not found")
	}
	b := &TmuxBackend{}

	id, err := b.Spawn("list-test", "sleep 30", pane.SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	t.Cleanup(func() { _ = b.Close(id) })

	infos, err := b.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	var found bool
	for _, p := range infos {
		if p.ID == id {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("spawned pane %s not found in List()", id)
	}
}

func TestWait_Integration(t *testing.T) {
	if !hasTmux() {
		t.Skip("tmux not found")
	}
	b := &TmuxBackend{}

	// A command that exits quickly with a known code.
	id, err := b.Spawn("wait-test", "exit 42", pane.SpawnOptions{})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	t.Cleanup(func() { _ = b.Close(id) })

	code, err := b.Wait(id)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	t.Logf("Wait returned code %d", code)
	// exit 42 should give 42; if remain-on-exit failed we get -1.
	if code != 42 && code != -1 {
		t.Errorf("Wait returned %d, want 42 (or -1 if remain-on-exit unavailable)", code)
	}
}
