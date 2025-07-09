package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github-okr-fetcher/internal/adapters/config"
)

var generateCmd = &cobra.Command{
	Use:   "generate-config",
	Short: "Generate an example configuration file",
	Long: `Generate an example configuration file with default values and comments.
This will create a config.json file in the current directory that you can
customize with your GitHub token, project URL, and other settings.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configRepo := config.NewRepository()
		
		configPath := "config.json"
		if configFile != "" {
			configPath = configFile
		}
		
		if err := configRepo.GenerateExampleConfig(configPath); err != nil {
			return fmt.Errorf("error generating config file: %v", err)
		}
		
		fmt.Printf("✅ Example config file generated: %s\n", configPath)
		fmt.Println("Please edit the file with your project URL and set GITHUB_TOKEN environment variable")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)
}