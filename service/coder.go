package service

var (
	enableCodeExecution = false
)

func EnableCodeExecution() {
	enableCodeExecution = true
}

func DisableCodeExecution() {
	enableCodeExecution = false
}

func IsCodeExecutionEnabled() bool {
	return enableCodeExecution
}
