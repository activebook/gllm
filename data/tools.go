package data

type ToolConfirmResult int

const (
	ToolConfirmYes    ToolConfirmResult = iota // Approve this tool call
	ToolConfirmCancel                          // Cancel entire operation immediately
)

type ToolsUse struct {
	AutoApprove bool              // Whether tools can be used without user confirmation
	Confirm     ToolConfirmResult // User confirmation result
	AgentName   string            // Name of the currently executing agent
	FilePath    string            // File path relevant to the tool use, if any
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
