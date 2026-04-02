// Package wezterm implements the pane.Backend interface for WezTerm.
//
// WezTerm pane IDs are plain unsigned integers (e.g. "3").
// The $WEZTERM_PANE env var holds the current pane's ID.
//
// Limitations:
//   - Exit codes are unavailable via the WezTerm CLI; Wait() and the final
//     OutputEvent always return Code=-1.
//   - Subscribe() uses a 200ms polling loop (no native streaming API).
//   - Env vars in Spawn() are injected via `sh -c 'export K=V; ...'` because
//     `wezterm cli spawn` has no --set-environment flag.
package wezterm

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/justEstif/openswarm/internal/pane"
)

func init() {
	pane.Register("wezterm", func() pane.Backend { return &WeztermBackend{} })
}

// WeztermBackend drives WezTerm via `wezterm cli` subcommands.
type WeztermBackend struct{}

// Name returns the registered backend name.
func (w *WeztermBackend) Name() string { return "wezterm" }

// Spawn creates a new WezTerm pane running cmd.
// Env vars are injected by wrapping cmd in `sh -c 'export K=V; ...; exec cmd'`.
//
// Placement semantics:
//   - current_tab (default): spawns a new tab in the current WezTerm window.
//   - new_tab, new_session: spawns in a brand-new WezTerm window (--new-window).
//     WezTerm has no session concept; new_session is treated identically to new_tab.
//
// CloseOnExit: WezTerm closes panes automatically when their process exits,
// so no explicit cleanup is needed regardless of this flag.
func (w *WeztermBackend) Spawn(name, cmd string, opts pane.SpawnOptions) (pane.PaneID, error) {
	env := opts.Env
	// Build env prefix: export K=V K2=V2; exec cmd
	var sb strings.Builder
	for k, v := range env {
		// Shell-quote the value to handle spaces and special chars.
		fmt.Fprintf(&sb, "export %s=%s; ", shellEscape(k), shellEscape(v))
	}
	sb.WriteString("exec ")
	sb.WriteString(cmd)
	shellCmd := sb.String()

	// Build spawn args: new_tab / new_session → open in a separate window.
	args := []string{"cli", "spawn"}
	switch opts.Placement {
	case pane.PlacementNewTab, pane.PlacementNewSession:
		args = append(args, "--new-window")
	}
	args = append(args, "--", "sh", "-c", shellCmd)

	out, err := runCmd(args...)
	if err != nil {
		return "", fmt.Errorf("wezterm spawn: %w", err)
	}
	id := pane.PaneID(strings.TrimSpace(out))
	if id == "" {
		return "", fmt.Errorf("wezterm spawn: empty pane ID returned")
	}

	// Set the tab title (best-effort; don't fail Spawn if this fails).
	_, _ = runCmd("cli", "set-tab-title", "--pane-id", string(id), name)

	return id, nil
}

// Send delivers text to a pane's stdin.
func (w *WeztermBackend) Send(id pane.PaneID, text string) error {
	_, err := runCmd("cli", "send-text", "--pane-id", string(id), "--no-paste", text)
	if err != nil {
		return fmt.Errorf("wezterm send-text: %w", err)
	}
	return nil
}

// Capture returns the current scrollback (last 200 lines) of a pane as plain text.
func (w *WeztermBackend) Capture(id pane.PaneID) (string, error) {
	out, err := runCmd("cli", "get-text", "--pane-id", string(id), "--start-line", "-200")
	if err != nil {
		return "", fmt.Errorf("wezterm get-text: %w", err)
	}
	return out, nil
}

// Subscribe streams output from a pane by polling every 200ms until the pane
// exits or ctx is cancelled.  When the pane disappears from `wezterm cli list`,
// a final OutputEvent with Exited=true and Code=-1 is sent.
//
// Exit codes are structurally unavailable via the WezTerm CLI (see docs/research/backends/wezterm.md).
func (w *WeztermBackend) Subscribe(ctx context.Context, id pane.PaneID) (<-chan pane.OutputEvent, error) {
	ch := make(chan pane.OutputEvent, 16)
	go func() {
		defer close(ch)
		var prev string
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Capture current output and diff.
				current, err := w.Capture(id)
				if err == nil && current != prev {
					newText := diffStrings(prev, current)
					if newText != "" {
						select {
						case ch <- pane.OutputEvent{PaneID: id, Text: newText}:
						case <-ctx.Done():
							return
						}
					}
					prev = current
				}

				// Check whether the pane still exists.
				exists, err := w.paneExists(id)
				if err != nil {
					// List call failed; keep polling.
					continue
				}
				if !exists {
					select {
					case ch <- pane.OutputEvent{PaneID: id, Exited: true, Code: -1}:
					case <-ctx.Done():
					}
					return
				}
			}
		}
	}()
	return ch, nil
}

