package app

import (
	"encoding/json"

	"github.com/cosmos/cosmos-sdk/codec"

	distr "github.com/cosmos/cosmos-sdk/x/distribution"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/cosmos/cosmos-sdk/x/mint"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// GenesisState 是所有模块的创世状态集合
type GenesisState map[string]json.RawMessage

// NewDefaultGenesisState 生成默认创世状态
func NewDefaultGenesisState() GenesisState {
	encCfg := MakeEncodingConfig()
	return ModuleBasics.DefaultGenesis(encCfg.Codec)
}

// AddExtraModuleDefaults 向 genesis 添加不在 BasicManager 中的模块的默认状态
// （staking/distr/mint/consensus 因为 CLI codec 限制不在 BasicManager 中）
func AddExtraModuleDefaults(cdc codec.JSONCodec, genesis GenesisState) GenesisState {
	// staking
	stakingBasic := staking.AppModuleBasic{}
	if _, ok := genesis[stakingtypes.ModuleName]; !ok {
		genesis[stakingtypes.ModuleName] = stakingBasic.DefaultGenesis(cdc)
	}
	// distribution
	distrBasic := distr.AppModuleBasic{}
	if _, ok := genesis[distrtypes.ModuleName]; !ok {
		genesis[distrtypes.ModuleName] = distrBasic.DefaultGenesis(cdc)
	}
	// mint
	mintBasic := mint.AppModuleBasic{}
	if _, ok := genesis[minttypes.ModuleName]; !ok {
		genesis[minttypes.ModuleName] = mintBasic.DefaultGenesis(cdc)
	}

	return genesis
}
