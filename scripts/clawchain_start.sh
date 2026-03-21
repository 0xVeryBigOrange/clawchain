#!/bin/bash
# ClawChain testnet start script
# Used by LaunchAgent com.clawchain.testnet

CHAIN_HOME="$HOME/.clawchain-testnet"
CLAWCHAIND="/Users/orbot/.openclaw/workspace/projects/clawchain/build/clawchaind"

export CLAWCHAIN_DEV=1
export CLAWCHAIN_DEV_MODE=1

if [ ! -f "$CLAWCHAIND" ]; then
    echo "ERROR: clawchaind not found at $CLAWCHAIND"
    exit 1
fi

if [ ! -d "$CHAIN_HOME/data" ]; then
    echo "ERROR: testnet data not found at $CHAIN_HOME/data"
    echo "Run setup_testnet.sh first"
    exit 1
fi

exec "$CLAWCHAIND" start --home "$CHAIN_HOME"
