//go:build with_tools_generate

package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/sagernet/sing-box/experimental/tools_generate"
	"github.com/sagernet/sing-box/log"
)

var commandToolsGenerate = &cobra.Command{
	Use:  "generate <config>",
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		err := toolsGenerate(args[0])
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	commandTools.AddCommand(commandToolsGenerate)
}

func toolsGenerate(configPath string) error {
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	config, err := tools_generate.Parse(configBytes)
	if err != nil {
		return err
	}

	singboxConfigBytes, err := tools_generate.GenerateSingBoxConfig(config)
	if err != nil {
		return err
	}

	singboxConfigFile, err := os.Create(config.SingBox.Output)
	if err != nil {
		return err
	}
	defer func() {
		_ = singboxConfigFile.Close()
	}()

	_, err = singboxConfigFile.Write(singboxConfigBytes)
	if err != nil {
		return err
	}

	return nil
}
