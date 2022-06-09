package root

import (
	"fmt"
	"github.com/Renloi/Renloi/command/backup"
	"github.com/Renloi/Renloi/command/genesis"
	"github.com/Renloi/Renloi/command/helper"
	"github.com/Renloi/Renloi/command/ibft"
	"github.com/Renloi/Renloi/command/license"
	"github.com/Renloi/Renloi/command/loadbot"
	"github.com/Renloi/Renloi/command/monitor"
	"github.com/Renloi/Renloi/command/peers"
	"github.com/Renloi/Renloi/command/secrets"
	"github.com/Renloi/Renloi/command/server"
	"github.com/Renloi/Renloi/command/status"
	"github.com/Renloi/Renloi/command/txpool"
	"github.com/Renloi/Renloi/command/version"
	"github.com/spf13/cobra"
	"os"
)

type RootCommand struct {
	baseCmd *cobra.Command
}

func NewRootCommand() *RootCommand {
	rootCommand := &RootCommand{
		baseCmd: &cobra.Command{
			Short: "Renloi is an EVM-Comatatable Proof of Stake Blockchain Network.",
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
