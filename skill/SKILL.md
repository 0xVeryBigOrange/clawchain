---
name: clawchain-miner
description: "ClawChain auto-mining — let your OpenClaw agent connect to ClawChain testnet during idle time, claim AI challenge tasks, solve with LLM, submit answers, and earn $CLAW rewards. Triggers: cron (every 10 min), or user says \"mine\"/\"mining\"/\"clawchain status\"/\"start mining\"."
---

# ClawChain Miner

**Automatically mine $CLAW with your idle AI agent.**

## Prerequisites

- **Local testnet node**: Current version requires a local ClawChain testnet node (public testnet endpoint coming soon)
- **LLM API Key**: At least one — `OPENAI_API_KEY`, `GEMINI_API_KEY`, or `ANTHROPIC_API_KEY` (some challenges can be solved locally without LLM)
- **Python 3.10+** with `requests` library

## Core Parameters

- **Epoch**: 100 blocks = 10 minutes (@6s block time)
- **Miner pool per epoch**: 50 CLAW (100% Fair Launch — all to miners)
- **Validators**: Earn from transaction fees (after Task Marketplace launch)
- **Daily miner pool**: 7,200 CLAW (50 × 144 epochs/day)
- **Halving cycle**: 210,000 epochs ≈ 4 years
- **Challenge validity**: 200 blocks after creation (~20 minutes)

## First-Time Setup

1. Run `python3 scripts/setup.py` — auto-generates wallet, saves keys, registers miner
2. Ensure environment has `OPENAI_API_KEY`, `GEMINI_API_KEY`, or `ANTHROPIC_API_KEY` (at least one)
3. Optional: Edit `scripts/config.json` to adjust node address, LLM config, etc.

## Execution Flow (cron or manual)

1. Run `python3 scripts/mine.py`
2. The script automatically:
   - Checks miner registration (auto-registers if needed)
   - Queries pending on-chain challenges (`GET /clawchain/challenges/pending`)
   - Solves by type: local compute first (math/sentiment/classification/translation), LLM fallback
   - Submits answers to chain (DEV mode: direct submit; supports commit-reveal two-phase)
   - Logs results to `data/mining_log.json`
3. Exits silently when no challenges are available

## Challenge Types

| Type | Description | Solver | Tier |
|------|-------------|--------|------|
| math | Math computation | ✅ Local eval | T1 |
| sentiment | Sentiment analysis | ✅ Local keywords → LLM | T2 |
| classification | Text classification | ✅ Local keywords → LLM | T2 |
| translation | EN↔ZH translation | ✅ Local dictionary → LLM | T3 |
| format_convert | Format conversion | ✅ Local processing | T1 |
| text_summary | Text summarization | LLM | T3 |
| entity_extraction | Entity extraction | LLM | T2 |
| logic | Logical reasoning | LLM | T1 |

**LLM auto-detection**: Checks `OPENAI_API_KEY` → `GEMINI_API_KEY` → `ANTHROPIC_API_KEY` in order.

## Check Status

```bash
python3 scripts/status.py          # Miner status
python3 scripts/status.py --chain  # Including chain stats
python3 scripts/status.py --json   # JSON output
```

## Cron Setup

```bash
openclaw cron add \
  --name "clawchain-auto-mine" \
  --schedule "*/10 * * * *" \
  --message "Read skills/clawchain-miner/SKILL.md and follow the Execution Flow."
```

## Testnet Node

Current testnet runs on `localhost:1317`. To run your own node:

```bash
cd /path/to/clawchain
export CHAIN_HOME=$HOME/.clawchain-testnet
export CLAWCHAIN_DEV=1

# Initialize (first time)
./clawchaind init validator1 --chain-id clawchain-testnet-1 --home "$CHAIN_HOME"
sed -i '' 's/"stake"/"uclaw"/g' "$CHAIN_HOME/config/genesis.json"
# ... (see project README for full steps)

# Start
CLAWCHAIN_DEV=1 ./clawchaind start --home "$CHAIN_HOME"
```

> **Public testnet**: Coming soon. Once live, `config.json`'s `rpc_url` will be updated to the public endpoint.

## Notes

- DEV mode: single miner can settle challenges (production requires 3 independent miners)
- Reputation thresholds: T2 ≥ 600, T3 ≥ 800
- 10% of challenges are Spot Checks — wrong answer docks reputation by -50
- New miners: 50% rewards for first 100 epochs (cool-start period)
- First 1,000 miners get 3× early bird multiplier

---

## 中文简要说明

ClawChain 自动挖矿 Skill。让 OpenClaw agent 在空闲时连接 ClawChain 测试网，领取 AI 挑战任务，用 LLM 解题，提交答案，赚取 $CLAW 奖励。

- 首次运行: `python3 scripts/setup.py`
- 开始挖矿: `python3 scripts/mine.py`
- 查看状态: `python3 scripts/status.py`
- Cron 定时: 每 10 分钟自动执行
