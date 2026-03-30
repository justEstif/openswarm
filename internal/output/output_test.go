package output_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/justEstif/openswarm/internal/output"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// captureStdout replaces os.Stdout with a pipe, runs f, and returns what was
// written.  It restores os.Stdout on return.
func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	old := os.Stdout
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("io.Copy: %v", err)
	}
	r.Close()
	return buf.String()
}

// captureStderr is the same but for stderr.
func captureStderr(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	old := os.Stderr
	os.Stderr = w

	f()

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("io.Copy: %v", err)
	}
	r.Close()
	return buf.String()
}

// ─── fixtures ────────────────────────────────────────────────────────────────

type agent struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

type task struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

// ─── SwarmError ──────────────────────────────────────────────────────────────

func TestSwarmError_Error(t *testing.T) {
	tests := []struct {
		err  *output.SwarmError
		want string
	}{
		{output.ErrNotFound("agent not found"), "NOT_FOUND: agent not found"},
		{output.ErrConflict("already exists"), "CONFLICT: already exists"},
		{output.ErrValidation("name required"), "VALIDATION: name required"},
		{output.ErrIO("read failed"), "IO_ERROR: read failed"},
		{output.ErrLocked("tasks.json"), "LOCKED: tasks.json"},
	}
	for _, tt := range tests {
		if got := tt.err.Error(); got != tt.want {
			t.Errorf("Error() = %q, want %q", got, tt.want)
		}
	}
}

func TestSwarmError_Codes(t *testing.T) {
	cases := map[string]*output.SwarmError{
		"NOT_FOUND":  output.ErrNotFound("x"),
		"CONFLICT":   output.ErrConflict("x"),
		"VALIDATION": output.ErrValidation("x"),
		"IO_ERROR":   output.ErrIO("x"),
		"LOCKED":     output.ErrLocked("x"),
	}
	for wantCode, err := range cases {
		if err.Code != wantCode {
			t.Errorf("expected code %q, got %q", wantCode, err.Code)
		}
	}
}

// ─── Print — nil is a no-op ───────────────────────────────────────────────────

