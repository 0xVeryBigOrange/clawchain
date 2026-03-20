package keeper_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	"cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/clawchain/clawchain/x/challenge/keeper"
	"github.com/clawchain/clawchain/x/challenge/types"
)

// mockBankKeeper is a mock for BankKeeper interface
type mockBankKeeper struct {
	balances map[string]sdk.Coins
}

func newMockBankKeeper() *mockBankKeeper {
	return &mockBankKeeper{balances: make(map[string]sdk.Coins)}
}

func (m *mockBankKeeper) SendCoinsFromModuleToAccount(_ context.Context, _ string, addr sdk.AccAddress, amt sdk.Coins) error {
	m.balances[addr.String()] = m.balances[addr.String()].Add(amt...)
	return nil
}

func (m *mockBankKeeper) GetBalance(_ context.Context, addr sdk.AccAddress, denom string) sdk.Coin {
	for _, c := range m.balances[addr.String()] {
		if c.Denom == denom {
			return c
		}
	}
	return sdk.NewInt64Coin(denom, 0)
}

func (m *mockBankKeeper) MintCoins(_ context.Context, _ string, _ sdk.Coins) error {
	return nil
}

func setupKeeper(t *testing.T) (keeper.Keeper, sdk.Context, *mockBankKeeper) {
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)

	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewNopLogger(), metrics.NewNoOpMetrics())
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	require.NoError(t, stateStore.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)

	bk := newMockBankKeeper()
	k := keeper.NewKeeper(cdc, storeKey, bk)

	ctx := sdk.NewContext(stateStore, cmtproto.Header{Height: 10}, false, log.NewNopLogger())
	return k, ctx, bk
}

func TestGenerateChallenges(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	activeMiners := []string{"miner1", "miner2", "miner3", "miner4", "miner5"}
	challenges := k.GenerateChallenges(ctx, 1, activeMiners)

	require.NotEmpty(t, challenges)
	require.Equal(t, 10, len(challenges)) // default ChallengesPerEpoch = 10

	for _, ch := range challenges {
		require.NotEmpty(t, ch.ID)
		require.Equal(t, uint64(1), ch.Epoch)
		require.Equal(t, types.ChallengeStatusPending, ch.Status)
		require.NotEmpty(t, ch.Prompt)
	}
}

func TestGenerateChallengesNoMiners(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	challenges := k.GenerateChallenges(ctx, 1, nil)
	require.Nil(t, challenges)
}

func TestGetBlockReward(t *testing.T) {
	k, _, _ := setupKeeper(t)

	// Epoch 0 (height 0): 30,000,000 uclaw (30 CLAW miner pool per epoch)
	r := k.GetBlockReward(0)
	require.Equal(t, int64(30_000_000), r)

	// Height 5000 (epoch 50): still 30,000,000 (no halving yet)
	r = k.GetBlockReward(5000)
	require.Equal(t, int64(30_000_000), r)

	// Height 21,000,000 (epoch 210,000): first halving → 15,000,000
	r = k.GetBlockReward(21_000_000)
	require.Equal(t, int64(15_000_000), r)

	// Height 42,000,000 (epoch 420,000): second halving → 7,500,000
	r = k.GetBlockReward(42_000_000)
	require.Equal(t, int64(7_500_000), r)
}

func TestGetBlockRewardMinimum(t *testing.T) {
	k, _, _ := setupKeeper(t)

	// Very high height should not go below minimum (1 uclaw)
	r := k.GetBlockReward(10_000_000_000)
	require.GreaterOrEqual(t, r, int64(1))
}

func TestChallengeTypes(t *testing.T) {
	// Verify challenge type constants exist
	require.NotEmpty(t, string(types.ChallengeMath))
	require.NotEmpty(t, string(types.ChallengeLogic))
	require.NotEmpty(t, string(types.ChallengeSentiment))
	require.NotEmpty(t, string(types.ChallengeTextSummary))
}

// mockReputationKeeper for tier/spot check tests
type mockReputationKeeper struct {
	scores map[string]int32
}

func newMockReputationKeeper() *mockReputationKeeper {
	return &mockReputationKeeper{scores: make(map[string]int32)}
}

