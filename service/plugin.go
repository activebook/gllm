package service

import (
	"github.com/spf13/viper"
)

var (
	// Create a response that prompts the LLM to provide insightful analysis
	ExecRespTmplInsight = `
Analyze the command below and provide an insightful response to the user. Consider:
Whether the command is dangerous and harmful and necessary and useful
`
	// ExecRespTmplConfirm is the template for the response to the user before executing a command.
	ExecRespTmplConfirm = "Based on your request, I've prepared the following command to execute on your system:\n\n" +
		"```\n%s\n```\n\n" +
		"This command will %s\n\n" +
		`**Check:** 
- First, Proceed with caution. Check whether this command may make irreversible changes or affect critical parts of your system. 
- Second, Ensure that the command is safe and does not contain any malicious or harmful actions.
- Third, If you deem it's safe, execute the command. If not, let user to choose whether to proceed or modify the command.
Such as:
Would you like me to run this command for you? Please confirm with 'yes', 'proceed', or provide alternative instructions.
`

	// ExecRespTmplOutput is the template for the response to the user after executing a command.
	ExecRespTmplOutput = `Command executed: %s
Status:
%s
%s

Analyze the execution result above and provide an insightful response to the user. Consider:
1. Whether the command succeeded or failed
2. If output is empty but status is success, explain what this likely means
3. What the output means, if present
4. What follow-up actions might be useful (e.g., verification commands)
5. Any potential issues or warnings based on the output
Keep your analysis concise and helpful.`
)

func IsExecPluginLoaded() bool {
	execPlugin := "plugins." + "exec" + ".loaded"
	var loaded bool
	if !viper.IsSet(execPlugin) {
		loaded = true
	} else {
		loaded = viper.GetBool(execPlugin)
	}
	return loaded
}
