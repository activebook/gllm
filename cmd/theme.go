package cmd

import (
	"fmt"

	"github.com/activebook/gllm/data"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(themeCmd)
	themeCmd.AddCommand(themeListCmd)
	themeCmd.AddCommand(themeSwitchCmd)
	themeCmd.AddCommand(themePreviewCmd)
}

var themeCmd = &cobra.Command{
	Use:   "theme",
	Short: "Manage and switch themes",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Current Theme: %s\n", data.CurrentThemeName)
	},
}

var themeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available themes",
	Run: func(cmd *cobra.Command, args []string) {
		themes := data.ListThemes()
		for _, name := range themes {
			if name == data.CurrentThemeName {
				fmt.Printf("%s* %s%s\n", data.SwitchOnColor, name, data.ResetSeq)
			} else {
				fmt.Println(name)
			}
		}
	},
}

var themeSwitchCmd = &cobra.Command{
	Use:     "switch <name>",
	Aliases: []string{"sw"},
	Short:   "Switch to a different theme",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		err := data.LoadTheme(name)
		if err != nil {
			fmt.Printf("%sError: %v%s\n", data.StatusErrorColor, err, data.ResetSeq)
			return
		}

		err = data.SaveThemeConfig(name)
		if err != nil {
			fmt.Printf("%sWarning: Failed to save theme config: %v%s\n", data.StatusWarnColor, err, data.ResetSeq)
		}

		fmt.Printf("%sSuccessfully switched to theme: %s%s\n", data.StatusSuccessColor, name, data.ResetSeq)
	},
}

var themePreviewCmd = &cobra.Command{
	Use:   "preview <name>",
	Short: "Preview a theme without saving",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		err := data.LoadTheme(name)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		// Show preview using existing color command logic (but inline here or just simple output)
		fmt.Printf("Previewing Theme: %s\n", name)
		fmt.Println("--- Sample Output ---")
		fmt.Printf("%sSystem: You are a helpful assistant.%s\n", data.RoleSystemColor, data.ResetSeq)
		fmt.Printf("%sUser: Hello world!%s\n", data.RoleUserColor, data.ResetSeq)
		fmt.Printf("%sAssistant: Hi! I love this %s theme!%s\n", data.RoleAssistantColor, name, data.ResetSeq)
		fmt.Printf("%s[TOOL CALL] list_files()%s\n", data.ToolCallColor, data.ResetSeq)
		fmt.Printf("%sSuccess: Operation completed.%s\n", data.StatusSuccessColor, data.ResetSeq)
	},
}
