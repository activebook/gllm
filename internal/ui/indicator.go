package ui

import (
	"fmt"

	"github.com/activebook/gllm/data"
)

// FormatEnabledIndicator returns a consistent enabled/disabled bracket indicator
// When enabled: [✔] with green color
// When disabled: [ ] with no color
func FormatEnabledIndicator(enabled bool) string {
	if enabled {
		return fmt.Sprintf("[%s✔%s]", data.SwitchOnColor, data.ResetSeq)
	}
	return "[ ]"
}
