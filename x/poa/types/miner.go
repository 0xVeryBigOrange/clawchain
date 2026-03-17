package types

import "time"

// MinerStatus 矿工状态
type MinerStatus int32

const (
	MinerStatusInactive  MinerStatus = 0 // 未激活
	MinerStatusActive    MinerStatus = 1 // 活跃
	MinerStatusUnstaking MinerStatus = 2 // 解质押中
	MinerStatusSuspended MinerStatus = 3 // 被暂停
)

// Miner 矿工信息
type Miner struct {
	Address             string      `json:"address"`
	StakeAmount         uint64      `json:"stake_amount"`
	Status              MinerStatus `json:"status"`
	RegisteredAt        time.Time   `json:"registered_at"`
	ReputationScore     int32       `json:"reputation_score"`
	TotalRewards        uint64      `json:"total_rewards"`
	ChallengesCompleted uint64      `json:"challenges_completed"`
	ChallengesFailed    uint64      `json:"challenges_failed"`
	LastActiveEpoch     uint64      `json:"last_active_epoch"`
}

// UnstakeEntry 解质押队列条目
type UnstakeEntry struct {
	MinerAddress   string    `json:"miner_address"`
	Amount         uint64    `json:"amount"`
	CompletionTime time.Time `json:"completion_time"`
}

// EpochInfo Epoch 信息
type EpochInfo struct {
	EpochNumber         uint64 `json:"epoch_number"`
	StartHeight         int64  `json:"start_height"`
	EndHeight           int64  `json:"end_height"`
	RewardPerEpoch      uint64 `json:"reward_per_epoch"`
	ActiveMiners        uint32 `json:"active_miners"`
	ChallengesIssued    uint32 `json:"challenges_issued"`
	ChallengesCompleted uint32 `json:"challenges_completed"`
}
