package keeper

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	storetypes "cosmossdk.io/store/types"
	"github.com/gorilla/mux"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"

	challengetypes "github.com/clawchain/clawchain/x/challenge/types"
)

// RESTHandler REST API 处理器
type RESTHandler struct {
	keeper     *Keeper
	bankKeeper *bankkeeper.BaseKeeper
	storeGetter func() storetypes.CommitMultiStore
}

// NewRESTHandler 创建 REST handler
func NewRESTHandler(k *Keeper, bk *bankkeeper.BaseKeeper, storeGetter func() storetypes.CommitMultiStore) *RESTHandler {
	return &RESTHandler{
		keeper:      k,
		bankKeeper:  bk,
		storeGetter: storeGetter,
	}
}

// RegisterRoutes 注册路由（在 app.go 调用）
func (h *RESTHandler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/clawchain/challenges/pending", h.GetPendingChallenges).Methods("GET")
	router.HandleFunc("/clawchain/challenge/submit", h.SubmitAnswer).Methods("POST")
	router.HandleFunc("/clawchain/challenge/commit", h.SubmitCommitREST).Methods("POST")
	router.HandleFunc("/clawchain/challenge/reveal", h.SubmitRevealREST).Methods("POST")
	router.HandleFunc("/clawchain/miner/register", h.RegisterMiner).Methods("POST")
	router.HandleFunc("/clawchain/miner/{address}", h.GetMinerInfo).Methods("GET")
	router.HandleFunc("/clawchain/miner/{address}/stats", h.GetMinerStats).Methods("GET")
	router.HandleFunc("/clawchain/stats", h.GetChainStats).Methods("GET")
	router.HandleFunc("/clawchain/faucet", h.Faucet).Methods("POST")
}

// getStore 获取 KV store
func (h *RESTHandler) getStore() storetypes.KVStore {
	cms := h.storeGetter()
	return cms.GetKVStore(h.keeper.storeKey)
}

// GetPendingChallenges GET /clawchain/challenges/pending
func (h *RESTHandler) GetPendingChallenges(w http.ResponseWriter, r *http.Request) {
	store := h.getStore()

	var challenges []challengetypes.Challenge
	iter := storetypes.KVStorePrefixIterator(store, []byte("challenge:"))
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var ch challengetypes.Challenge
		if err := json.Unmarshal(iter.Value(), &ch); err != nil {
			continue
		}
		// 只返回待处理的挑战
		if ch.Status == challengetypes.ChallengeStatusPending || ch.Status == challengetypes.ChallengeStatusCommit {
			challenges = append(challenges, ch)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"challenges": challenges,
	})
}

// SubmitAnswerRequest 提交答案请求
type SubmitAnswerRequest struct {
	ChallengeID string `json:"challenge_id"`
	MinerAddr   string `json:"miner_address"`
	Answer      string `json:"answer"`
}

