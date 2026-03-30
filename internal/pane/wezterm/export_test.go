// export_test.go exposes internal helpers for the external test package.
package wezterm

import "github.com/justEstif/openswarm/internal/pane"

// ParseWeztermListForTest is the test-only export of parseWeztermList.
func ParseWeztermListForTest(jsonData string) ([]pane.PaneInfo, error) {
	return parseWeztermList(jsonData)
}
