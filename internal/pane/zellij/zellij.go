// Package zellij implements the pane.Backend interface for Zellij v0.44.0+.
//
// Register via blank import:
//
//	import _ "github.com/justEstif/openswarm/internal/pane/zellij"
//
// Requires Zellij v0.44.0+ for --pane-id targeting on write-chars, dump-screen,
// close-pane, and for new-pane returning the pane ID on stdout.
package zellij

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/justEstif/openswarm/internal/pane"
)

func init() {
	pane.Register("zellij", func() pane.Backend { return &ZellijBackend{} })
}

// ZellijBackend implements pane.Backend using the Zellij CLI.
type ZellijBackend struct{}

// Name returns the backend identifier.
func (b *ZellijBackend) Name() string { return "zellij" }

// paneInt strips the "terminal_" prefix from a PaneID and returns the bare integer string.
// e.g. pane.PaneID("terminal_5") → "5"
func paneInt(id pane.PaneID) string {
	s := string(id)
	if after, ok := strings.CutPrefix(s, "terminal_"); ok {
		return after
	}
	return s
}

// Spawn creates a new Zellij pane running cmd under the given name.
// opts.Placement controls whether the pane is created in the current tab (default),
// a new tab, or a new session. opts.CloseOnExit injects a cleanup trailer that
// auto-closes the pane's container (tab/session) when the command exits.
func (b *ZellijBackend) Spawn(name, cmd string, opts pane.SpawnOptions) (pane.PaneID, error) {
	fullCmd := buildEnvCmd(cmd, opts.Env)

	switch opts.Placement {
	case pane.PlacementNewTab:
		return b.spawnNewTab(name, fullCmd, opts.CloseOnExit)
	case pane.PlacementNewSession:
		return b.spawnNewSession(name, fullCmd, opts.CloseOnExit)
	default: // PlacementCurrentTab or ""
		return b.spawnCurrentTab(name, fullCmd, opts.CloseOnExit)
	}
}

// spawnCurrentTab creates a pane in the active Zellij tab.
// With closeOnExit=true, the native -c flag closes the pane on exit.
func (b *ZellijBackend) spawnCurrentTab(name, fullCmd string, closeOnExit bool) (pane.PaneID, error) {
	args := []string{"action", "new-pane", "--name", name}
	if closeOnExit {
		args = append(args, "-c")
	}
	args = append(args, "--", "sh", "-c", fullCmd)
	out, err := exec.Command("zellij", args...).Output()
	if err != nil {
		return "", fmt.Errorf("zellij new-pane: %w", err)
	}
	id := strings.TrimSpace(string(out))
	if id != "" {
		return pane.PaneID(id), nil
	}
	return findPaneByName(name)
}

// spawnNewTab creates a dedicated Zellij tab and a pane inside it.
// The new tab opens in the background — focus is returned to the original tab
// immediately after spawning so the user's workflow is not interrupted.
// With closeOnExit=true, a cleanup trailer closes the tab when the command exits.
func (b *ZellijBackend) spawnNewTab(name, fullCmd string, closeOnExit bool) (pane.PaneID, error) {
	// One list-tabs call gives us both the active tab ID and the full snapshot.
	originalTabID, tabsBefore, err := listTabsSnapshot()
	if err != nil {
		return "", fmt.Errorf("zellij new_tab: list tabs: %w", err)
	}

	// Create the tab (focus moves there).
	if err := exec.Command("zellij", "action", "new-tab", "--name", name).Run(); err != nil {
		return "", fmt.Errorf("zellij new-tab: %w", err)
	}

	// Find the new tab's stable ID.
	newTabID, err := findNewTabID(tabsBefore)
	if err != nil {
		return "", fmt.Errorf("zellij new_tab: %w", err)
	}

	if closeOnExit {
		// Append trailer that closes the tab (and all its panes) on exit.
		fullCmd = fmt.Sprintf("%s; zellij action close-tab-by-id %d 2>/dev/null || true", fullCmd, newTabID)
	}

	// Spawn the pane into the now-focused new tab. No -c — cleanup trailer handles it.
	out, err := exec.Command("zellij", "action", "new-pane", "--name", name, "--", "sh", "-c", fullCmd).Output()
	if err != nil {
		return "", fmt.Errorf("zellij new-pane (new_tab): %w", err)
	}

	// Restore focus to the original tab — the new tab runs in the background.
	_ = exec.Command("zellij", "action", "go-to-tab-by-id", fmt.Sprintf("%d", originalTabID)).Run()

	id := strings.TrimSpace(string(out))
	if id != "" {
		return pane.PaneID(id), nil
	}
	return findPaneByName(name)
}