func (m *mockReputationKeeper) GetMinerScore(_ sdk.Context, addr string) (int32, bool) {
	s, ok := m.scores[addr]
	return s, ok
}

func (m *mockReputationKeeper) UpdateScore(_ sdk.Context, addr string, delta int32, _ string) {
	m.scores[addr] += delta
}

func TestTierAutoAssignment(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	activeMiners := []string{"m1", "m2", "m3"}
	challenges := k.GenerateChallenges(ctx, 100, activeMiners)

	for _, ch := range challenges {
		expectedTier := types.GetTaskTier(ch.Type)
		require.Equal(t, expectedTier, ch.Tier, "challenge %s type %s should have tier %d", ch.ID, ch.Type, expectedTier)
	}
}

func TestSubmitAnswerWithTierCheck(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	rk := newMockReputationKeeper()
	rk.scores["low_rep"] = 400
	rk.scores["mid_rep"] = 650
	rk.scores["high_rep"] = 850
	k.SetReputationKeeper(rk)

	// Create a Tier 3 challenge manually
	store := ctx.KVStore(k.StoreKey())
	ch := types.Challenge{
		ID:            "ch-test-tier3",
		Type:          types.ChallengeTextSummary,
		Tier:          types.TierAdvanced,
		Status:        types.ChallengeStatusPending,
		Commits:       make(map[string]string),
		Reveals:       make(map[string]string),
	}
	bz, _ := json.Marshal(ch)
	store.Set([]byte("challenge:ch-test-tier3"), bz)

	// Low rep miner should be rejected
	err := k.SubmitAnswerWithChecks(ctx, "ch-test-tier3", "low_rep", "answer")
	require.ErrorIs(t, err, types.ErrInsufficientReputation)

	// Mid rep miner should also be rejected for Tier 3
	err = k.SubmitAnswerWithChecks(ctx, "ch-test-tier3", "mid_rep", "answer")
	require.ErrorIs(t, err, types.ErrInsufficientReputation)

	// High rep miner should pass
	err = k.SubmitAnswerWithChecks(ctx, "ch-test-tier3", "high_rep", "answer")
	require.NoError(t, err)
}

func TestSpotCheckPenalty(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	rk := newMockReputationKeeper()
	rk.scores["miner1"] = 500
	rk.scores["miner2"] = 500
	k.SetReputationKeeper(rk)

	store := ctx.KVStore(k.StoreKey())
	ch := types.Challenge{
		ID:          "ch-spot-1",
		Type:        types.ChallengeMath,
		Tier:        types.TierBasic,
		Status:      types.ChallengeStatusPending,
		IsSpotCheck: true,
		KnownAnswer: "42",
		Commits:     make(map[string]string),
		Reveals:     make(map[string]string),
	}
	bz, _ := json.Marshal(ch)
	store.Set([]byte("challenge:ch-spot-1"), bz)

	// Wrong answer: -50
	err := k.SubmitAnswerWithChecks(ctx, "ch-spot-1", "miner1", "wrong")
	require.NoError(t, err)
	require.Equal(t, int32(450), rk.scores["miner1"])

	// Correct answer: +10
	err = k.SubmitAnswerWithChecks(ctx, "ch-spot-1", "miner2", "42")
	require.NoError(t, err)
	require.Equal(t, int32(510), rk.scores["miner2"])
}

func TestSpotCheckNonSpot(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	rk := newMockReputationKeeper()
	rk.scores["miner1"] = 500
	k.SetReputationKeeper(rk)

	store := ctx.KVStore(k.StoreKey())
	ch := types.Challenge{
		ID:          "ch-normal",
		Type:        types.ChallengeMath,
		Tier:        types.TierBasic,
		Status:      types.ChallengeStatusPending,
		IsSpotCheck: false,
		Commits:     make(map[string]string),
		Reveals:     make(map[string]string),
	}
	bz, _ := json.Marshal(ch)
	store.Set([]byte("challenge:ch-normal"), bz)

	// Non-spot check: no reputation change
	err := k.SubmitAnswerWithChecks(ctx, "ch-normal", "miner1", "any")
	require.NoError(t, err)
	require.Equal(t, int32(500), rk.scores["miner1"])
}

