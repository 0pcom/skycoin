/*
skycoin daemon, cli, & newcoin
*/
package commands

import (
	"log"
	"fmt"
	"os"
	"strings"
	"path/filepath"
	"github.com/spf13/cobra"
	skycoin	"github.com/skycoin/skycoin/cmd/skycoin/commands"
	cli	"github.com/skycoin/skycoin/cmd/skycoin-cli/commands"
	newcoin	"github.com/skycoin/skycoin/cmd/newcoin/commands"
)
func init() {

	RootCmd.AddCommand(
		skycoin.RootCmd,
		cli.RootCmd,
		newcoin.RootCmd,
	)
	skycoin.RootCmd.Use="daemon"
}

// RootCmd contains every daemon, cli, & newcoin
var RootCmd = &cobra.Command{
	Use: func() string {
		return strings.Split(filepath.Base(strings.ReplaceAll(strings.ReplaceAll(fmt.Sprintf("%v", os.Args), "[", ""), "]", "")), " ")[0]
	}(),
	Long: `
	┌─┐┬┌─┬ ┬┌─┐┌─┐┬┌┐┌
	└─┐├┴┐└┬┘│  │ │││││
	└─┘┴ ┴ ┴ └─┘└─┘┴┘└┘`,
	SilenceErrors:         true,
	SilenceUsage:          true,
	DisableSuggestions:    true,
	DisableFlagsInUseLine: true,
}

// Execute executes root CLI command.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		log.Fatal("Failed to execute command: ", err)
	}
}
