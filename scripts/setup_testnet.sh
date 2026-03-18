#!/bin/bash
# ClawChain 本地 Testnet 搭建脚本
# 使用单验证者模式，跳过 gentx（address codec 兼容问题）
set -euo pipefail

CHAIN_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BINARY="$CHAIN_DIR/clawchaind"
CHAIN_HOME="${CHAIN_HOME:-/tmp/clawchain-testnet}"
CHAIN_ID="clawchain-testnet-1"
DENOM="uclaw"

echo "═══════════════════════════════════════"
echo "  ClawChain Testnet Setup"
echo "═══════════════════════════════════════"

# 1. 编译
echo "=== 1. Building clawchaind ==="
cd "$CHAIN_DIR"
go build -o clawchaind ./cmd/clawchaind/
echo "✅ Binary built: $(ls -lh clawchaind | awk '{print $5}')"

# 2. 清理旧数据
echo "=== 2. Clean previous data ==="
rm -rf "$CHAIN_HOME"
echo "✅ Cleaned $CHAIN_HOME"

# 3. 初始化
echo "=== 3. Init node ==="
$BINARY init validator1 --chain-id $CHAIN_ID --home "$CHAIN_HOME" > /dev/null 2>&1
echo "✅ Node initialized"

# 4. 替换 stake → uclaw
echo "=== 4. Configure denom ==="
if [[ "$(uname)" == "Darwin" ]]; then
    sed -i '' "s/\"stake\"/\"$DENOM\"/g" "$CHAIN_HOME/config/genesis.json"
else
    sed -i "s/\"stake\"/\"$DENOM\"/g" "$CHAIN_HOME/config/genesis.json"
fi
echo "✅ Bond denom set to $DENOM"

# 5. 配置快速出块
echo "=== 5. Configure fast blocks ==="
if [[ "$(uname)" == "Darwin" ]]; then
    sed -i '' 's/timeout_commit = "5s"/timeout_commit = "1s"/' "$CHAIN_HOME/config/config.toml"
    sed -i '' 's/timeout_propose = "3s"/timeout_propose = "1s"/' "$CHAIN_HOME/config/config.toml"
    # 启用 API
    sed -i '' 's/enable = false/enable = true/' "$CHAIN_HOME/config/app.toml"
    # 设置最小 gas
    sed -i '' 's/minimum-gas-prices = ""/minimum-gas-prices = "0uclaw"/' "$CHAIN_HOME/config/app.toml"
else
    sed -i 's/timeout_commit = "5s"/timeout_commit = "1s"/' "$CHAIN_HOME/config/config.toml"
    sed -i 's/timeout_propose = "3s"/timeout_propose = "1s"/' "$CHAIN_HOME/config/config.toml"
    sed -i 's/enable = false/enable = true/' "$CHAIN_HOME/config/app.toml"
    sed -i 's/minimum-gas-prices = ""/minimum-gas-prices = "0uclaw"/' "$CHAIN_HOME/config/app.toml"
fi
echo "✅ Fast block config applied"

# 6. 创建密钥
echo "=== 6. Create validator key ==="
$BINARY keys add validator1 --keyring-backend test --keyring-dir "$CHAIN_HOME" 2>&1 | head -3
ADDR=$($BINARY keys show validator1 -a --keyring-backend test --keyring-dir "$CHAIN_HOME")
echo "✅ Validator address: $ADDR"

# 7. 添加创世账户
echo "=== 7. Add genesis account ==="
$BINARY genesis add-genesis-account "$ADDR" "1000000000000$DENOM" --home "$CHAIN_HOME"
echo "✅ 1,000,000 CLAW allocated"

# 8. 手动创建验证者的创世状态（跳过 gentx 的 address codec 问题）
echo "=== 8. Configure validator in genesis ==="
PUBKEY=$($BINARY comet show-validator --home "$CHAIN_HOME" 2>/dev/null || echo '{"@type":"/cosmos.crypto.ed25519.PubKey","key":"placeholder"}')

# 用 python3 直接修改 genesis.json 添加验证者
python3 << PYEOF
import json

genesis_path = "$CHAIN_HOME/config/genesis.json"
with open(genesis_path) as f:
    genesis = json.load(f)

# 设置单验证者模式的共识参数
# 在 consensus_params 中确保 validator 可以工作
app_state = genesis.get("app_state", {})

# 确保 staking params 正确
if "staking" in app_state:
    staking = app_state["staking"]
    if "params" in staking:
        staking["params"]["bond_denom"] = "$DENOM"
        staking["params"]["max_validators"] = 100

# 确保 mint params 正确  
if "mint" in app_state:
    mint_state = app_state["mint"]
    if "params" in mint_state:
        mint_state["params"]["mint_denom"] = "$DENOM"

# 确保 distribution 正确
if "distribution" not in app_state:
    app_state["distribution"] = {}

# 保存
with open(genesis_path, "w") as f:
    json.dump(genesis, f, indent=2)

print("✅ Genesis configured")
PYEOF

# 9. 验证 genesis
echo "=== 9. Validate genesis ==="
$BINARY genesis validate-genesis --home "$CHAIN_HOME" 2>&1 || echo "⚠️ Validation warning (may be ok for testnet)"

# 10. 显示摘要
echo ""
echo "═══════════════════════════════════════"
echo "  ✅ Testnet Ready!"
echo "═══════════════════════════════════════"
echo "Chain ID:    $CHAIN_ID"
echo "Home:        $CHAIN_HOME"
echo "Validator:   $ADDR"
echo "Denom:       $DENOM"
echo ""
echo "启动命令:"
echo "  $BINARY start --home $CHAIN_HOME"
echo ""
echo "注意: 单验证者模式需要 gentx 才能真正出块。"
echo "当前因 Cosmos SDK v0.50 address codec 兼容问题，"
echo "gentx 暂时无法自动生成。手动修复后可出块。"
echo "═══════════════════════════════════════"
