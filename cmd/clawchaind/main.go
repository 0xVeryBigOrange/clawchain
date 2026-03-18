package main

import (
	"fmt"
	"io"
	"os"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/spf13/cobra"

	"cosmossdk.io/log"

	cmtcfg "github.com/cometbft/cometbft/config"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/config"
	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/cosmos-sdk/server"
	serverconfig "github.com/cosmos/cosmos-sdk/server/config"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
	genutilcli "github.com/cosmos/cosmos-sdk/x/genutil/client/cli"

	"github.com/clawchain/clawchain/app"
)

func main() {
	app.SetConfig()

	rootCmd := NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// NewRootCmd 创建根命令
func NewRootCmd() *cobra.Command {
	encodingConfig := app.MakeEncodingConfig()

	initClientCtx := client.Context{}.
		WithCodec(encodingConfig.Codec).
		WithInterfaceRegistry(encodingConfig.InterfaceRegistry).
		WithTxConfig(encodingConfig.TxConfig).
		WithLegacyAmino(encodingConfig.Amino).
		WithInput(os.Stdin).
		WithAccountRetriever(types.AccountRetriever{}).
		WithHomeDir(app.DefaultNodeHome).
		WithViper("")

	rootCmd := &cobra.Command{
		Use:   "clawchaind",
		Short: "ClawChain - Proof of Availability AI Agent Blockchain",
		Long: `ClawChain daemon — A Cosmos SDK blockchain for AI Agent mining
via Proof of Availability consensus.

Chain ID: clawchain-testnet-1
Token: $CLAW (uclaw)
Bech32: claw`,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SetOut(cmd.OutOrStdout())
			cmd.SetErr(cmd.ErrOrStderr())

			initClientCtx, err := client.ReadPersistentCommandFlags(initClientCtx, cmd.Flags())
			if err != nil {
				return err
			}

			initClientCtx, err = config.ReadFromClientConfig(initClientCtx)
			if err != nil {
				return err
			}

			if err := client.SetCmdClientContextHandler(initClientCtx, cmd); err != nil {
				return err
			}

			customAppTemplate, customAppConfig := initAppConfig()
			customCMTConfig := initCometBFTConfig()

			return server.InterceptConfigsPreRunHandler(cmd, customAppTemplate, customAppConfig, customCMTConfig)
		},
	}

	initRootCmd(rootCmd, encodingConfig, app.ModuleBasics)
	return rootCmd
}

// initRootCmd 初始化根命令的所有子命令
func initRootCmd(
	rootCmd *cobra.Command,
	encodingConfig app.EncodingConfig,
	basicManager module.BasicManager,
) {
	rootCmd.AddCommand(
		genutilcli.InitCmd(app.AllModuleBasics(), app.DefaultNodeHome),
		genutilcli.Commands(encodingConfig.TxConfig, app.AllModuleBasics(), app.DefaultNodeHome),
		keys.Commands(),
	)

	server.AddCommands(rootCmd, app.DefaultNodeHome, newApp, appExport, addModuleInitFlags)

	// add query and tx commands
	queryCmd := &cobra.Command{
		Use:                        "query",
		Aliases:                    []string{"q"},
		Short:                      "Querying subcommands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	txCmd := &cobra.Command{
		Use:                        "tx",
		Short:                      "Transactions subcommands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	// ModuleBasics 不包含 staking/distr/mint（它们需要 codec），所以这里安全
	basicManager.AddQueryCommands(queryCmd)
	basicManager.AddTxCommands(txCmd)

	rootCmd.AddCommand(queryCmd, txCmd)
}

// newApp 创建新的应用实例
func newApp(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	appOpts servertypes.AppOptions,
) servertypes.Application {
	return app.NewClawChainApp(
		logger,
		db,
		traceStore,
		true,
		appOpts,
	)
}

// appExport 导出应用状态
func appExport(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	height int64,
	forZeroHeight bool,
	jailAllowedAddrs []string,
	appOpts servertypes.AppOptions,
	modulesToExport []string,
) (servertypes.ExportedApp, error) {
	var clawApp *app.ClawChainApp
	if height != -1 {
		clawApp = app.NewClawChainApp(logger, db, traceStore, false, appOpts)
		if err := clawApp.LoadHeight(height); err != nil {
			return servertypes.ExportedApp{}, err
		}
	} else {
		clawApp = app.NewClawChainApp(logger, db, traceStore, true, appOpts)
	}

	return clawApp.ExportAppStateAndValidators(forZeroHeight, jailAllowedAddrs, modulesToExport)
}

// addModuleInitFlags 添加模块初始化标志
func addModuleInitFlags(startCmd *cobra.Command) {
	// Add any module-specific init flags here if needed
}

// initAppConfig 初始化应用配置
func initAppConfig() (string, interface{}) {
	type CustomAppConfig struct {
		serverconfig.Config
	}

	srvCfg := serverconfig.DefaultConfig()
	// 设置默认最小 gas 价格
	srvCfg.MinGasPrices = "0uclaw"

	customAppConfig := CustomAppConfig{
		Config: *srvCfg,
	}

	customAppTemplate := serverconfig.DefaultConfigTemplate

	return customAppTemplate, customAppConfig
}

// initCometBFTConfig 初始化 CometBFT 配置
func initCometBFTConfig() *cmtcfg.Config {
	return cmtcfg.DefaultConfig()
}
