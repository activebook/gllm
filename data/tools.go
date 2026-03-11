package data

type ToolConfirmResult int

const (
	ToolConfirmYes    ToolConfirmResult = iota // Approve this tool call
	ToolConfirmCancel                          // Cancel entire operation immediately
)

type ToolsUse struct {
	AutoApprove bool              // Whether tools can be used without user confirmation
	Confirm     ToolConfirmResult // User confirmation result
}

func (tu *ToolsUse) ConfirmOnce() {
	tu.Confirm = ToolConfirmYes
	tu.AutoApprove = false
}

func (tu *ToolsUse) ConfirmAlways() {
	tu.Confirm = ToolConfirmYes
	tu.AutoApprove = true
}

func (tu *ToolsUse) ConfirmCancel() {
	tu.Confirm = ToolConfirmCancel
	tu.AutoApprove = false
}

var (
	// Whether tools can be used without user confirmation in the current session
	toolCallAutoApproveInSession = false
	// Whether Plan Mode is enabled in the current session
	planModeInSession = false
)

/**
 * Set tool call auto approve in session
 */
func SetToolCallAutoApproveInSession(value bool) {
	toolCallAutoApproveInSession = value
}

/**
 * Get tool call auto approve in session
 */
func GetToolCallAutoApproveInSession() bool {
	return toolCallAutoApproveInSession
}

/**
 * Set plan mode in session
 */
func SetPlanModeInSession(value bool) {
	planModeInSession = value
}

/**
 * Get plan mode in session
 */
func GetPlanModeInSession() bool {
	return planModeInSession
}
