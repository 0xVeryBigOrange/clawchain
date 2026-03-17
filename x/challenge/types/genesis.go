package types

// GenesisState 挑战模块创世状态
type GenesisState struct {
	Params Params `json:"params"`
}

// DefaultGenesis 默认创世
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params: DefaultChallengeParams(),
	}
}

// Validate 验证
func (gs GenesisState) Validate() error {
	return nil
}