// SubmitAnswer POST /clawchain/challenge/submit
func (h *RESTHandler) SubmitAnswer(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var req SubmitAnswerRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	store := h.getStore()
	
	// 检查矿工是否已注册
	minerKey := []byte(fmt.Sprintf("miner:%s", req.MinerAddr))
	minerBz := store.Get(minerKey)
	if minerBz == nil {
		http.Error(w, "miner not registered", http.StatusForbidden)
		return
	}
	
	// 检查矿工状态
	var minerData map[string]interface{}
	json.Unmarshal(minerBz, &minerData)
	if status, ok := minerData["status"].(string); ok && status != "active" {
		http.Error(w, "miner not active", http.StatusForbidden)
		return
	}

	key := []byte(fmt.Sprintf("challenge:%s", req.ChallengeID))
	bz := store.Get(key)
	if bz == nil {
		http.Error(w, "challenge not found", http.StatusNotFound)
		return
	}

	var ch challengetypes.Challenge
	json.Unmarshal(bz, &ch)
	
	// 检查挑战是否过期（创建后 200 blocks 内有效 ≈ 2 epoch @6s = 20 分钟）
	cms := h.storeGetter()
	currentHeight := cms.LatestVersion()
	if os.Getenv("CLAWCHAIN_DEV") != "1" {
		// 生产模式才检查过期（测试模式跳过过期检查）
		if currentHeight-ch.CreatedHeight > 200 {
			ch.Status = challengetypes.ChallengeStatusExpired
			bz, _ := json.Marshal(ch)
			store.Set(key, bz)
			http.Error(w, "challenge expired", http.StatusGone)
			return
		}
	}

	// 验证矿工是否被分配（公开挑战 Assignees 为空，任何人可参与）
	if len(ch.Assignees) > 0 {
		assigned := false
		for _, a := range ch.Assignees {
			if a == req.MinerAddr {
				assigned = true
				break
			}
		}
		if !assigned {
			http.Error(w, "not assigned to this challenge", http.StatusForbidden)
			return
		}
	}
	
	// 防重复提交：检查该矿工是否已提交答案
	if ch.Reveals == nil {
		ch.Reveals = make(map[string]string)
	}
	if _, exists := ch.Reveals[req.MinerAddr]; exists {
		http.Error(w, "already submitted", http.StatusConflict)
		return
	}

	// 记录答案（不立即判断对错）
	ch.Reveals[req.MinerAddr] = req.Answer
	ch.Status = challengetypes.ChallengeStatusReveal

	submissionCount := len(ch.Reveals)
	requiredSubmissions := 3 // 需要 3 个矿工提交

	// Dev mode: 环境变量 CLAWCHAIN_DEV=1 时允许单矿工结算
	if os.Getenv("CLAWCHAIN_DEV") == "1" {
		requiredSubmissions = 1
	}

	// 检查是否达到 3 个提交，触发验证
	if submissionCount >= requiredSubmissions {
		// 执行多数一致验证
		answerVotes := make(map[string][]string) // answer -> []minerAddr
		for minerAddr, answer := range ch.Reveals {
			normalizedAnswer := strings.TrimSpace(strings.ToLower(answer))
			answerVotes[normalizedAnswer] = append(answerVotes[normalizedAnswer], minerAddr)
		}

		// 找到多数答案（至少 2/3）
		var majorityAnswer string
		var majorityMiners []string
		maxVotes := 0
		for answer, miners := range answerVotes {
			if len(miners) > maxVotes {
				maxVotes = len(miners)
				majorityAnswer = answer
				majorityMiners = miners
			}
		}

		// 判断是否达到多数（至少 2 票，dev mode 下 1 票即可）
		minMajority := 2
		if os.Getenv("CLAWCHAIN_DEV") == "1" {
			minMajority = 1
		}
		if maxVotes >= minMajority {
			ch.Status = challengetypes.ChallengeStatusComplete

			// P0-1 修复：获取当前 epoch 的矿工池总奖励，然后按挑战数和答对人数分
			epochMinerPool := h.keeper.GetBlockReward(currentHeight)

			// 计算本 epoch 生成的挑战数量（用于分摊奖励）
			numChallengesInEpoch := h.getEpochChallengeCount(store, ch.Epoch)
			if numChallengesInEpoch < 1 {
				numChallengesInEpoch = 1
			}

			// 每道题的奖励池 = 矿工池 / 挑战数
			perChallengePool := epochMinerPool / int64(numChallengesInEpoch)
			if perChallengePool < 1 {
				perChallengePool = 1
			}

			// 每个答对矿工的奖励 = 每题奖励池 / 答对人数
			rewardPerMiner := perChallengePool / int64(len(majorityMiners))
			if rewardPerMiner < 1 {
				rewardPerMiner = 1
			}

			// P1-3 修复：应用早鸟倍率和签到倍率
			for _, minerAddr := range majorityMiners {
				actualReward := h.applyBonusMultipliers(store, minerAddr, rewardPerMiner)

				// 写入 pending_reward 供 EndBlock 的 ProcessPendingRewards 处理
				pendingKey := []byte(fmt.Sprintf("pending_reward:%d:%s:%s", currentHeight, req.ChallengeID, minerAddr))
				pendingReward := map[string]interface{}{
					"challenge_id": req.ChallengeID,
					"miner_addr":   minerAddr,
					"amount":       actualReward,
					"height":       currentHeight,
				}
				pendingBz, _ := json.Marshal(pendingReward)
				store.Set(pendingKey, pendingBz)

				// 更新矿工统计（注意: ProcessPendingRewards 也会更新，这里只更新 challenges_completed）
				mKey := []byte(fmt.Sprintf("miner:%s", minerAddr))
				mBz := store.Get(mKey)
				if mBz != nil {
					var mData map[string]interface{}
					if json.Unmarshal(mBz, &mData) == nil {
						completed := int64(0)
						if v, ok := mData["challenges_completed"].(float64); ok {
							completed = int64(v)
						}
						mData["challenges_completed"] = completed + 1
						mBz, _ = json.Marshal(mData)
						store.Set(mKey, mBz)
					}
				}
			}

			// 惩罚不一致的矿工（扣声誉分）
			for minerAddr, answer := range ch.Reveals {
				normalizedAnswer := strings.TrimSpace(strings.ToLower(answer))
				if normalizedAnswer != majorityAnswer {
					// 更新矿工失败记录
					minerKey := []byte(fmt.Sprintf("miner:%s", minerAddr))
					minerBz := store.Get(minerKey)
					if minerBz != nil {
						var minerData map[string]interface{}
						json.Unmarshal(minerBz, &minerData)
						
						failed := int64(0)
						if v, ok := minerData["challenges_failed"].(float64); ok {
							failed = int64(v)
						}
						minerData["challenges_failed"] = failed + 1
						
						minerBz, _ = json.Marshal(minerData)
						store.Set(minerKey, minerBz)
					}
				}
			}
		}
	}

	bz, _ = json.Marshal(ch)
	store.Set(key, bz)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":            true,
		"submission_count":   submissionCount,
		"required_submissions": requiredSubmissions,
		"status":             ch.Status,
		"message":            "answer recorded, waiting for other miners to submit",
	})
}