// spawnNewSession creates a new Zellij session containing the pane.
// The session name is derived from the pane name. With closeOnExit=true,
// a cleanup trailer kills the session (via $ZELLIJ_SESSION_NAME) on exit.
//
// LIMITATION: The returned PaneID has the prefix "session_" (e.g. "session_swarm-foo").
// The pane lives in the new session, not the caller's session, so Send, Capture,
// and Wait will not work on it. new_session is fire-and-forget: use CloseOnExit=true
// and do not call Wait. Close is idempotent and tolerates session-scoped IDs.
func (b *ZellijBackend) spawnNewSession(name, fullCmd string, closeOnExit bool) (pane.PaneID, error) {
	if closeOnExit {
		fullCmd = fullCmd + `; zellij kill-session "$ZELLIJ_SESSION_NAME" 2>/dev/null || true`
	}

	sessionName := "swarm-" + name
	if err := exec.Command("zellij",
		"--session", sessionName,
		"run", "-c", "--", "sh", "-c", fullCmd,
	).Run(); err != nil {
		return "", fmt.Errorf("zellij new_session: %w", err)
	}

	// The pane lives in the new session — not visible from list-panes in the
	// current session. Return a synthetic session-scoped ID so callers can at
	// least identify the run in runs.json.
	return pane.PaneID("session_" + sessionName), nil
}

// buildEnvCmd constructs "env KEY='VAL' KEY='VAL' <cmd>" if env is non-empty, else just cmd.
// Values are single-quoted so spaces and shell special characters are safe.
func buildEnvCmd(cmd string, env map[string]string) string {
	if len(env) == 0 {
		return cmd
	}
	var sb strings.Builder
	sb.WriteString("env")
	for k, v := range env {
		sb.WriteString(" ")
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(singleQuote(v))
	}
	sb.WriteString(" ")
	sb.WriteString(cmd)
	return sb.String()
}

// singleQuote wraps s in POSIX single quotes, escaping any literal single quotes inside.
func singleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''")+"'"
}

// ─── Tab / pane discovery helpers ───────────────────────────────────────────

// zellijTabJSON is the JSON schema for a single entry from `zellij action list-tabs --json`.
type zellijTabJSON struct {
	Position int    `json:"position"`
	Name     string `json:"name"`
	TabID    int    `json:"tab_id"`
	Active   bool   `json:"active"`
}

// listTabsSnapshot calls list-tabs --json once and returns both the active tab_id
// and the full set of all tab IDs. Used by spawnNewTab to avoid a double round-trip.
func listTabsSnapshot() (activeID int, allIDs map[int]struct{}, err error) {
	out, execErr := exec.Command("zellij", "action", "list-tabs", "--json").Output()
	if execErr != nil {
		err = fmt.Errorf("list-tabs: %w", execErr)
		return
	}
	var tabs []zellijTabJSON
	if parseErr := json.Unmarshal(out, &tabs); parseErr != nil {
		err = fmt.Errorf("list-tabs parse: %w", parseErr)
		return
	}
	allIDs = make(map[int]struct{}, len(tabs))
	for _, t := range tabs {
		allIDs[t.TabID] = struct{}{}
		if t.Active {
			activeID = t.TabID
		}
	}
	return
}

