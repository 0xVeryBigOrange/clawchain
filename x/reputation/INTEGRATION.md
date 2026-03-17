# x/reputation 模块集成指南

本文档说明如何将 `x/reputation` 模块集成到 ClawChain app 中。

## 前置条件

- Cosmos SDK v0.50.10+
- CometBFT v0.38.12+
- Go 1.22+

## 集成步骤

### 1. 在 app/app.go 中添加导入

```go
import (
    // ... 其他导入
    
    reputationkeeper "github.com/clawchain/clawchain/x/reputation/keeper"
    reputationmodule "github.com/clawchain/clawchain/x/reputation/module"
    reputationtypes "github.com/clawchain/clawchain/x/reputation/types"
)
```

### 2. 在 ClawChainApp 结构体中添加 Keeper

```go
type ClawChainApp struct {
    *baseapp.BaseApp

    // ... 其他 Keeper
    
    PoAKeeper        poakeeper.Keeper
    ChallengeKeeper  challengekeeper.Keeper
    ReputationKeeper reputationkeeper.Keeper  // 新增
}
```

### 3. 注册 StoreKey

```go
func NewClawChainApp(...) *ClawChainApp {
    // ... 
    
    keys := sdk.NewKVStoreKeys(
        // ... 其他 keys
        reputationtypes.StoreKey,
    )
    
    // ...
}
```

### 4. 初始化 Keeper

```go
// 在 NewClawChainApp() 中，在 PoA Keeper 之后初始化

// 创建 Reputation Keeper
app.ReputationKeeper = reputationkeeper.NewKeeper(
    appCodec,
    keys[reputationtypes.StoreKey],
)

// 注入 PoA Slasher 接口（用于作弊时 slash 质押）
app.ReputationKeeper.SetPoASlasher(&app.PoAKeeper)
```

**重要：** `SetPoASlasher` 需要在 `PoAKeeper` 完全初始化后调用。如果 PoA 模块还未实现 `SlashMinerStake` 方法，可以暂时跳过这一步（作弊时不会执行 slash，但会记录事件）。

### 5. 注册模块

```go
// 创建 AppModule
reputationModule := reputationmodule.NewAppModule(app.ReputationKeeper)

// 注册到 Module Manager
app.ModuleManager = module.NewManager(
    // ... 其他模块
    reputationModule,
)

// 设置创世初始化顺序
app.ModuleManager.SetOrderInitGenesis(
    // ... 其他模块
    reputationtypes.ModuleName,
)

// 设置 EndBlocker 执行顺序
// reputation 应在 challenge 之后执行，以处理挑战结果
app.ModuleManager.SetOrderEndBlockers(
    // ... 其他模块
    challengetypes.ModuleName,
    reputationtypes.ModuleName,  // challenge 后执行
)
```

**注意：** `x/reputation` 的 `EndBlocker` 必须在 `x/challenge` 的 `EndBlocker` 之后执行，确保挑战结果事件先提交。

### 6. 注册 BasicManager（用于编解码）

```go
var (
    ModuleBasics = module.NewBasicManager(
        // ... 其他模块
        reputationmodule.AppModuleBasic{},
    )
)
```

### 7. x/challenge 模块集成

在 `x/challenge` 模块中，挑战验证完成后提交声誉事件：

```go
// 在 x/challenge/keeper/keeper.go 或 EndBlocker 中

import reputationtypes "github.com/clawchain/clawchain/x/reputation/types"

// 挑战成功
app.ReputationKeeper.EnqueueEvent(ctx, reputationtypes.PendingReputationEvent{
    Type:         reputationtypes.EventChallengeCompleted,
    MinerAddress: minerAddr.String(),
    ChallengeID:  challenge.ID,
})

// 挑战失败
app.ReputationKeeper.EnqueueEvent(ctx, reputationtypes.PendingReputationEvent{
    Type:         reputationtypes.EventChallengeFailed,
    MinerAddress: minerAddr.String(),
    ChallengeID:  challenge.ID,
})

// 超时未响应
app.ReputationKeeper.EnqueueEvent(ctx, reputationtypes.PendingReputationEvent{
    Type:         reputationtypes.EventTimeout,
    MinerAddress: minerAddr.String(),
    ChallengeID:  challenge.ID,
})

// 作弊检测
app.ReputationKeeper.EnqueueEvent(ctx, reputationtypes.PendingReputationEvent{
    Type:         reputationtypes.EventCheating,
    MinerAddress: minerAddr.String(),
    ChallengeID:  challenge.ID,
})

// 在线心跳（每个 epoch，只给活跃矿工发）
app.ReputationKeeper.EnqueueEvent(ctx, reputationtypes.PendingReputationEvent{
    Type:         reputationtypes.EventOnlineHeartbeat,
    MinerAddress: minerAddr.String(),
})
```

### 8. x/poa 模块集成（可选）

如果需要在作弊时自动 slash 质押，需在 `x/poa/keeper/keeper.go` 中实现接口：

