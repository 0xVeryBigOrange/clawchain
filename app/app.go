package app

import (
	"io"

	dbm "github.com/cosmos/cosmos-db"
	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/baseapp"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"

	"github.com/clawchain/clawchain/x/poa/keeper"
	challengekeeper "github.com/clawchain/clawchain/x/challenge/keeper"
	reputationkeeper "github.com/clawchain/clawchain/x/reputation/keeper"
)

// ClawChainApp is the main application struct for ClawChain.
type ClawChainApp struct {
	*baseapp.BaseApp

	// Keepers — custom ClawChain modules
	PoAKeeper        keeper.Keeper
	ChallengeKeeper  challengekeeper.Keeper
	ReputationKeeper reputationkeeper.Keeper
}

// NewClawChainApp creates and returns a new ClawChainApp instance.
func NewClawChainApp(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	loadLatest bool,
	appOpts servertypes.AppOptions,
) *ClawChainApp {
	encodingConfig := MakeEncodingConfig()
	bApp := baseapp.NewBaseApp(AppName, logger, db, encodingConfig.TxConfig.TxDecoder())
	bApp.SetVersion("0.1.0")

	app := &ClawChainApp{
		BaseApp: bApp,
	}

	return app
}

// Name returns the name of the app.
func (app *ClawChainApp) Name() string { return AppName }