func TestPrint_Nil(t *testing.T) {
	out := captureStdout(t, func() {
		if err := output.Print(nil, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if out != "" {
		t.Errorf("expected no output for nil, got %q", out)
	}

	out = captureStdout(t, func() {
		if err := output.Print(nil, true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if out != "" {
		t.Errorf("expected no JSON output for nil, got %q", out)
	}
}

func TestPrint_NilPointer(t *testing.T) {
	var a *agent
	out := captureStdout(t, func() {
		if err := output.Print(a, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if out != "" {
		t.Errorf("expected no output for nil pointer, got %q", out)
	}
}

// ─── Print — JSON struct ──────────────────────────────────────────────────────

func TestPrint_JSON_Struct(t *testing.T) {
	a := agent{ID: "a1", Name: "alice", Role: "researcher"}
	out := captureStdout(t, func() {
		if err := output.Print(a, true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var got agent
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out)
	}
	if got.ID != "a1" || got.Name != "alice" || got.Role != "researcher" {
		t.Errorf("unexpected decoded value: %+v", got)
	}

	// Must be indented (2-space).
	if !strings.Contains(out, "  ") {
		t.Errorf("expected 2-space indent, got: %s", out)
	}
}

// ─── Print — JSON slice ───────────────────────────────────────────────────────

func TestPrint_JSON_Slice(t *testing.T) {
	agents := []agent{
		{ID: "a1", Name: "alice", Role: "researcher"},
		{ID: "a2", Name: "bob", Role: "coder"},
	}
	out := captureStdout(t, func() {
		if err := output.Print(agents, true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var got []agent
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(got))
	}
	if got[0].Name != "alice" || got[1].Name != "bob" {
		t.Errorf("unexpected decoded slice: %+v", got)
	}
}

// ─── Print — human struct ─────────────────────────────────────────────────────

func TestPrint_Human_Struct(t *testing.T) {
	a := agent{ID: "a1", Name: "alice", Role: "researcher"}
	out := captureStdout(t, func() {
		if err := output.Print(a, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	for _, want := range []string{"id", "a1", "name", "alice", "role", "researcher"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q\nactual output:\n%s", want, out)
		}
	}
}

// ─── Print — human slice ──────────────────────────────────────────────────────

func TestPrint_Human_Slice(t *testing.T) {
	agents := []agent{
		{ID: "a1", Name: "alice", Role: "researcher"},
		{ID: "a2", Name: "bob", Role: "coder"},
	}
	out := captureStdout(t, func() {
		if err := output.Print(agents, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Header row must be present.
	upperOut := strings.ToUpper(out)
	for _, col := range []string{"ID", "NAME", "ROLE"} {
		if !strings.Contains(upperOut, col) {
			t.Errorf("expected header column %q\nactual output:\n%s", col, out)
		}
	}
	// Data must be present.
	for _, want := range []string{"a1", "alice", "researcher", "a2", "bob", "coder"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output\nactual output:\n%s", want, out)
		}
	}
}

// ─── Print — human empty slice ───────────────────────────────────────────────

func TestPrint_Human_EmptySlice(t *testing.T) {
	out := captureStdout(t, func() {
		if err := output.Print([]agent{}, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "none") {
		t.Errorf("expected '(none)' for empty slice, got: %q", out)
	}
}

// ─── Print — pointer to struct ────────────────────────────────────────────────

func TestPrint_Human_PointerToStruct(t *testing.T) {
	a := &agent{ID: "a3", Name: "carol", Role: "planner"}
	out := captureStdout(t, func() {
		if err := output.Print(a, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	for _, want := range []string{"a3", "carol", "planner"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output\nactual:\n%s", want, out)
		}
	}
}

// ─── Print — JSON goes to stdout, not stderr ──────────────────────────────────

func TestPrint_JSON_GoesToStdout(t *testing.T) {
	a := agent{ID: "a1", Name: "alice", Role: "researcher"}

	stdout := captureStdout(t, func() {
		if err := output.Print(a, true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if stdout == "" {
		t.Error("expected JSON on stdout, got nothing")
	}
}

// ─── Print — human output with multiple field types ───────────────────────────

func TestPrint_Human_MultipleStructTypes(t *testing.T) {
	tasks := []task{
		{ID: "t1", Title: "build the thing", Status: "open"},
		{ID: "t2", Title: "test everything", Status: "done"},
	}
	out := captureStdout(t, func() {
		if err := output.Print(tasks, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	for _, want := range []string{"t1", "build the thing", "open", "t2", "test everything", "done"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output\nactual:\n%s", want, out)
		}
	}
}

// ─── PrintError — human mode goes to stderr ───────────────────────────────────

func TestPrintError_Human_GoesToStderr(t *testing.T) {
	err := output.ErrNotFound("task t1 not found")

	stderr := captureStderr(t, func() {
		output.PrintError(err, false)
	})

	if !strings.Contains(stderr, "NOT_FOUND") {
		t.Errorf("expected NOT_FOUND in stderr\nactual: %q", stderr)
	}
	if !strings.Contains(stderr, "task t1 not found") {
		t.Errorf("expected message in stderr\nactual: %q", stderr)
	}
}

func TestPrintError_Human_NothingOnStdout(t *testing.T) {
	err := output.ErrNotFound("task t1 not found")

	stdout := captureStdout(t, func() {
		// stderr captured separately — we only care stdout is empty.
		old := os.Stderr
		os.Stderr, _ = os.Open(os.DevNull)
		output.PrintError(err, false)
		os.Stderr = old
	})

	if stdout != "" {
		t.Errorf("expected nothing on stdout in human mode, got: %q", stdout)
	}
}

// ─── PrintError — JSON mode goes to stdout ────────────────────────────────────

func TestPrintError_JSON_GoesToStdout(t *testing.T) {
	err := output.ErrConflict("agent already registered")

	stdout := captureStdout(t, func() {
		output.PrintError(err, true)
	})

	var wrapper struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if jsonErr := json.Unmarshal([]byte(stdout), &wrapper); jsonErr != nil {
		t.Fatalf("invalid JSON on stdout: %v\noutput: %s", jsonErr, stdout)
	}
	if wrapper.Error.Code != "CONFLICT" {
		t.Errorf("expected code CONFLICT, got %q", wrapper.Error.Code)
	}
	if wrapper.Error.Message != "agent already registered" {
		t.Errorf("expected message, got %q", wrapper.Error.Message)
	}
}

// ─── PrintError — non-SwarmError is wrapped as IO_ERROR ──────────────────────

func TestPrintError_NonSwarmError_WrappedAsIO(t *testing.T) {
	plainErr := io.ErrUnexpectedEOF

	stdout := captureStdout(t, func() {
		output.PrintError(plainErr, true)
	})

	var wrapper struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(stdout), &wrapper); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout)
	}
	if wrapper.Error.Code != "IO_ERROR" {
		t.Errorf("expected IO_ERROR, got %q", wrapper.Error.Code)
	}
	if !strings.Contains(wrapper.Error.Message, "EOF") {
		t.Errorf("expected EOF in message, got %q", wrapper.Error.Message)
	}
}

func TestPrintError_NonSwarmError_Human(t *testing.T) {
	plainErr := io.ErrUnexpectedEOF

	stderr := captureStderr(t, func() {
		output.PrintError(plainErr, false)
	})

	if !strings.Contains(stderr, "IO_ERROR") {
		t.Errorf("expected IO_ERROR in stderr, got: %q", stderr)
	}
}

// ─── PrintError — nil is a no-op ─────────────────────────────────────────────

func TestPrintError_Nil(t *testing.T) {
	stdout := captureStdout(t, func() {
		output.PrintError(nil, true)
	})
	if stdout != "" {
		t.Errorf("expected no output for nil error, got: %q", stdout)
	}

	stderr := captureStderr(t, func() {
		output.PrintError(nil, false)
	})
	if stderr != "" {
		t.Errorf("expected no stderr for nil error, got: %q", stderr)
	}
}

// ─── Print — json tag "-" fields are skipped ─────────────────────────────────

func TestPrint_SkipsHiddenFields(t *testing.T) {
	type withHidden struct {
		Visible string `json:"visible"`
		Hidden  string `json:"-"`
	}
	v := withHidden{Visible: "yes", Hidden: "secret"}

	out := captureStdout(t, func() {
		if err := output.Print(v, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if strings.Contains(out, "secret") {
		t.Errorf("hidden field should not appear in output, got:\n%s", out)
	}
	if !strings.Contains(out, "yes") {
		t.Errorf("visible field missing from output, got:\n%s", out)
	}
}

func TestPrint_SkipsHiddenFields_JSON(t *testing.T) {
	type withHidden struct {
		Visible string `json:"visible"`
		Hidden  string `json:"-"`
	}
	v := withHidden{Visible: "yes", Hidden: "secret"}

	out := captureStdout(t, func() {
		if err := output.Print(v, true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	// Standard encoding/json already excludes "-" tagged fields.
	if strings.Contains(out, "secret") {
		t.Errorf("hidden field should not appear in JSON, got:\n%s", out)
	}
}