```go
// 实现 reputation.PoASlasher 接口
func (k Keeper) SlashMinerStake(ctx sdk.Context, minerAddr sdk.AccAddress, slashPercent int64) error {
    // 获取矿工质押
    stake, err := k.GetStake(ctx, minerAddr)
    if err != nil {
        return err
    }

    // 计算 slash 金额
    slashAmount := stake.Amount.Mul(sdk.NewInt(slashPercent)).Quo(sdk.NewInt(100))

    // 扣除质押（转入 community pool 或销毁）
    if err := k.deductStake(ctx, minerAddr, slashAmount); err != nil {
        return err
    }

    // 记录 slash 事件
    ctx.EventManager().EmitEvent(
        sdk.NewEvent(
            "miner_slashed",
            sdk.NewAttribute("miner", minerAddr.String()),
            sdk.NewAttribute("amount", slashAmount.String()),
            sdk.NewAttribute("reason", "cheating"),
        ),
    )

    return nil
}
```

### 9. 创世状态配置

在 `genesis.json` 中添加 reputation 模块配置：

```json
{
  "app_state": {
    "reputation": {
      "scores": [
        {
          "miner_address": "claw1abc...",
          "score": 800,
          "tier": "elite",
          "total_challenges": 100,
          "completed_challenges": 95,
          "failed_challenges": 5,
          "consecutive_online_blocks": 100,
          "last_update_height": 12345
        }
      ]
    }
  }
}
```

大部分情况下使用默认空配置即可：

```json
{
  "app_state": {
    "reputation": {
      "scores": []
    }
  }
}
```

## 查询示例

### CLI 查询（需实现 CLI 命令）

```bash
# 查询矿工声誉
clawchaind query reputation score claw1abc...

# 查询排行榜（前 50 名）
clawchaind query reputation leaderboard --limit 50

# 查询声誉历史（最近 20 条）
clawchaind query reputation history claw1abc... --limit 20
```

### 编程查询

```go
// 在其他模块中查询声誉
score, err := app.ReputationKeeper.GetScore(ctx, minerAddr)
if err != nil {
    return err
}

// 检查矿工是否有资格参与挑战
if app.ReputationKeeper.IsMinerSuspended(ctx, minerAddr) {
    return fmt.Errorf("矿工声誉过低，暂停挖矿")
}

// 获取排行榜
leaderboard := app.ReputationKeeper.GetLeaderboard(ctx, 100)
```

## 事件监听示例（区块浏览器/客户端）

```go
// 监听声誉更新事件
for _, event := range ctx.EventManager().Events() {
    if event.Type == reputationtypes.EventTypeReputationUpdated {
        miner := event.GetAttribute("miner")
        score := event.GetAttribute("score")
        delta := event.GetAttribute("delta")
        fmt.Printf("矿工 %s 声誉更新: %s (%s)\n", miner, score, delta)
    }
}

// 监听矿工暂停事件
if event.Type == reputationtypes.EventTypeMinerSuspended {
    miner := event.GetAttribute("miner")
    // 通知客户端矿工被暂停
}

// 监听作弊检测事件
if event.Type == reputationtypes.EventTypeCheatingDetected {
    miner := event.GetAttribute("miner")
    challengeID := event.GetAttribute("challenge_id")
    // 记录作弊行为，可能需要人工审核
}
```

## 测试验证

### 1. 编译测试

```bash
cd chain
go build ./...
```

### 2. 单元测试

```bash
go test ./x/reputation/...
```

### 3. 集成测试

启动本地测试网，验证完整流程：

```bash
# 初始化
clawchaind init test --chain-id clawchain-testnet-1

# 添加测试账户
clawchaind keys add miner1
clawchaind keys add miner2

# 启动节点
clawchaind start

# 注册矿工（假设 x/poa 已实现）
clawchaind tx poa register-miner --from miner1

# 提交挑战（假设 x/challenge 已实现）
clawchaind tx challenge submit-commit <hash> --from miner1

# 查询声誉（几个区块后）
clawchaind query reputation score $(clawchaind keys show miner1 -a)
```

## 常见问题

**Q: EndBlocker 执行顺序很重要吗？**  
A: 是的。`x/reputation` 必须在 `x/challenge` 之后执行，因为 challenge 的 EndBlocker 会提交声誉事件，reputation 的 EndBlocker 负责处理这些事件。

**Q: 如果 PoA 模块还没实现 SlashMinerStake 怎么办？**  
A: 不调用 `SetPoASlasher` 即可。Keeper 内部会检查 `poaSlasher != nil`，如果为 nil 则跳过 slash，但仍会记录作弊事件和扣除声誉分。

**Q: 声誉分会在链升级时丢失吗？**  
A: 不会。所有数据存储在链上 KVStore，通过 InitGenesis/ExportGenesis 支持导入导出。升级前导出创世状态，升级后恢复即可。

**Q: 如何手动修复错误的声誉分？**  
A: 通过治理提案，或在 x/reputation 中添加 `MsgUpdateScore` 消息（需权限验证）。

**Q: 如何防止声誉分被滥用？**  
A: 事件提交只能由 x/challenge 模块调用（keeper 不暴露给交易），确保声誉变化只能通过链上验证的挑战触发。

## 后续优化

- [ ] 实现 gRPC 查询服务
- [ ] 添加 CLI 命令
- [ ] 添加完整的单元测试和集成测试
- [ ] 实现声誉历史按时间范围查询
- [ ] 实现声誉分衰减机制（可选）

---

**当前状态：** ✅ 核心功能完整，可直接集成  
**下一步：** 集成到 app.go 并测试 EndBlocker 流程
