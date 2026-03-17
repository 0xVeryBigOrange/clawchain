package app

import (
	"os"
	"path/filepath"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	AppName              = "clawchain"
	AccountAddressPrefix = "claw"
	BondDenom            = "uclaw"
	ChainID              = "clawchain-testnet-1"
	DisplayDenom         = "CLAW"
	CoinExponent         = 6
)

var DefaultNodeHome string

func init() {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	DefaultNodeHome = filepath.Join(userHomeDir, ".clawchain")
}

func SetConfig() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount(AccountAddressPrefix, AccountAddressPrefix+"pub")
	config.SetBech32PrefixForValidator(AccountAddressPrefix+"valoper", AccountAddressPrefix+"valoperpub")
	config.SetBech32PrefixForConsensusNode(AccountAddressPrefix+"valcons", AccountAddressPrefix+"valconspub")
	config.Seal()
}
