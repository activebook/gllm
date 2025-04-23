package cmd

import (
	"fmt"

	"github.com/activebook/gllm/service"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	defaultLoadedPlugin = "exec"
	availablePlugins    = []string{"exec" /*"search", "vision"*/} // Example plugins
	loadedPlugins       = map[string]bool{}
)

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Manage plugins",
	Long: `Manage plugins.

- exec plugin give gllm the ablity to execute commands.
- it will require yours confirm before executing any command.
- exec plugin is loaded by default.
- using unload exec to unload it.`,
}

var pluginListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls", "pr", "print"},
	Short:   "List all plugins",
	Run: func(cmd *cobra.Command, args []string) {
		for _, name := range availablePlugins {
			loaded := viper.GetBool("plugins." + name + ".loaded")
			if name == defaultLoadedPlugin && !viper.IsSet("plugins."+name+".loaded") {
				loaded = true
			}
			loadedPlugins[name] = loaded
			if loadedPlugins[name] {
				fmt.Printf("[✔] %s\n", name)
			} else {
				fmt.Printf("[ ] %s\n", name)
			}
		}
		fmt.Println("\n[✔] Indicates that a plugin is loaded.")
	},
}

var pluginLoadCmd = &cobra.Command{
	Use:   "load [name]",
	Short: "Load a plugin",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		if !service.Contains(availablePlugins, name) {
			service.Warnf("Plugin '%s' not found.\n", name)
			return
		}
		loadedPlugins[name] = true
		viper.Set("plugins."+name+".loaded", true)
		err := viper.WriteConfig()
		if err != nil {
			service.Errorf("Error writing config:", err)
			return
		}
		fmt.Printf("Loaded plugin: %s\n", name)
	},
}

var pluginUnloadCmd = &cobra.Command{
	Use:   "unload [name]",
	Short: "Unload a plugin",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		if !loadedPlugins[name] {
			service.Warnf("Plugin '%s' is not loaded.\n", name)
			return
		}
		delete(loadedPlugins, name)
		viper.Set("plugins."+name+".loaded", false)
		err := viper.WriteConfig()
		if err != nil {
			service.Errorf("Error writing config:", err)
			return
		}
		fmt.Printf("Unloaded plugin: %s\n", name)
	},
}

func loadPlugins() {
	for _, name := range availablePlugins {
		loaded := viper.GetBool("plugins." + name + ".loaded")
		if name == defaultLoadedPlugin && !viper.IsSet("plugins."+name+".loaded") {
			loaded = true
		}
		loadedPlugins[name] = loaded
	}
}

func init() {
	pluginCmd.AddCommand(pluginListCmd)
	pluginCmd.AddCommand(pluginLoadCmd)
	pluginCmd.AddCommand(pluginUnloadCmd)
	rootCmd.AddCommand(pluginCmd)

	// Initialize loadedPlugins map
	loadPlugins()
}

func GetLoadedPlugins() map[string]bool {
	return loadedPlugins
}