// findNewTabID polls list-tabs until a tab_id appears that was absent in before.
// Retries for up to 2 seconds.
func findNewTabID(before map[int]struct{}) (int, error) {
	for range 20 {
		out, err := exec.Command("zellij", "action", "list-tabs", "--json").Output()
		if err == nil {
			var tabs []zellijTabJSON
			if err := json.Unmarshal(out, &tabs); err == nil {
				for _, t := range tabs {
					if _, exists := before[t.TabID]; !exists {
						return t.TabID, nil
					}
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return 0, fmt.Errorf("new tab did not appear in list-tabs within 2s")
}

// findPaneByName lists all panes and returns the ID of the one matching name.
// Used as a fallback when new-pane doesn't print the pane ID (pre-v0.44.0).
func findPaneByName(name string) (pane.PaneID, error) {
	out, err := exec.Command("zellij", "action", "list-panes", "--json").Output()
	if err != nil {
		return "", fmt.Errorf("zellij list-panes (fallback): %w", err)
	}

	var raw []zellijPaneJSON
	if err := json.Unmarshal(out, &raw); err != nil {
		return "", fmt.Errorf("zellij list-panes parse (fallback): %w", err)
	}

	for _, p := range raw {
		if p.Title == name && !p.IsPlugin {
			return pane.PaneID(fmt.Sprintf("terminal_%d", p.ID)), nil
		}
	}
	return "", fmt.Errorf("zellij: pane with name %q not found after spawn", name)
}

// Send delivers text to the pane's stdin using write-chars.
func (b *ZellijBackend) Send(id pane.PaneID, text string) error {
	n := paneInt(id)
	cmd := exec.Command("zellij", "action", "write-chars", "--pane-id", n, text)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zellij write-chars: %w: %s", err, bytes.TrimSpace(out))
	}
	return nil
}

// Capture returns the full screen content (viewport + scrollback) of a pane.
func (b *ZellijBackend) Capture(id pane.PaneID) (string, error) {
	n := paneInt(id)
	out, err := exec.Command("zellij", "action", "dump-screen", "--pane-id", n, "--full").Output()
	if err != nil {
		return "", fmt.Errorf("zellij dump-screen: %w", err)
	}
	return string(out), nil
}

// Subscribe streams output from a pane using `zellij subscribe --format json` (NDJSON).
// Falls back to polling if the subscribe subcommand is unavailable.
func (b *ZellijBackend) Subscribe(ctx context.Context, id pane.PaneID) (<-chan pane.OutputEvent, error) {
	ch := make(chan pane.OutputEvent, 64)

	n := paneInt(id)

	// Check if the subscribe subcommand is available by attempting a dry-run help lookup.
	if !zellijSubscribeAvailable() {
		go pollSubscribe(ctx, b, id, ch)
		return ch, nil
	}

	cmd := exec.CommandContext(ctx, "zellij", "subscribe", "--pane-id", n, "--format", "json")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("zellij subscribe stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("zellij subscribe start: %w", err)
	}

	go func() {
		defer close(ch)
		defer cmd.Wait() //nolint:errcheck

		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			evt := parseSubscribeEvent(id, line)
			select {
			case ch <- evt:
			case <-ctx.Done():
				return
			}
			if evt.Exited {
				return
			}
		}
		// Process exited or ctx cancelled — emit final Exited event.
		select {
		case ch <- pane.OutputEvent{PaneID: id, Exited: true}:
		default:
		}
	}()

	return ch, nil
}

// zellijSubscribeEvent is the NDJSON line schema from `zellij subscribe --format json`.
type zellijSubscribeEvent struct {
	Event    string   `json:"event"`
	PaneID   string   `json:"pane_id"`
	Viewport []string `json:"viewport"`
	Exited   bool     `json:"exited"`
	ExitCode *int     `json:"exit_code"`
	Text     string   `json:"text"`
}

// parseSubscribeEvent parses one NDJSON line from `zellij subscribe`.
func parseSubscribeEvent(id pane.PaneID, line []byte) pane.OutputEvent {
	var raw zellijSubscribeEvent
	if err := json.Unmarshal(line, &raw); err != nil {
		// Unparseable line — treat as raw text output.
		return pane.OutputEvent{PaneID: id, Text: string(line)}
	}

	evt := pane.OutputEvent{PaneID: id}

	switch raw.Event {
	case "pane_closed":
		evt.Exited = true
	case "pane_update":
		if len(raw.Viewport) > 0 {
			evt.Text = strings.Join(raw.Viewport, "\n")
		} else {
			evt.Text = raw.Text
		}
		if raw.Exited {
			evt.Exited = true
			if raw.ExitCode != nil {
				evt.Code = *raw.ExitCode
			}
		}
	default:
		// Unknown event type — pass text through if available.
		evt.Text = raw.Text
		if raw.Exited {
			evt.Exited = true
			if raw.ExitCode != nil {
				evt.Code = *raw.ExitCode
			}
		}
	}

	return evt
}

// zellijSubscribeAvailable checks whether `zellij subscribe` is a known subcommand.
// We probe by running `zellij subscribe --help` and checking for a non-"unknown command" error.
func zellijSubscribeAvailable() bool {
	cmd := exec.Command("zellij", "subscribe", "--help")
	out, err := cmd.CombinedOutput()
	if err == nil {
		return true
	}
	// If the output mentions "unknown" we assume subscribe isn't available.
	lower := strings.ToLower(string(out))
	if strings.Contains(lower, "unknown") || strings.Contains(lower, "unrecognized") {
		return false
	}
	// Any other error (e.g. not in a session) — assume available.
	return true
}

// pollSubscribe is the fallback Subscribe implementation for older Zellij versions.
// It polls dump-screen every 200ms and emits OutputEvents on changes.
func pollSubscribe(ctx context.Context, b *ZellijBackend, id pane.PaneID, ch chan<- pane.OutputEvent) {
	defer close(ch)

	var last string
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			ch <- pane.OutputEvent{PaneID: id, Exited: true}
			return
		case <-ticker.C:
			content, err := b.Capture(id)
			if err != nil {
				// Pane likely gone.
				ch <- pane.OutputEvent{PaneID: id, Exited: true}
				return
			}
			if content != last {
				ch <- pane.OutputEvent{PaneID: id, Text: content}
				last = content
			}
			// Check if pane has exited.
			infos, err := b.List()
			if err != nil {
				continue
			}
			found := false
			for _, info := range infos {
				if info.ID == id {
					found = true
					if !info.Running {
						ch <- pane.OutputEvent{PaneID: id, Exited: true}
						return
					}
					break
				}
			}
			if !found {
				ch <- pane.OutputEvent{PaneID: id, Exited: true}
				return
			}
		}
	}
}

