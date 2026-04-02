// Package tmux implements the pane.Backend interface using tmux.
//
// Import this package with a blank import to register the "tmux" driver:
//
//	import _ "github.com/justEstif/openswarm/internal/pane/tmux"
package tmux

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/justEstif/openswarm/internal/pane"
)

func init() { pane.Register("tmux", func() pane.Backend { return &TmuxBackend{} }) }

// TmuxBackend implements pane.Backend using tmux CLI commands.
type TmuxBackend struct{}

// Name returns the registered backend name.
func (b *TmuxBackend) Name() string { return "tmux" }

// buildEnvCmd constructs a shell command string with an env prefix.
//
// Output format:  env KEY1=VAL1 KEY2=VAL2 sh -c '<cmd>'
// Keys are sorted for deterministic output (useful in tests).
// Values and cmd are single-quoted so special characters are safe.
func buildEnvCmd(cmd string, env map[string]string) string {
	var parts []string
	if len(env) > 0 {
		parts = append(parts, "env")
		keys := make([]string, 0, len(env))
		for k := range env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			parts = append(parts, k+"="+singleQuote(env[k]))
		}
	}
	parts = append(parts, "sh", "-c", singleQuote(cmd))
	return strings.Join(parts, " ")
}

// singleQuote wraps s in POSIX single quotes, escaping any literal single
// quotes inside so the result is always safe in a shell context.
func singleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// tmuxOutput runs a tmux command and returns trimmed stdout.
func tmuxOutput(args ...string) (string, error) {
	out, err := exec.Command("tmux", args...).Output()
	if err != nil {
		return "", fmt.Errorf("tmux %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// tmuxRun runs a tmux command and returns any error, ignoring stdout.
func tmuxRun(args ...string) error {
	if err := exec.Command("tmux", args...).Run(); err != nil {
		return fmt.Errorf("tmux %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

// Spawn creates a new tmux pane running cmd.
//
// opts.Placement controls where the pane is created:
//   - PlacementCurrentTab: split-window in the active window (split pane).
//   - PlacementNewTab (default): new-window in the current session (or a new
//     detached session when not inside tmux).
//   - PlacementNewSession: always creates a fresh detached session.
//
// When opts.CloseOnExit is true a shell cleanup trailer is appended to the
// command so the container (pane / window / session) self-destructs on exit.
// remain-on-exit is set only when CloseOnExit is false, so Wait() can read
// the exit code after the process finishes.
func (b *TmuxBackend) Spawn(name, cmd string, opts pane.SpawnOptions) (pane.PaneID, error) {
	fullCmd := buildEnvCmd(cmd, opts.Env)
	if opts.CloseOnExit {
		fullCmd = appendCleanupTrailer(fullCmd, opts.Placement)
	}

	var rawID string
	var err error

	switch opts.Placement {
	case pane.PlacementCurrentTab:
		if os.Getenv("TMUX") != "" {
			// Split a pane within the active window.
			rawID, err = tmuxOutput("split-window", "-d", "-P", "-F", "#{pane_id}", fullCmd)
		} else {
			// No active tmux session — fall back to a new detached session.
			sessionName := fmt.Sprintf("swarm-%d", rand.Int63()) //nolint:gosec
			rawID, err = tmuxOutput("new-session", "-d", "-s", sessionName, "-P", "-F", "#{pane_id}", fullCmd)
		}
	case pane.PlacementNewSession:
		sessionName := fmt.Sprintf("swarm-%s", name)
		rawID, err = tmuxOutput("new-session", "-d", "-s", sessionName, "-P", "-F", "#{pane_id}", fullCmd)
	default: // PlacementNewTab or ""
		if os.Getenv("TMUX") != "" {
			rawID, err = tmuxOutput("new-window", "-d", "-n", name, "-P", "-F", "#{pane_id}", fullCmd)
		} else {
			sessionName := fmt.Sprintf("swarm-%d", rand.Int63()) //nolint:gosec
			rawID, err = tmuxOutput("new-session", "-d", "-s", sessionName, "-P", "-F", "#{pane_id}", fullCmd)
		}
	}
	if err != nil {
		return "", fmt.Errorf("spawn pane: %w", err)
	}

	id := pane.PaneID(rawID)

	// remain-on-exit lets Wait() read the exit code. Not needed when CloseOnExit
	// is true: the cleanup trailer kills the container before we'd read it.
	if !opts.CloseOnExit {
		_ = tmuxRun("set-option", "-t", string(id), "remain-on-exit", "on")
	}

	return id, nil
}

// appendCleanupTrailer appends a shell snippet that destroys the pane's container
// (pane, window, or session) when the main command exits.
func appendCleanupTrailer(cmd string, placement pane.Placement) string {
	var trailer string
	switch placement {
	case pane.PlacementCurrentTab:
		// Kill just this pane (#D is the pane's unique ID).
		trailer = `tmux kill-pane -t "$(tmux display-message -p '#D')" 2>/dev/null || true`
	case pane.PlacementNewSession:
		// Kill the entire session.
		trailer = `tmux kill-session -t "$(tmux display-message -p '#{session_id}')" 2>/dev/null || true`
	default: // new_tab — kill the window
		trailer = `tmux kill-window -t "$(tmux display-message -p '#{window_id}')" 2>/dev/null || true`
	}
	return cmd + "; " + trailer
}

// Send delivers text to a pane's stdin using literal mode (-l flag).
// No newline is appended; send "\n" explicitly if needed.
func (b *TmuxBackend) Send(id pane.PaneID, text string) error {
	return tmuxRun("send-keys", "-t", string(id), "-l", text)
}

// Capture returns the current scrollback+viewport of a pane as plain text.
// Up to 500 lines of scrollback are included (-S -500).
func (b *TmuxBackend) Capture(id pane.PaneID) (string, error) {
	out, err := exec.Command("tmux", "capture-pane", "-t", string(id), "-p", "-S", "-500").Output()
	if err != nil {
		return "", fmt.Errorf("capture pane %s: %w", id, err)
	}
	return string(out), nil
}

// Subscribe streams output from a pane until it exits or ctx is cancelled.
//
// Implementation: polls Capture every 200 ms, diffs against the previous
// snapshot, and emits new content as OutputEvent.Text.  When the pane dies
// (#{pane_dead}==1) a final OutputEvent with Exited=true and the exit Code
// is sent before the channel is closed.
//
// Callers must drain the channel; a full channel blocks the goroutine.
func (b *TmuxBackend) Subscribe(ctx context.Context, id pane.PaneID) (<-chan pane.OutputEvent, error) {
	ch := make(chan pane.OutputEvent, 32)
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
				output, err := b.Capture(id)
				if err != nil {
					// Pane may have disappeared.
					return
				}

				if output != prev {
					delta := output
					if strings.HasPrefix(output, prev) {
						delta = output[len(prev):]
					}
					if delta != "" {
						select {
						case ch <- pane.OutputEvent{PaneID: id, Text: delta}:
						case <-ctx.Done():
							return
						}
					}
					prev = output
				}

				// Check if the pane process has exited.
				deadOut, err := exec.Command("tmux", "display-message",
					"-t", string(id), "-p", "#{pane_dead}").Output()
				if err != nil {
					// Pane no longer accessible.
					return
				}
				if strings.TrimSpace(string(deadOut)) == "1" {
					code := exitCode(id)
					select {
					case ch <- pane.OutputEvent{PaneID: id, Exited: true, Code: code}:
					case <-ctx.Done():
					}
					return
				}
			}
		}
	}()
	return ch, nil
}

// exitCode reads #{pane_dead_status} for id, returning -1 on any error.
func exitCode(id pane.PaneID) int {
	out, err := exec.Command("tmux", "display-message",
		"-t", string(id), "-p", "#{pane_dead_status}").Output()
	if err != nil {
		return -1
	}
	c, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return -1
	}
	return c
}

// parseListOutput parses the tab-separated output of
//
//	tmux list-panes -a -F '#{pane_id}\t#{window_name}\t#{pane_dead}\t#{pane_current_command}'
//
// into a slice of PaneInfo.  pane_dead=="0" means the pane is still running.
func parseListOutput(output string) ([]pane.PaneInfo, error) {
	var infos []pane.PaneInfo
	for _, line := range strings.Split(strings.TrimRight(output, "\n"), "\n") {
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, "\t", 4)
		if len(fields) < 4 {
			// Malformed line — skip gracefully.
			continue
		}
		infos = append(infos, pane.PaneInfo{
			ID:      pane.PaneID(fields[0]),
			Name:    fields[1],
			Running: fields[2] == "0",
			Command: fields[3],
		})
	}
	return infos, nil
}

// List returns all panes across all sessions known to this tmux server.
func (b *TmuxBackend) List() ([]pane.PaneInfo, error) {
	out, err := exec.Command("tmux", "list-panes", "-a", "-F",
		"#{pane_id}\t#{window_name}\t#{pane_dead}\t#{pane_current_command}").Output()
	if err != nil {
		return nil, fmt.Errorf("list panes: %w", err)
	}
	return parseListOutput(string(out))
}

// Close kills a pane.  Idempotent — returns nil if the pane is already gone.
func (b *TmuxBackend) Close(id pane.PaneID) error {
	cmd := exec.Command("tmux", "kill-pane", "-t", string(id))
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.ToLower(string(out))
		// Treat these as "pane is already gone" — idempotent success:
		//   - tmux prints "no pane" / "can't find pane" when the pane is gone
		//   - "no server" / "error connecting" / "failed to connect" when the
		//     tmux server exited after its last session was removed (happens in
		//     CI when closing the only pane tears down the server too)
		if strings.Contains(msg, "no pane") ||
			strings.Contains(msg, "can't find pane") ||
			strings.Contains(msg, "unknown pane") ||
			strings.Contains(msg, "no server") ||
			strings.Contains(msg, "error connecting") ||
			strings.Contains(msg, "failed to connect") {
			return nil
		}
		return fmt.Errorf("kill pane %s: %w", id, err)
	}
	return nil
}

// Wait blocks until the pane's process exits and returns its exit code.
//
// Polls #{pane_dead} every 200 ms.  Returns -1 if the pane disappears before
// remain-on-exit captures the status (e.g. if Spawn failed to set it).
func (b *TmuxBackend) Wait(id pane.PaneID) (int, error) {
	for {
		out, err := exec.Command("tmux", "display-message",
			"-t", string(id), "-p", "#{pane_dead}").Output()
		if err != nil {
			// Pane has gone away — exit code unavailable.
			return -1, nil
		}
		if strings.TrimSpace(string(out)) == "1" {
			return exitCode(id), nil
		}
		time.Sleep(200 * time.Millisecond)
	}
}
