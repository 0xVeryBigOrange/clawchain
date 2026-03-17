package types

import "time"

// 默认参数常量
const (
	DefaultMinStake              uint64 = 100_000_000   // 100 CLAW
	DefaultValidatorMinStake     uint64 = 10_000_000_000 // 10,000 CLAW
	DefaultEpochBlocks           int64  = 100            // 10 分钟
	DefaultHalvingEpochs         uint64 = 210_000        // ~4 年
	DefaultInitialReward         uint64 = 50_000_000     // 50 CLAW/epoch
	DefaultMinerRewardShare      uint32 = 60
	DefaultValidatorRewardShare  uint32 = 20
	DefaultEcoFundRewardShare    uint32 = 20
	DefaultMaxMinersPerIP        uint32 = 3
	DefaultNewMinerCooldownEpochs uint64 = 100
)

// DefaultUnstakeCooldown 7天
var DefaultUnstakeCooldown = 7 * 24 * time.Hour

// Params 模块参数
type Params struct {
	MinStake               uint64        `json:"min_stake"`
	ValidatorMinStake      uint64        `json:"validator_min_stake"`
	EpochBlocks            int64         `json:"epoch_blocks"`
	HalvingEpochs          uint64        `json:"halving_epochs"`
	InitialReward          uint64        `json:"initial_reward"`
	MinerRewardShare       uint32        `json:"miner_reward_share"`
	ValidatorRewardShare   uint32        `json:"validator_reward_share"`
	EcoFundRewardShare     uint32        `json:"eco_fund_reward_share"`
	UnstakeCooldown        time.Duration `json:"unstake_cooldown"`
	MaxMinersPerIP         uint32        `json:"max_miners_per_ip"`
	NewMinerCooldownEpochs uint64        `json:"new_miner_cooldown_epochs"`
}

// DefaultParams 默认参数
func DefaultParams() Params {
	return Params{
		MinStake:               DefaultMinStake,
		ValidatorMinStake:      DefaultValidatorMinStake,
		EpochBlocks:            DefaultEpochBlocks,
		HalvingEpochs:          DefaultHalvingEpochs,
		InitialReward:          DefaultInitialReward,
		MinerRewardShare:       DefaultMinerRewardShare,
		ValidatorRewardShare:   DefaultValidatorRewardShare,
		EcoFundRewardShare:     DefaultEcoFundRewardShare,
		UnstakeCooldown:        DefaultUnstakeCooldown,
		MaxMinersPerIP:         DefaultMaxMinersPerIP,
		NewMinerCooldownEpochs: DefaultNewMinerCooldownEpochs,
	}
}

// Validate 参数验证
func (p Params) Validate() error {
	if p.MinStake == 0 {
		return ErrInvalidStakeAmount
	}
	if p.EpochBlocks <= 0 {
		return ErrInvalidStakeAmount
	}
	return nil
}

// CalculateEpochReward 计算当前 epoch 奖励（含减半）
func CalculateEpochReward(epochNumber uint64, params Params) uint64 {
	halvings := epochNumber / params.HalvingEpochs
	reward := params.InitialReward
	for i := uint64(0); i < halvings; i++ {
		reward /= 2
		if reward == 0 {
			return 0
		}
	}
	return reward
}