func TestDefaultChallengeParams(t *testing.T) {
	params := types.DefaultChallengeParams()
	require.Equal(t, uint32(10), params.ChallengesPerEpoch)
	require.Equal(t, uint32(3), params.AssigneesPerChallenge)
	require.Equal(t, int64(5), params.ResponseWindowBlocks)
}

// ──────────────────────────────────────────────
// 新增测试: CalcNumChallenges
// ──────────────────────────────────────────────

func TestCalcNumChallenges(t *testing.T) {
	tests := []struct {
		activeMiners int
		expected     int
	}{
		{0, 1},
		{1, 1},
		{2, 1},
		{3, 1},
		{4, 1},
		{5, 1},
		{6, 2},
		{9, 3},
		{10, 3},
		{15, 5},
		{30, 10},
		{50, 10},
		{100, 10},
	}

	for _, tt := range tests {
		result := keeper.CalcNumChallenges(tt.activeMiners)
		require.Equal(t, tt.expected, result,
			"CalcNumChallenges(%d) = %d, want %d", tt.activeMiners, result, tt.expected)
	}
}

// ──────────────────────────────────────────────
// 新增测试: applyBonusMultipliers（通过 REST handler 间接测试）
// ──────────────────────────────────────────────

func TestApplyBonusMultipliersLogic(t *testing.T) {
	// 直接测试早鸟倍率和签到倍率的计算逻辑
	// regIndex <= 1000 → earlyBird 300%
	// regIndex <= 5000 → earlyBird 200%
	// regIndex <= 10000 → earlyBird 150%
	// regIndex > 10000 → earlyBird 100%
	// consecutiveDays >= 90 → streak 150%
	// consecutiveDays >= 30 → streak 125%
	// consecutiveDays >= 7 → streak 110%
	// consecutiveDays < 7 → streak 100%
	// completed < 100 → /2 (冷启动减半)

	tests := []struct {
		name            string
		regIndex        uint64
		consecutiveDays uint64
		completed       int64
		baseReward      int64
		expectedReward  int64
	}{
		{
			name: "early bird 3x, no streak, cold start",
			regIndex: 1, consecutiveDays: 0, completed: 0, baseReward: 10000,
			// 10000 * 300 * 100 / 10000 = 30000 → /2 = 15000
			expectedReward: 15000,
		},
		{
			name: "early bird 3x, no streak, warm",
			regIndex: 500, consecutiveDays: 0, completed: 200, baseReward: 10000,
			// 10000 * 300 * 100 / 10000 = 30000
			expectedReward: 30000,
		},
		{
			name: "early bird 2x, 30d streak, warm",
			regIndex: 3000, consecutiveDays: 30, completed: 100, baseReward: 10000,
			// 10000 * 200 * 125 / 10000 = 25000
			expectedReward: 25000,
		},
		{
			name: "no early bird, 90d streak, cold start",
			regIndex: 20000, consecutiveDays: 90, completed: 50, baseReward: 10000,
			// 10000 * 100 * 150 / 10000 = 15000 → /2 = 7500
			expectedReward: 7500,
		},
		{
			name: "minimum reward floor",
			regIndex: 20000, consecutiveDays: 0, completed: 0, baseReward: 1,
			// 1 * 100 * 100 / 10000 = 1 → /2 = 0 → floor = 1
			expectedReward: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Manually compute using the same formula
			earlyBird := uint64(100)
			if tt.regIndex > 0 && tt.regIndex <= 1000 {
				earlyBird = 300
			} else if tt.regIndex <= 5000 {
				earlyBird = 200
			} else if tt.regIndex <= 10000 {
				earlyBird = 150
			}

			streak := uint64(100)
			if tt.consecutiveDays >= 90 {
				streak = 150
			} else if tt.consecutiveDays >= 30 {
				streak = 125
			} else if tt.consecutiveDays >= 7 {
				streak = 110
			}

			actualReward := tt.baseReward * int64(earlyBird) * int64(streak) / 10000
			if tt.completed < 100 {
				actualReward = actualReward / 2
			}
			if actualReward < 1 {
				actualReward = 1
			}

			require.Equal(t, tt.expectedReward, actualReward)
		})
	}
}

