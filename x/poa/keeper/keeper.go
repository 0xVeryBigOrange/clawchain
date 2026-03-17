package keeper

import (
	"encoding/json"
	"fmt"
	"time"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/clawchain/clawchain/x/poa/types"
)

// Keeper 管理 PoA 模块的链上状态
type Keeper struct {
	cdc      codec.BinaryCodec
	storeKey storetypes.StoreKey
	params   types.Params
}

// NewKeeper 创建新的 Keeper 实例
func NewKeeper(cdc codec.BinaryCodec, storeKey storetypes.StoreKey) Keeper {
	return Keeper{
		cdc:      cdc,
		storeKey: storeKey,
		params:   types.DefaultParams(),
	}
}

// Logger 获取模块日志
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", "x/"+types.ModuleName)
}

// ──────────────────────────────────────────────
// 矿工管理
// ──────────────────────────────────────────────

// RegisterMiner 注册新矿工
func (k Keeper) RegisterMiner(ctx sdk.Context, address string) error {
	store := ctx.KVStore(k.storeKey)
	key := types.GetMinerKey(address)

	// 检查是否已注册
	if store.Has(key) {
		return types.ErrMinerAlreadyRegistered
	}

	miner := types.Miner{
		Address:         address,
		StakeAmount:     0,
		Status:          types.MinerStatusInactive,
		RegisteredAt:    ctx.BlockTime(),
		ReputationScore: 500, // 初始声誉
		TotalRewards:    0,
	}

	bz, err := json.Marshal(miner)
	if err != nil {
		return err
	}
	store.Set(key, bz)

	k.Logger(ctx).Info("矿工注册成功", "address", address)
	return nil
}

// StakeMiner 矿工质押
func (k Keeper) StakeMiner(ctx sdk.Context, address string, amount uint64) error {
	store := ctx.KVStore(k.storeKey)
	key := types.GetMinerKey(address)

	bz := store.Get(key)
	if bz == nil {
		return types.ErrMinerNotFound
	}

	var miner types.Miner
	if err := json.Unmarshal(bz, &miner); err != nil {
		return err
	}

	miner.StakeAmount += amount

	// 质押达到最低要求则激活
	if miner.StakeAmount >= k.params.MinStake && miner.Status == types.MinerStatusInactive {
		miner.Status = types.MinerStatusActive
		k.Logger(ctx).Info("矿工激活", "address", address, "stake", miner.StakeAmount)
	}

	bz, err := json.Marshal(miner)
	if err != nil {
		return err
	}
	store.Set(key, bz)
	return nil
}

// UnstakeMiner 矿工解质押（进入7天冷却队列）
func (k Keeper) UnstakeMiner(ctx sdk.Context, address string, amount uint64) error {
	store := ctx.KVStore(k.storeKey)
	key := types.GetMinerKey(address)

	bz := store.Get(key)
	if bz == nil {
		return types.ErrMinerNotFound
	}

	var miner types.Miner
	if err := json.Unmarshal(bz, &miner); err != nil {
		return err
	}

	if amount > miner.StakeAmount {
		return types.ErrInsufficientStake
	}

	miner.StakeAmount -= amount
	miner.Status = types.MinerStatusUnstaking

	// 如果剩余质押低于最低要求，变为非活跃
	if miner.StakeAmount < k.params.MinStake {
		miner.Status = types.MinerStatusInactive
	}

	bz, err := json.Marshal(miner)
	if err != nil {
		return err
	}
	store.Set(key, bz)

	// 加入解质押队列
	entry := types.UnstakeEntry{
		MinerAddress:   address,
		Amount:         amount,
		CompletionTime: ctx.BlockTime().Add(k.params.UnstakeCooldown),
	}
	entryBz, _ := json.Marshal(entry)
	queueKey := append(types.UnstakeQueueKeyPrefix, []byte(fmt.Sprintf("%d:%s", ctx.BlockTime().Unix(), address))...)
	store.Set(queueKey, entryBz)

	k.Logger(ctx).Info("矿工解质押", "address", address, "amount", amount, "completion", entry.CompletionTime)
	return nil
}

// GetMiner 获取矿工信息
func (k Keeper) GetMiner(ctx sdk.Context, address string) (*types.Miner, error) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.GetMinerKey(address))
	if bz == nil {
		return nil, types.ErrMinerNotFound
	}
	var miner types.Miner
	if err := json.Unmarshal(bz, &miner); err != nil {
		return nil, err
	}
	return &miner, nil
}