// weztermPane is the JSON representation of one entry in `wezterm cli list --format json`.
type weztermPane struct {
	PaneID   int    `json:"pane_id"`
	Title    string `json:"title"`
	TabTitle string `json:"tab_title"`
	IsActive bool   `json:"is_active"`
}

// List returns all panes known to WezTerm in the current session.
// Running is always true because WezTerm does not expose exit state via CLI.
func (w *WeztermBackend) List() ([]pane.PaneInfo, error) {
	out, err := runCmd("cli", "list", "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("wezterm list: %w", err)
	}
	return parseWeztermList(out)
}

// parseWeztermList parses `wezterm cli list --format json` output.
// Extracted for unit-testability.
func parseWeztermList(jsonData string) ([]pane.PaneInfo, error) {
	var raw []weztermPane
	if err := json.Unmarshal([]byte(jsonData), &raw); err != nil {
		return nil, fmt.Errorf("wezterm list: parse JSON: %w", err)
	}
	infos := make([]pane.PaneInfo, 0, len(raw))
	for _, p := range raw {
		name := p.TabTitle
		if name == "" {
			name = p.Title
		}
		infos = append(infos, pane.PaneInfo{
			ID:      pane.PaneID(fmt.Sprintf("%d", p.PaneID)),
			Name:    name,
			Running: true, // WezTerm does not expose exit state via CLI
		})
	}
	return infos, nil
}

// Close terminates a pane. Idempotent — errors from killing an already-dead pane are ignored.
func (w *WeztermBackend) Close(id pane.PaneID) error {
	_, err := runCmd("cli", "kill-pane", "--pane-id", string(id))
	if err != nil {
		// kill-pane exits non-zero if the pane is already gone; treat as success.
		return nil
	}
	return nil
}

// Wait blocks until the pane exits (i.e. disappears from `wezterm cli list`).
// Always returns -1 because WezTerm does not expose exit codes via CLI.
func (w *WeztermBackend) Wait(id pane.PaneID) (int, error) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		exists, err := w.paneExists(id)
		if err != nil {
			// Transient list failure; keep waiting.
			continue
		}
		if !exists {
			// Pane is gone. Exit code is structurally unavailable.
			return -1, nil
		}
	}
	// ticker.C never returns false from range, but satisfy the compiler.
	return -1, nil
}

// paneExists returns true if id appears in `wezterm cli list --format json`.
func (w *WeztermBackend) paneExists(id pane.PaneID) (bool, error) {
	out, err := runCmd("cli", "list", "--format", "json")
	if err != nil {
		return false, err
	}
	var raw []weztermPane
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return false, err
	}
	want := string(id)
	for _, p := range raw {
		if fmt.Sprintf("%d", p.PaneID) == want {
			return true, nil
		}
	}
	return false, nil
}

// runCmd runs a wezterm CLI command and returns its combined stdout output.
// stderr is captured and included in the error message on failure.
func runCmd(args ...string) (string, error) { //nolint:unparam
	const name = "wezterm"
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("%s %s: exit %d: %s",
				name, strings.Join(args, " "), ee.ExitCode(), string(ee.Stderr))
		}
		return "", fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return string(out), nil
}

// shellEscape returns a single-quoted shell-safe string.
// Single quotes are escaped by ending the single-quote, adding an escaped
// single-quote, and re-opening the single-quote.
func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// diffStrings returns the suffix of next that is not in prev.
// This simple approach works well for append-only terminal output.
func diffStrings(prev, next string) string {
	if strings.HasPrefix(next, prev) {
		return next[len(prev):]
	}
	// Output changed in a non-append way (e.g. screen cleared); return full current.
	return next
}