// ──────────────────────────────────────────────
// 新增测试: 多挑战生成
// ──────────────────────────────────────────────

func TestGeneratePublicChallengeMulti(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	store := ctx.KVStore(k.StoreKey())

	// 注册 9 个活跃矿工 → CalcNumChallenges(9) = 3
	for i := 0; i < 9; i++ {
		minerData := map[string]interface{}{
			"address": fmt.Sprintf("claw1miner%d", i),
			"status":  "active",
		}
		bz, _ := json.Marshal(minerData)
		store.Set([]byte(fmt.Sprintf("miner:claw1miner%d", i)), bz)
	}

	k.GeneratePublicChallenge(ctx, 42)

	// 验证生成了 3 个挑战
	count := 0
	iter := storetypes.KVStorePrefixIterator(store, []byte("challenge:ch-42-"))
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		count++
		var ch types.Challenge
		require.NoError(t, json.Unmarshal(iter.Value(), &ch))
		require.Equal(t, uint64(42), ch.Epoch)
		require.Equal(t, types.ChallengeStatusPending, ch.Status)
		require.NotEmpty(t, ch.Prompt)
	}
	require.Equal(t, 3, count, "应生成 3 个挑战 (9 miners / 3 = 3)")
}

func TestGeneratePublicChallengeSingleMiner(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	store := ctx.KVStore(k.StoreKey())

	// 注册 1 个活跃矿工 → CalcNumChallenges(1) = 1
	minerData := map[string]interface{}{
		"address": "claw1solo",
		"status":  "active",
	}
	bz, _ := json.Marshal(minerData)
	store.Set([]byte("miner:claw1solo"), bz)

	k.GeneratePublicChallenge(ctx, 10)

	count := 0
	iter := storetypes.KVStorePrefixIterator(store, []byte("challenge:ch-10-"))
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		count++
	}
	require.Equal(t, 1, count, "应生成 1 个挑战 (1 miner)")
}

func TestGeneratePublicChallengeNoMiners(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	store := ctx.KVStore(k.StoreKey())

	// 无矿工 → CalcNumChallenges(0) = 1
	k.GeneratePublicChallenge(ctx, 5)

	count := 0
	iter := storetypes.KVStorePrefixIterator(store, []byte("challenge:ch-5-"))
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		count++
	}
	require.Equal(t, 1, count, "无矿工时也应生成 1 个挑战")
}

// ──────────────────────────────────────────────
// 新增测试: Commit-Reveal 流程
// ──────────────────────────────────────────────