// RegisterMinerRequest 注册矿工请求
type RegisterMinerRequest struct {
	Address string `json:"address"`
	Name    string `json:"name"`
}

// RegisterMiner POST /clawchain/miner/register
func (h *RESTHandler) RegisterMiner(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var req RegisterMinerRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// 验证地址格式
	accAddr, err := sdk.AccAddressFromBech32(req.Address)
	if err != nil {
		http.Error(w, "invalid address format", http.StatusBadRequest)
		return
	}

	store := h.getStore()

	// 检查是否已注册
	minerKey := []byte(fmt.Sprintf("miner:%s", req.Address))
	if store.Has(minerKey) {
		http.Error(w, "miner already registered", http.StatusConflict)
		return
	}

	// P0-3 修复：检查余额 >= 100 CLAW (100,000,000 uclaw)，DEV 模式跳过
	const minStakeUclaw = int64(100_000_000) // 100 CLAW
	if os.Getenv("CLAWCHAIN_DEV") != "1" {
		cms := h.storeGetter()
		sdkCtx := sdk.Context{}.WithMultiStore(cms).WithBlockHeight(cms.LatestVersion())
		balance := h.bankKeeper.GetBalance(sdkCtx, accAddr, "uclaw")
		if balance.Amount.Int64() < minStakeUclaw {
			http.Error(w, fmt.Sprintf("insufficient balance: need at least 100 CLAW (100000000 uclaw), have %s uclaw. Please claim from faucet first: POST /clawchain/faucet", balance.Amount.String()), http.StatusForbidden)
			return
		}
	}

	// 获取全局注册序号（用于早鸟倍率）
	regCountKey := []byte("global:miner_count")
	regCount := int64(0)
	if bz := store.Get(regCountKey); bz != nil {
		json.Unmarshal(bz, &regCount)
	}
	regCount++
	regCountBz, _ := json.Marshal(regCount)
	store.Set(regCountKey, regCountBz)

	// 存储矿工信息（包含早鸟/签到字段）
	minerData := map[string]interface{}{
		"address":              req.Address,
		"name":                 req.Name,
		"status":               "active",
		"registered_height":    h.storeGetter().LatestVersion(),
		"challenges_completed": 0,
		"total_rewards":        0,
		"challenges_failed":    0,
		"registration_index":   regCount,
		"consecutive_days":     0,
		"last_checkin_epoch":   0,
	}
	bz, _ := json.Marshal(minerData)
	store.Set(minerKey, bz)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":            true,
		"message":            "miner registered successfully",
		"address":            req.Address,
		"registration_index": regCount,
	})
}

