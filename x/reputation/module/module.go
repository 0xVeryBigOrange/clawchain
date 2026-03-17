package module

import (
	"encoding/json"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/clawchain/clawchain/x/reputation/keeper"
	"github.com/clawchain/clawchain/x/reputation/types"
)

var (
	
)

type AppModuleBasic struct{}

func (AppModuleBasic) Name() string { return types.ModuleName }

func (AppModuleBasic) RegisterLegacyAminoCodec(_ *codec.LegacyAmino) {}

func (AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	gs := types.DefaultGenesis()
	bz, _ := json.Marshal(gs)
	return bz
}

func (AppModuleBasic) ValidateGenesis(_ codec.JSONCodec, _ interface{}, bz json.RawMessage) error {
	var gs types.GenesisState
	return json.Unmarshal(bz, &gs)
}

func (AppModuleBasic) RegisterGRPCGatewayRoutes(_ interface{}, _ interface{}) {}

func (AppModuleBasic) RegisterInterfaces(_ interface{}) {}

type AppModule struct {
	AppModuleBasic
	keeper keeper.Keeper
}

func NewAppModule(keeper keeper.Keeper) AppModule {
	return AppModule{keeper: keeper}
}

func (am AppModule) RegisterServices(_ interface{}) {}

func (am AppModule) InitGenesis(ctx sdk.Context, _ codec.JSONCodec, data json.RawMessage) {
	var gs types.GenesisState
	json.Unmarshal(data, &gs)
	am.keeper.InitGenesis(ctx, gs)
}

func (am AppModule) ExportGenesis(ctx sdk.Context, _ codec.JSONCodec) json.RawMessage {
	gs := am.keeper.ExportGenesis(ctx)
	bz, _ := json.Marshal(gs)
	return bz
}

func (am AppModule) ConsensusVersion() uint64 { return 1 }

func (am AppModule) IsOnePerModuleType() {}
func (am AppModule) IsAppModule()        {}