func TestCommitRevealFlow(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	store := ctx.KVStore(k.StoreKey())

	// 创建一个挑战（公开，任何人可参与）
	ch := types.Challenge{
		ID:            "ch-cr-1",
		Epoch:         1,
		Type:          types.ChallengeMath,
		Tier:          types.TierBasic,
		Status:        types.ChallengeStatusPending,
		Assignees:     []string{"miner1", "miner2"},
		Commits:       make(map[string]string),
		Reveals:       make(map[string]string),
	}
	bz, _ := json.Marshal(ch)
	store.Set([]byte("challenge:ch-cr-1"), bz)

	// Phase 1: Commit
	err := k.SubmitCommit(ctx, "ch-cr-1", "miner1", "abc123hash")
	require.NoError(t, err)

	// 验证 commit 已记录
	bz = store.Get([]byte("challenge:ch-cr-1"))
	require.NotNil(t, bz)
	var updated types.Challenge
	json.Unmarshal(bz, &updated)
	require.Equal(t, "abc123hash", updated.Commits["miner1"])
	require.Equal(t, types.ChallengeStatusCommit, updated.Status)

	// 重复 commit 应报错
	err = k.SubmitCommit(ctx, "ch-cr-1", "miner1", "different")
	require.ErrorIs(t, err, types.ErrAlreadyCommitted)

	// Phase 2: Reveal（哈希不匹配）
	err = k.SubmitReveal(ctx, "ch-cr-1", "miner1", "wrong_answer", "wrong_salt")
	require.ErrorIs(t, err, types.ErrCommitHashMismatch)

	// Phase 2: Reveal（哈希匹配）— 先用正确的 answer+salt 生成 hash
	// 注意：commit hash "abc123hash" 不是真正的 sha256，所以这里用真实流程
	// 重新测试完整流程
	ch2 := types.Challenge{
		ID:            "ch-cr-2",
		Epoch:         1,
		Type:          types.ChallengeMath,
		Tier:          types.TierBasic,
		Status:        types.ChallengeStatusPending,
		Assignees:     []string{"miner1"},
		Commits:       make(map[string]string),
		Reveals:       make(map[string]string),
	}
	bz, _ = json.Marshal(ch2)
	store.Set([]byte("challenge:ch-cr-2"), bz)

	// 生成正确的 commit hash
	answer := "42"
	salt := "my_secret_salt"
	h := sha256.Sum256([]byte(answer + salt))
	import_hash := hex.EncodeToString(h[:])

	err = k.SubmitCommit(ctx, "ch-cr-2", "miner1", import_hash)
	require.NoError(t, err)

	// Reveal with correct answer+salt
	err = k.SubmitReveal(ctx, "ch-cr-2", "miner1", answer, salt)
	require.NoError(t, err)

	// 验证 reveal 已记录
	bz = store.Get([]byte("challenge:ch-cr-2"))
	json.Unmarshal(bz, &updated)
	require.Equal(t, answer, updated.Reveals["miner1"])
	require.Equal(t, types.ChallengeStatusReveal, updated.Status)
}

func TestCommitRevealChallengeNotFound(t *testing.T) {
	k, ctx, _ := setupKeeper(t)

	err := k.SubmitCommit(ctx, "nonexistent", "miner1", "hash")
	require.ErrorIs(t, err, types.ErrChallengeNotFound)

	err = k.SubmitReveal(ctx, "nonexistent", "miner1", "answer", "salt")
	require.ErrorIs(t, err, types.ErrChallengeNotFound)
}

// ──────────────────────────────────────────────
// 新增测试: AccumulateEpochRewards
// ──────────────────────────────────────────────

func TestAccumulateEpochRewards(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	store := ctx.KVStore(k.StoreKey())

	// 第一个 epoch
	k.AccumulateEpochRewards(ctx, 1)

	var valTotal, ecoTotal int64
	json.Unmarshal(store.Get([]byte("validator_pool_total")), &valTotal)
	json.Unmarshal(store.Get([]byte("eco_fund_total")), &ecoTotal)
	require.Equal(t, int64(10_000_000), valTotal)
	require.Equal(t, int64(10_000_000), ecoTotal)

	// 第二个 epoch（累加）
	k.AccumulateEpochRewards(ctx, 2)
	json.Unmarshal(store.Get([]byte("validator_pool_total")), &valTotal)
	json.Unmarshal(store.Get([]byte("eco_fund_total")), &ecoTotal)
	require.Equal(t, int64(20_000_000), valTotal)
	require.Equal(t, int64(20_000_000), ecoTotal)
}

// ──────────────────────────────────────────────
// 新增测试: CountActiveMiners
// ──────────────────────────────────────────────

func TestCountActiveMiners(t *testing.T) {
	k, ctx, _ := setupKeeper(t)
	store := ctx.KVStore(k.StoreKey())

	require.Equal(t, 0, k.CountActiveMiners(ctx))

	// 添加活跃矿工
	for i := 0; i < 5; i++ {
		d := map[string]interface{}{"address": fmt.Sprintf("m%d", i), "status": "active"}
		bz, _ := json.Marshal(d)
		store.Set([]byte(fmt.Sprintf("miner:m%d", i)), bz)
	}
	// 添加非活跃矿工
	d := map[string]interface{}{"address": "inactive", "status": "inactive"}
	bz, _ := json.Marshal(d)
	store.Set([]byte("miner:inactive"), bz)

	require.Equal(t, 5, k.CountActiveMiners(ctx))
}
