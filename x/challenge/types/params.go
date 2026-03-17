package types

// Params 挑战模块参数
type Params struct {
	// ChallengesPerEpoch 每 epoch 生成的挑战数
	ChallengesPerEpoch uint32 `json:"challenges_per_epoch"`
	// AssigneesPerChallenge 每个挑战分配的矿工数
	AssigneesPerChallenge uint32 `json:"assignees_per_challenge"`
	// ResponseWindowBlocks 响应窗口（区块数）
	ResponseWindowBlocks int64 `json:"response_window_blocks"`
	// CommitWindowBlocks commit 窗口
	CommitWindowBlocks int64 `json:"commit_window_blocks"`
	// RevealWindowBlocks reveal 窗口
	RevealWindowBlocks int64 `json:"reveal_window_blocks"`
}

// DefaultChallengeParams 默认参数
func DefaultChallengeParams() Params {
	return Params{
		ChallengesPerEpoch:    10,
		AssigneesPerChallenge: 3,
		ResponseWindowBlocks:  5,  // 5 blocks * 6s = 30s
		CommitWindowBlocks:    3,
		RevealWindowBlocks:    2,
	}
}
