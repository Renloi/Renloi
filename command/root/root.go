package root

import (
	"fmt"
	"github.com/renloi/Renloi/command/backup"
	"github.com/renloi/Renloi/command/genesis"
	"github.com/renloi/Renloi/command/helper"
	"github.com/renloi/Renloi/command/ibft"
	"github.com/renloi/Renloi/command/license"
	"github.com/renloi/Renloi/command/loadbot"
	"github.com/renloi/Renloi/command/monitor"
	"github.com/renloi/Renloi/command/peers"
	"github.com/renloi/Renloi/command/secrets"
	"github.com/renloi/Renloi/command/server"
	"github.com/renloi/Renloi/command/status"
	"github.com/renloi/Renloi/command/txpool"
	"github.com/renloi/Renloi/command/version"
	"github.com/spf13/cobra"
	"os"
)

type RootCommand struct {
	baseCmd *cobra.Command
}

func NewRootCommand() *RootCommand {
	rootCommand := &RootCommand{
		baseCmd: &cobra.Command{
			Short: "Renloi is a Proof of Stake EVM-compatible blockchain network",
		},
	}

	helper.RegisterJSONOutputFlag(rootCommand.baseCmd)

	rootCommand.registerSubCommands()

	return rootCommand
}

func (rc *RootCommand) registerSubCommands() {
	rc.baseCmd.AddCommand(
		version.GetCommand(),
		txpool.GetCommand(),
		status.GetCommand(),
		secrets.GetCommand(),
		peers.GetCommand(),
		monitor.GetCommand(),
		loadbot.GetCommand(),
		ibft.GetCommand(),
		backup.GetCommand(),
		genesis.GetCommand(),
		server.GetCommand(),
		license.GetCommand(),
	)
}

func (rc *RootCommand) Execute() {
	if err := rc.baseCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)

		os.Exit(1)
	}
}