// GetMinerInfo GET /clawchain/miner/{address}
func (h *RESTHandler) GetMinerInfo(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	address := vars["address"]
	
	if address == "" {
		http.Error(w, "address required", http.StatusBadRequest)
		return
	}

	store := h.getStore()
	
	minerKey := []byte(fmt.Sprintf("miner:%s", address))
	bz := store.Get(minerKey)
	if bz == nil {
		http.Error(w, "miner not found", http.StatusNotFound)
		return
	}

	var minerData map[string]interface{}
	json.Unmarshal(bz, &minerData)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(minerData)
}

// GetMinerStats GET /clawchain/miner/{address}/stats
func (h *RESTHandler) GetMinerStats(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	address := vars["address"]
	
	if address == "" {
		http.Error(w, "address required", http.StatusBadRequest)
		return
	}

	store := h.getStore()
	
	minerKey := []byte(fmt.Sprintf("miner:%s", address))
	bz := store.Get(minerKey)
	if bz == nil {
		http.Error(w, "miner not found", http.StatusNotFound)
		return
	}

	var minerData map[string]interface{}
	json.Unmarshal(bz, &minerData)
	
	completed := int64(0)
	failed := int64(0)
	totalRewards := int64(0)
	
	if v, ok := minerData["challenges_completed"].(float64); ok {
		completed = int64(v)
	}
	if v, ok := minerData["challenges_failed"].(float64); ok {
		failed = int64(v)
	}
	if v, ok := minerData["total_rewards"].(float64); ok {
		totalRewards = int64(v)
	}
	
	total := completed + failed
	successRate := float64(0)
	if total > 0 {
		successRate = float64(completed) / float64(total) * 100
	}

	stats := map[string]interface{}{
		"address":             address,
		"challenges_completed": completed,
		"challenges_failed":    failed,
		"total_challenges":     total,
		"success_rate":         fmt.Sprintf("%.2f%%", successRate),
		"total_rewards":        totalRewards,
		"total_rewards_uclaw":  fmt.Sprintf("%d uclaw", totalRewards),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// GetChainStats GET /clawchain/stats
func (h *RESTHandler) GetChainStats(w http.ResponseWriter, r *http.Request) {
	store := h.getStore()
	
	// 统计挑战总数
	totalChallenges := 0
	completedChallenges := 0
	iter := storetypes.KVStorePrefixIterator(store, []byte("challenge:"))
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		totalChallenges++
		var ch challengetypes.Challenge
		if err := json.Unmarshal(iter.Value(), &ch); err == nil {
			if ch.Status == challengetypes.ChallengeStatusComplete || ch.Status == challengetypes.ChallengeStatusReveal {
				completedChallenges++
			}
		}
	}
	
	// 统计活跃矿工数和总奖励
	activeMiners := 0
	totalRewardsPaid := int64(0)
	minerIter := storetypes.KVStorePrefixIterator(store, []byte("miner:"))
	defer minerIter.Close()
	for ; minerIter.Valid(); minerIter.Next() {
		var minerData map[string]interface{}
		if err := json.Unmarshal(minerIter.Value(), &minerData); err == nil {
			if status, ok := minerData["status"].(string); ok && status == "active" {
				activeMiners++
			}
			if rewards, ok := minerData["total_rewards"].(float64); ok {
				totalRewardsPaid += int64(rewards)
			}
		}
	}
	
	currentHeight := h.storeGetter().LatestVersion()
	currentReward := h.keeper.GetBlockReward(currentHeight)

	// 读取验证者池和生态基金累计值
	validatorPoolTotal := int64(0)
	if bz := store.Get([]byte("validator_pool_total")); bz != nil {
		json.Unmarshal(bz, &validatorPoolTotal)
	}
	ecoFundTotal := int64(0)
	if bz := store.Get([]byte("eco_fund_total")); bz != nil {
		json.Unmarshal(bz, &ecoFundTotal)
	}

	stats := map[string]interface{}{
		"total_challenges":       totalChallenges,
		"completed_challenges":   completedChallenges,
		"active_miners":          activeMiners,
		"total_rewards_paid":     totalRewardsPaid,
		"total_rewards_uclaw":    fmt.Sprintf("%d uclaw", totalRewardsPaid),
		"current_block_height":   currentHeight,
		"current_block_reward":   currentReward,
		"current_reward_uclaw":   fmt.Sprintf("%d uclaw", currentReward),
		"validator_pool_total":   validatorPoolTotal,
		"validator_pool_uclaw":   fmt.Sprintf("%d uclaw", validatorPoolTotal),
		"eco_fund_total":         ecoFundTotal,
		"eco_fund_uclaw":         fmt.Sprintf("%d uclaw", ecoFundTotal),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// Faucet POST /clawchain/faucet — 测试网水龙头，给新矿工发 200 CLAW
func (h *RESTHandler) Faucet(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var req struct {
		Address string `json:"address"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if _, err := sdk.AccAddressFromBech32(req.Address); err != nil {
		http.Error(w, "invalid address format", http.StatusBadRequest)
		return
	}

	store := h.getStore()

	// 检查是否已领取过
	faucetKey := []byte(fmt.Sprintf("faucet:%s", req.Address))
	if store.Has(faucetKey) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "已领取过水龙头代币",
		})
		return
	}

	// 标记已领取
	store.Set(faucetKey, []byte("claimed"))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("已向 %s 发放 200 CLAW (200,000,000 uclaw)", req.Address),
		"amount":  200_000_000,
	})
}

// ──────────────────────────────────────────────
// Commit-Reveal 两阶段提交 REST 接口
// ──────────────────────────────────────────────

// CommitRequest commit 阶段请求
type CommitRequest struct {
	ChallengeID string `json:"challenge_id"`
	MinerAddr   string `json:"miner_address"`
	CommitHash  string `json:"commit_hash"` // sha256(answer+nonce)
}

// SubmitCommitREST POST /clawchain/challenge/commit
func (h *RESTHandler) SubmitCommitREST(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var req CommitRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if req.ChallengeID == "" || req.MinerAddr == "" || req.CommitHash == "" {
		http.Error(w, "challenge_id, miner_address, and commit_hash are required", http.StatusBadRequest)
		return
	}

	store := h.getStore()

	// 检查矿工是否已注册且活跃
	minerKey := []byte(fmt.Sprintf("miner:%s", req.MinerAddr))
	minerBz := store.Get(minerKey)
	if minerBz == nil {
		http.Error(w, "miner not registered", http.StatusForbidden)
		return
	}
	var minerData map[string]interface{}
	json.Unmarshal(minerBz, &minerData)
	if status, ok := minerData["status"].(string); ok && status != "active" {
		http.Error(w, "miner not active", http.StatusForbidden)
		return
	}

	// 获取挑战
	key := []byte(fmt.Sprintf("challenge:%s", req.ChallengeID))
	bz := store.Get(key)
	if bz == nil {
		http.Error(w, "challenge not found", http.StatusNotFound)
		return
	}

	var ch challengetypes.Challenge
	json.Unmarshal(bz, &ch)

	// 检查状态
	if ch.Status != challengetypes.ChallengeStatusPending && ch.Status != challengetypes.ChallengeStatusCommit {
		http.Error(w, fmt.Sprintf("challenge not accepting commits (status: %s)", ch.Status), http.StatusConflict)
		return
	}

	// 检查是否已 commit
	if ch.Commits == nil {
		ch.Commits = make(map[string]string)
	}
	if _, exists := ch.Commits[req.MinerAddr]; exists {
		http.Error(w, "already committed", http.StatusConflict)
		return
	}

	// 存储 commit hash
	ch.Commits[req.MinerAddr] = req.CommitHash
	ch.Status = challengetypes.ChallengeStatusCommit

	bz, _ = json.Marshal(ch)
	store.Set(key, bz)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "commit recorded, please reveal your answer later",
		"status":  ch.Status,
	})
}

// RevealRequest reveal 阶段请求
type RevealRequest struct {
	ChallengeID string `json:"challenge_id"`
	MinerAddr   string `json:"miner_address"`
	Answer      string `json:"answer"`
	Nonce       string `json:"nonce"`
}

// SubmitRevealREST POST /clawchain/challenge/reveal
func (h *RESTHandler) SubmitRevealREST(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var req RevealRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if req.ChallengeID == "" || req.MinerAddr == "" || req.Answer == "" || req.Nonce == "" {
		http.Error(w, "challenge_id, miner_address, answer, and nonce are required", http.StatusBadRequest)
		return
	}

	store := h.getStore()

	// 检查矿工是否已注册
	minerKey := []byte(fmt.Sprintf("miner:%s", req.MinerAddr))
	if !store.Has(minerKey) {
		http.Error(w, "miner not registered", http.StatusForbidden)
		return
	}

	// 获取挑战
	key := []byte(fmt.Sprintf("challenge:%s", req.ChallengeID))
	bz := store.Get(key)
	if bz == nil {
		http.Error(w, "challenge not found", http.StatusNotFound)
		return
	}

	var ch challengetypes.Challenge
	json.Unmarshal(bz, &ch)

	// 验证 commit hash 存在
	if ch.Commits == nil {
		http.Error(w, "no commits found, please commit first", http.StatusBadRequest)
		return
	}
	commitHash, hasCommit := ch.Commits[req.MinerAddr]
	if !hasCommit {
		http.Error(w, "no commit found for this miner, please commit first", http.StatusBadRequest)
		return
	}

	// 验证 sha256(answer+nonce) == commit hash
	h2 := sha256.Sum256([]byte(req.Answer + req.Nonce))
	expectedHash := hex.EncodeToString(h2[:])
	if expectedHash != commitHash {
		http.Error(w, "reveal does not match commit hash", http.StatusForbidden)
		return
	}

	// 防重复 reveal
	if ch.Reveals == nil {
		ch.Reveals = make(map[string]string)
	}
	if _, exists := ch.Reveals[req.MinerAddr]; exists {
		http.Error(w, "already revealed", http.StatusConflict)
		return
	}

	// 存入 Reveals
	ch.Reveals[req.MinerAddr] = req.Answer
	ch.Status = challengetypes.ChallengeStatusReveal

	cms := h.storeGetter()
	currentHeight := cms.LatestVersion()

	submissionCount := len(ch.Reveals)
	requiredSubmissions := 3
	if os.Getenv("CLAWCHAIN_DEV") == "1" {
		requiredSubmissions = 1
	}

	// 结算逻辑（与 SubmitAnswer 一致）
	if submissionCount >= requiredSubmissions {
		answerVotes := make(map[string][]string)
		for minerAddr, answer := range ch.Reveals {
			normalizedAnswer := strings.TrimSpace(strings.ToLower(answer))
			answerVotes[normalizedAnswer] = append(answerVotes[normalizedAnswer], minerAddr)
		}

		var majorityAnswer string
		var majorityMiners []string
		maxVotes := 0
		for answer, miners := range answerVotes {
			if len(miners) > maxVotes {
				maxVotes = len(miners)
				majorityAnswer = answer
				majorityMiners = miners
			}
		}

		minMajority := 2
		if os.Getenv("CLAWCHAIN_DEV") == "1" {
			minMajority = 1
		}
		if maxVotes >= minMajority {
			ch.Status = challengetypes.ChallengeStatusComplete

			epochMinerPool := h.keeper.GetBlockReward(currentHeight)
			numChallengesInEpoch := h.getEpochChallengeCount(store, ch.Epoch)
			if numChallengesInEpoch < 1 {
				numChallengesInEpoch = 1
			}

			perChallengePool := epochMinerPool / int64(numChallengesInEpoch)
			if perChallengePool < 1 {
				perChallengePool = 1
			}

			rewardPerMiner := perChallengePool / int64(len(majorityMiners))
			if rewardPerMiner < 1 {
				rewardPerMiner = 1
			}

			for _, minerAddr := range majorityMiners {
				actualReward := h.applyBonusMultipliers(store, minerAddr, rewardPerMiner)

				pendingKey := []byte(fmt.Sprintf("pending_reward:%d:%s:%s", currentHeight, req.ChallengeID, minerAddr))
				pendingReward := map[string]interface{}{
					"challenge_id": req.ChallengeID,
					"miner_addr":   minerAddr,
					"amount":       actualReward,
					"height":       currentHeight,
				}
				pendingBz, _ := json.Marshal(pendingReward)
				store.Set(pendingKey, pendingBz)

				mKey := []byte(fmt.Sprintf("miner:%s", minerAddr))
				mBz := store.Get(mKey)
				if mBz != nil {
					var mData map[string]interface{}
					if json.Unmarshal(mBz, &mData) == nil {
						completed := int64(0)
						if v, ok := mData["challenges_completed"].(float64); ok {
							completed = int64(v)
						}
						mData["challenges_completed"] = completed + 1
						mBz, _ = json.Marshal(mData)
						store.Set(mKey, mBz)
					}
				}
			}

			// 惩罚不一致的矿工
			for minerAddr, answer := range ch.Reveals {
				normalizedAnswer := strings.TrimSpace(strings.ToLower(answer))
				if normalizedAnswer != majorityAnswer {
					mk := []byte(fmt.Sprintf("miner:%s", minerAddr))
					mb := store.Get(mk)
					if mb != nil {
						var md map[string]interface{}
						json.Unmarshal(mb, &md)
						failed := int64(0)
						if v, ok := md["challenges_failed"].(float64); ok {
							failed = int64(v)
						}
						md["challenges_failed"] = failed + 1
						mb, _ = json.Marshal(md)
						store.Set(mk, mb)
					}
				}
			}
		}
	}

	bz, _ = json.Marshal(ch)
	store.Set(key, bz)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":              true,
		"submission_count":     submissionCount,
		"required_submissions": requiredSubmissions,
		"status":               ch.Status,
		"message":              "reveal recorded",
	})
}

// getEpochChallengeCount 获取指定 epoch 生成的挑战数量
func (h *RESTHandler) getEpochChallengeCount(store storetypes.KVStore, epoch uint64) int {
	count := 0
	prefix := []byte(fmt.Sprintf("challenge:ch-%d-", epoch))
	iter := storetypes.KVStorePrefixIterator(store, prefix)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		count++
	}
	if count == 0 {
		count = 1
	}
	return count
}

// applyBonusMultipliers 应用早鸟倍率和连续在线加成
func (h *RESTHandler) applyBonusMultipliers(store storetypes.KVStore, minerAddr string, baseReward int64) int64 {
	mKey := []byte(fmt.Sprintf("miner:%s", minerAddr))
	mBz := store.Get(mKey)
	if mBz == nil {
		return baseReward
	}

	var mData map[string]interface{}
	if err := json.Unmarshal(mBz, &mData); err != nil {
		return baseReward
	}

	regIndex := uint64(0)
	if v, ok := mData["registration_index"].(float64); ok {
		regIndex = uint64(v)
	}

	consecutiveDays := uint64(0)
	if v, ok := mData["consecutive_days"].(float64); ok {
		consecutiveDays = uint64(v)
	}

	// 早鸟倍率
	earlyBird := uint64(100)
	if regIndex > 0 && regIndex <= 1000 {
		earlyBird = 300
	} else if regIndex <= 5000 {
		earlyBird = 200
	} else if regIndex <= 10000 {
		earlyBird = 150
	}

	// 签到倍率
	streak := uint64(100)
	if consecutiveDays >= 90 {
		streak = 150
	} else if consecutiveDays >= 30 {
		streak = 125
	} else if consecutiveDays >= 7 {
		streak = 110
	}

	completed := int64(0)
	if v, ok := mData["challenges_completed"].(float64); ok {
		completed = int64(v)
	}

	actualReward := baseReward * int64(earlyBird) * int64(streak) / 10000
	if completed < 100 {
		actualReward = actualReward / 2
	}
	if actualReward < 1 {
		actualReward = 1
	}
	return actualReward
}
