package protocol

import (
	"context"
	"fmt"

	"github.com/datazip-inc/olake/logger/console"
	"github.com/datazip-inc/olake/types"
	"github.com/datazip-inc/olake/utils"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	configPath            string
	destinationConfigPath string
	statePath             string
	catalogPath           string
	batchSize             int64

	catalog           *types.Catalog
	state             *types.State
	destinationConfig *types.WriterConfig

	commands  = []*cobra.Command{}
	connector Driver

	concurrentStreamExecution = 6
	// Global Stream concurrency group;
	//
	// Not to confuse with individual stream level concurrency
	GlobalCxGroup = utils.NewCGroupWithLimit(context.Background(), concurrentStreamExecution)
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "olake",
	Short: "root command",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}

		if ok := utils.IsValidSubcommand(commands, args[0]); !ok {
			return fmt.Errorf("'%s' is an invalid command. Use 'olake --help' to display usage guide", args[0])
		}

		return nil
	},
}

func CreateRootCommand(_ bool, driver any) *cobra.Command {
	RootCmd.AddCommand(commands...)
	connector = driver.(Driver)

	return RootCmd
}

func init() {
	commands = append(commands, specCmd, checkCmd, discoverCmd, syncCmd)
	RootCmd.PersistentFlags().StringVarP(&configPath, "config", "", "", "(Required) Config for connector")
	RootCmd.PersistentFlags().StringVarP(&destinationConfigPath, "destination", "", "", "(Required) Destination config for connector")
	RootCmd.PersistentFlags().StringVarP(&catalogPath, "catalog", "", "", "(Required) Catalog for connector")
	RootCmd.PersistentFlags().StringVarP(&statePath, "state", "", "", "(Required) State for connector")
	RootCmd.PersistentFlags().Int64VarP(&batchSize, "batch", "", 10000, "(Optional) Batch size for connector")

	// Disable Cobra CLI's built-in usage and error handling
	RootCmd.SilenceUsage = true
	RootCmd.SilenceErrors = true

	// Disable logging
	logrus.SetOutput(nil)

	console.SetupWriter(RootCmd.OutOrStdout(), RootCmd.ErrOrStderr())
}