// zellijPaneJSON is the JSON schema for a single entry from `zellij action list-panes --json`.
type zellijPaneJSON struct {
	ID         int    `json:"id"`
	IsPlugin   bool   `json:"is_plugin"`
	IsFocused  bool   `json:"is_focused"`
	Title      string `json:"title"`
	Exited     bool   `json:"exited"`
	ExitStatus *int   `json:"exit_status"` // null = still running
}

// List returns all non-plugin panes in the current Zellij session.
func (b *ZellijBackend) List() ([]pane.PaneInfo, error) {
	out, err := exec.Command("zellij", "action", "list-panes", "--json").Output()
	if err != nil {
		return nil, fmt.Errorf("zellij list-panes: %w", err)
	}
	return parsePaneList(out)
}

// parsePaneList parses the JSON output of `zellij action list-panes --json`.
// Extracted for unit testing without a real Zellij instance.
func parsePaneList(data []byte) ([]pane.PaneInfo, error) {
	var raw []zellijPaneJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse list-panes JSON: %w", err)
	}

	var result []pane.PaneInfo
	for _, p := range raw {
		if p.IsPlugin {
			continue // skip plugin panes
		}
		info := pane.PaneInfo{
			ID:      pane.PaneID(fmt.Sprintf("terminal_%d", p.ID)),
			Name:    p.Title,
			Running: p.ExitStatus == nil, // null exit_status = still running
		}
		result = append(result, info)
	}
	return result, nil
}

// Close terminates a pane. Idempotent — "pane not found" errors are ignored.
func (b *ZellijBackend) Close(id pane.PaneID) error {
	n := paneInt(id)
	out, err := exec.Command("zellij", "action", "close-pane", "--pane-id", n).CombinedOutput()
	if err != nil {
		msg := strings.ToLower(string(out))
		if strings.Contains(msg, "not found") || strings.Contains(msg, "no pane") {
			return nil // already gone — idempotent
		}
		return fmt.Errorf("zellij close-pane: %w: %s", err, bytes.TrimSpace(out))
	}
	return nil
}

// Wait polls list-panes every 200ms until the pane exits and returns its exit code.
// Returns -1 if the pane disappears from the list before the exit_status becomes visible.
func (b *ZellijBackend) Wait(id pane.PaneID) (int, error) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		out, err := exec.Command("zellij", "action", "list-panes", "--json").Output()
		if err != nil {
			return -1, fmt.Errorf("zellij list-panes (wait): %w", err)
		}

		var raw []zellijPaneJSON
		if err := json.Unmarshal(out, &raw); err != nil {
			return -1, fmt.Errorf("zellij list-panes parse (wait): %w", err)
		}

		found := false
		for _, p := range raw {
			if pane.PaneID(fmt.Sprintf("terminal_%d", p.ID)) == id {
				found = true
				if p.ExitStatus != nil {
					return *p.ExitStatus, nil
				}
				break // still running
			}
		}

		if !found {
			// Pane disappeared before exit_status was visible.
			// This is expected when CloseOnExit is set — the pane closes cleanly.
			return 0, nil
		}
	}

	// ticker.C is unbuffered and Stop() drains it — unreachable but satisfies compiler.
	return -1, nil
}