// GetActiveMiners 获取所有活跃矿工
func (k Keeper) GetActiveMiners(ctx sdk.Context) []types.Miner {
	store := ctx.KVStore(k.storeKey)
	iter := storetypes.KVStorePrefixIterator(store, types.MinerKeyPrefix)
	defer iter.Close()

	var miners []types.Miner
	for ; iter.Valid(); iter.Next() {
		var miner types.Miner
		if err := json.Unmarshal(iter.Value(), &miner); err != nil {
			continue
		}
		if miner.Status == types.MinerStatusActive {
			miners = append(miners, miner)
		}
	}
	return miners
}

// ──────────────────────────────────────────────
// 奖励分发
// ──────────────────────────────────────────────

// DistributeEpochRewards 分发 epoch 奖励
func (k Keeper) DistributeEpochRewards(ctx sdk.Context, epochNumber uint64, completedMiners []string) {
	reward := types.CalculateEpochReward(epochNumber, k.params)
	if reward == 0 || len(completedMiners) == 0 {
		return
	}

	// 60% 给矿工
	minerPool := reward * uint64(k.params.MinerRewardShare) / 100
	perMiner := minerPool / uint64(len(completedMiners))

	store := ctx.KVStore(k.storeKey)
	for _, addr := range completedMiners {
		key := types.GetMinerKey(addr)
		bz := store.Get(key)
		if bz == nil {
			continue
		}
		var miner types.Miner
		if err := json.Unmarshal(bz, &miner); err != nil {
			continue
		}

		// 新矿工冷启动期奖励减半
		minerEpochs := epochNumber - miner.LastActiveEpoch
		_ = minerEpochs // TODO: 用注册时的 epoch 来判断
		actualReward := perMiner

		miner.TotalRewards += actualReward
		miner.ChallengesCompleted++
		miner.LastActiveEpoch = epochNumber

		bz, _ = json.Marshal(miner)
		store.Set(key, bz)
	}

	k.Logger(ctx).Info("Epoch 奖励分发完成",
		"epoch", epochNumber,
		"reward", reward,
		"miners", len(completedMiners),
		"per_miner", perMiner,
	)
}

// ProcessUnstakeQueue 处理到期的解质押
func (k Keeper) ProcessUnstakeQueue(ctx sdk.Context) {
	store := ctx.KVStore(k.storeKey)
	now := ctx.BlockTime()

	iter := storetypes.KVStorePrefixIterator(store, types.UnstakeQueueKeyPrefix)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var entry types.UnstakeEntry
		if err := json.Unmarshal(iter.Value(), &entry); err != nil {
			continue
		}
		if now.After(entry.CompletionTime) {
			// 解质押到期，返还代币
			k.Logger(ctx).Info("解质押完成", "address", entry.MinerAddress, "amount", entry.Amount)
			// TODO: 实际转账逻辑（通过 bankKeeper）
			store.Delete(iter.Key())
		}
	}
}

// SlashMiner 惩罚矿工（扣质押）
func (k Keeper) SlashMiner(ctx sdk.Context, address string, slashPercent uint32) error {
	store := ctx.KVStore(k.storeKey)
	key := types.GetMinerKey(address)

	bz := store.Get(key)
	if bz == nil {
		return types.ErrMinerNotFound
	}

	var miner types.Miner
	if err := json.Unmarshal(bz, &miner); err != nil {
		return err
	}

	slashAmount := miner.StakeAmount * uint64(slashPercent) / 100
	if slashAmount > miner.StakeAmount {
		slashAmount = miner.StakeAmount
	}
	miner.StakeAmount -= slashAmount

	// 质押不足则暂停
	if miner.StakeAmount < k.params.MinStake {
		miner.Status = types.MinerStatusSuspended
	}

	bz, _ = json.Marshal(miner)
	store.Set(key, bz)

	k.Logger(ctx).Warn("矿工被惩罚",
		"address", address,
		"slash_percent", slashPercent,
		"slash_amount", slashAmount,
		"remaining_stake", miner.StakeAmount,
	)
	return nil
}

// GetCurrentEpoch 获取当前 epoch
func (k Keeper) GetCurrentEpoch(ctx sdk.Context) uint64 {
	return uint64(ctx.BlockHeight()) / uint64(k.params.EpochBlocks)
}

// IsEpochEnd 判断当前区块是否为 epoch 结束
func (k Keeper) IsEpochEnd(ctx sdk.Context) bool {
	return ctx.BlockHeight()%k.params.EpochBlocks == 0 && ctx.BlockHeight() > 0
}

// GetParams 获取参数
func (k Keeper) GetParams() types.Params {
	return k.params
}

// ──────────────────────────────────────────────
// 辅助函数
// ──────────────────────────────────────────────

// Unused but needed by some callers
var _ = time.Now
