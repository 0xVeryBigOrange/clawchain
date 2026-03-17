package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/clawchain/clawchain/app"
)

func main() {
	app.SetConfig()

	rootCmd := &cobra.Command{
		Use:   "clawchaind",
		Short: "ClawChain - Proof of Availability AI Agent Blockchain",
		Long: `ClawChain daemon — A Cosmos SDK blockchain for AI Agent mining
via Proof of Availability consensus.

Chain ID: clawchain-testnet-1
Token: $CLAW (uclaw)
Bech32: claw`,
	}

	rootCmd.AddCommand(versionCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version of clawchaind",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("clawchaind v0.1.0")
			fmt.Println("Chain ID: " + app.ChainID)
			fmt.Println("Denom: " + app.BondDenom)
			fmt.Println("Bech32 Prefix: " + app.AccountAddressPrefix)
		},
	}
}
