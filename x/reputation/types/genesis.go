package types

// GenesisState 声誉模块创世状态
type GenesisState struct {
	Scores []ReputationScore `json:"scores"`
}

// DefaultGenesis 默认创世
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Scores: []ReputationScore{},
	}
}

// Validate 验证
func (gs GenesisState) Validate() error {
	return nil
}
