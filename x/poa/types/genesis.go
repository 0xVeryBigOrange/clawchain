package types

// DefaultGenesis 返回默认创世状态
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:       DefaultParams(),
		Miners:       []Miner{},
		CurrentEpoch: 0,
	}
}

// GenesisState 定义 poa 模块的创世状态
type GenesisState struct {
	Params       Params  `json:"params"`        // 模块参数
	Miners       []Miner `json:"miners"`        // 矿工列表
	CurrentEpoch uint64  `json:"current_epoch"` // 当前 epoch
}

// Validate 执行创世状态基础验证
func (gs GenesisState) Validate() error {
	if err := gs.Params.Validate(); err != nil {
		return err
	}

	// 验证矿工地址唯一性
	minerAddresses := make(map[string]bool)
	for _, miner := range gs.Miners {
		if minerAddresses[miner.Address] {
			return ErrMinerAlreadyExists.Wrapf("duplicate miner address: %s", miner.Address)
		}
		minerAddresses[miner.Address] = true
	}

	return nil
}
