package types

// PoAKeeper 从 x/poa 需要的接口
type PoAKeeper interface {
	// GetActiveMinersAddresses 获取活跃矿工地址列表
	GetActiveMinersAddresses() []string
}
