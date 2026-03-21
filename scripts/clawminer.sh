#!/bin/bash
# ClawMiner Direct Client — Wrapper around clawchaind tx CLI
# Provides end-to-end mining operations through the chain.
#
# Usage:
#   ./clawminer.sh register <address> [stake]
#   ./clawminer.sh submit-commit <address> <challenge-id> <commit-hash>
#   ./clawminer.sh submit-reveal <address> <challenge-id> <answer> <salt>
#   ./clawminer.sh status
#   ./clawminer.sh mine <address>   # auto-mine loop

set -e

CLAWCHAIND="${CLAWCHAIND:-./build/clawchaind}"
NODE="${NODE:-tcp://localhost:26657}"
CHAIN_ID="${CHAIN_ID:-clawchain-testnet-1}"
KEYRING_BACKEND="${KEYRING_BACKEND:-test}"
KEYRING_DIR="${KEYRING_DIR:-$HOME/.clawchain-testnet}"
KEY_NAME="${KEY_NAME:-validator}"
FEES="${FEES:-10uclaw}"
GAS="${GAS:-200000}"

# Track sequence number
SEQ_FILE="/tmp/clawminer_seq_$(echo -n $KEY_NAME | md5 -q 2>/dev/null || echo -n $KEY_NAME | md5sum 2>/dev/null | cut -c1-8 || echo $KEY_NAME)"

get_sequence() {
    if [ -f "$SEQ_FILE" ]; then
        cat "$SEQ_FILE"
    else
        echo "0"
    fi
}

bump_sequence() {
    local seq=$(get_sequence)
    echo $((seq + 1)) > "$SEQ_FILE"
}

# Build → Sign → Broadcast a tx
send_tx() {
    local tx_type=$1
    shift
    local seq=$(get_sequence)
    
    # Generate
    $CLAWCHAIND tx $tx_type "$@" \
        --from $(get_address) \
        --fees $FEES --gas $GAS \
        --generate-only --chain-id $CHAIN_ID \
        > /tmp/clawminer_tx.json 2>&1
    
    # Sign (offline with explicit account/sequence)
    $CLAWCHAIND tx sign /tmp/clawminer_tx.json \
        --from $KEY_NAME \
        --keyring-backend $KEYRING_BACKEND \
        --keyring-dir $KEYRING_DIR \
        --chain-id $CHAIN_ID \
        --account-number 0 --sequence $seq \
        --offline \
        > /tmp/clawminer_signed.json 2>&1
    
    # Broadcast
    local result=$($CLAWCHAIND tx broadcast /tmp/clawminer_signed.json --node $NODE 2>&1)
    local code=$(echo "$result" | python3 -c "import sys,json; print(json.loads(sys.stdin.read()).get('code','?'))" 2>/dev/null || echo "?")
    local txhash=$(echo "$result" | python3 -c "import sys,json; print(json.loads(sys.stdin.read()).get('txhash','?')[:20])" 2>/dev/null || echo "?")
    
    if [ "$code" = "0" ]; then
        bump_sequence
        echo "✅ TX accepted: code=$code hash=$txhash..."
    else
        echo "❌ TX failed: code=$code"
        echo "$result"
    fi
}

get_address() {
    $CLAWCHAIND keys show $KEY_NAME --keyring-backend $KEYRING_BACKEND --keyring-dir $KEYRING_DIR --address 2>/dev/null
}

case "$1" in
    register)
        ADDR=${2:-$(get_address)}
        STAKE=${3:-0}
        echo "📝 Registering miner: $ADDR (stake: $STAKE)"
        send_tx "poa register" "$ADDR" "$STAKE"
        ;;
    
    submit-commit)
        ADDR=$2; CH_ID=$3; COMMIT=$4
        echo "📤 Submitting commit for $CH_ID"
        send_tx "challenge submit-commit" "$ADDR" "$CH_ID" "$COMMIT"
        ;;
    
    submit-reveal)
        ADDR=$2; CH_ID=$3; ANSWER=$4; SALT=$5
        echo "📤 Submitting reveal for $CH_ID"
        send_tx "challenge submit-reveal" "$ADDR" "$CH_ID" "$ANSWER" "$SALT"
        ;;
    
    status)
        echo "📡 Node status:"
        curl -s $NODE/status | python3 -c "
import sys,json
d=json.load(sys.stdin)
si=d['result']['sync_info']
ni=d['result']['node_info']
print(f'  Network: {ni[\"network\"]}')
print(f'  Height:  {si[\"latest_block_height\"]}')
print(f'  Syncing: {si[\"catching_up\"]}')
" 2>&1
        ;;
    
    *)
        echo "Usage: $0 {register|submit-commit|submit-reveal|status} [args...]"
        exit 1
        ;;
esac
