package types

const (
	// ModuleName 模块名称
	ModuleName = "poa"

	// StoreKey 存储键
	StoreKey = ModuleName

	// RouterKey 路由键
	RouterKey = ModuleName

	// QuerierRoute 查询路由
	QuerierRoute = ModuleName
)

// 存储前缀
var (
	// MinerKeyPrefix 矿工信息前缀
	MinerKeyPrefix = []byte{0x01}

	// StakeKeyPrefix 质押信息前缀
	StakeKeyPrefix = []byte{0x02}

	// EpochKeyPrefix Epoch 信息前缀
	EpochKeyPrefix = []byte{0x03}

	// RewardKeyPrefix 奖励记录前缀
	RewardKeyPrefix = []byte{0x04}

	// UnstakeQueueKeyPrefix 解质押队列前缀
	UnstakeQueueKeyPrefix = []byte{0x05}

	// ParamsKey 参数存储键
	ParamsKey = []byte{0x06}
)

// GetMinerKey 获取矿工存储键
func GetMinerKey(addr string) []byte {
	return append(MinerKeyPrefix, []byte(addr)...)
}

// GetStakeKey 获取质押存储键
func GetStakeKey(addr string) []byte {
	return append(StakeKeyPrefix, []byte(addr)...)
}
