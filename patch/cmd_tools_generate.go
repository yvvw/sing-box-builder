//go:build with_generate_tool

package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/sagernet/sing-box/experimental/generate_tool"
	"github.com/sagernet/sing-box/log"
)

var commandToolGenerate = &cobra.Command{
	Use:  "generate <config>",
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		err := toolGenerate(args[0])
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	commandTools.AddCommand(commandToolGenerate)
}

func toolGenerate(configPath string) error {
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	config, err := generate_tool.Parse(configBytes)
	if err != nil {
		return err
	}

	singboxConfigBytes, err := generate_tool.GenerateSingBoxConfig(config)
	if err != nil {
		return err
	}

	singboxFile, err := os.Create(config.SingBox.Output)
	if err != nil {
		return err
	}
	defer func() {
		_ = singboxFile.Close()
	}()

	_, err = singboxFile.Write(singboxConfigBytes)
	if err != nil {
		return err
	}

	return nil
}
