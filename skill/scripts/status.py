#!/usr/bin/env python3
"""
ClawChain Miner Status Query
"""

import argparse
import json
import sys
from pathlib import Path
from urllib.parse import urlparse

try:
    import requests
except ImportError:
    print("❌ Required: pip install requests")
    sys.exit(1)

SCRIPT_DIR = Path(__file__).parent
CONFIG_PATH = SCRIPT_DIR / "config.json"
DATA_DIR = SCRIPT_DIR.parent / "data"
LOG_PATH = DATA_DIR / "mining_log.json"

# Import wallet crypto for status checks
sys.path.insert(0, str(SCRIPT_DIR))
from wallet_crypto import detect_wallet_version


def load_config():
    """Load configuration with backward compatibility for node_url → rpc_url migration."""
    if not CONFIG_PATH.exists():
        print(f"❌ Config file not found: {CONFIG_PATH}")
        sys.exit(1)
    with open(CONFIG_PATH) as f:
        config = json.load(f)

    # Backward compat: migrate node_url → rpc_url
    if "rpc_url" not in config and "node_url" in config:
        print("⚠️  DEPRECATION: 'node_url' is deprecated, migrating to 'rpc_url'. Please update config.json.")
        config["rpc_url"] = config.pop("node_url")
        with open(CONFIG_PATH, "w") as f:
            json.dump(config, f, indent=2)

    if "rpc_url" not in config:
        print("❌ 'rpc_url' not set in config.json")
        sys.exit(1)

    return config


def warn_insecure_rpc(url):
    """Warn if RPC URL uses plain HTTP on a non-localhost endpoint."""
    parsed = urlparse(url)
    if parsed.scheme == "http" and parsed.hostname not in ("localhost", "127.0.0.1", "::1"):
        print(f"⚠️  SECURITY WARNING: RPC endpoint uses plain HTTP ({url}). Use HTTPS for production.")


def get_miner_stats(rpc_url, address):
    """Get on-chain miner stats"""
    try:
        resp = requests.get(f"{rpc_url}/clawchain/miner/{address}/stats", timeout=10)
        if resp.status_code == 404:
            return None
        resp.raise_for_status()
        return resp.json()
    except requests.ConnectionError:
        print("⚠️ Cannot connect to chain node")
        return None
    except Exception as e:
        print(f"⚠️ Query failed: {e}")
        return None


def get_chain_stats(rpc_url):
    """Get chain statistics"""
    try:
        resp = requests.get(f"{rpc_url}/clawchain/stats", timeout=10)
        resp.raise_for_status()
        return resp.json()
    except Exception:
        return None


def get_local_logs():
    """Read local mining log"""
    if not LOG_PATH.exists():
        return []
    try:
        with open(LOG_PATH) as f:
            return json.load(f)
    except (json.JSONDecodeError, IOError):
        return []


def main():
    parser = argparse.ArgumentParser(description="ClawChain Miner Status")
    parser.add_argument("--json", action="store_true", help="JSON output")
    parser.add_argument("--chain", action="store_true", help="Show chain statistics")
    parser.add_argument("--logs", type=int, default=10, help="Show last N mining records (default: 10)")
    args = parser.parse_args()

    config = load_config()
    rpc_url = config["rpc_url"]
    warn_insecure_rpc(rpc_url)

    address = config.get("miner_address", "")

    if not address:
        print("❌ Miner address not configured. Run first: python3 scripts/setup.py")
        sys.exit(1)

    # On-chain stats
    stats = get_miner_stats(rpc_url, address)
    chain = get_chain_stats(rpc_url) if args.chain else None
    logs = get_local_logs()

    if args.json:
        output = {
            "miner": stats,
            "chain": chain,
            "local_logs_count": len(logs),
            "recent_logs": logs[-args.logs:] if logs else [],
        }
        print(json.dumps(output, indent=2, ensure_ascii=False))
        return

    print("📊 ClawChain Mining Status")
    print("=" * 40)

    # Miner info
    print(f"\n🔗 Miner: {address}")
    if stats:
        print(f"   Challenges completed: {stats.get('challenges_completed', 0)}")
        print(f"   Challenges failed:    {stats.get('challenges_failed', 0)}")
        print(f"   Success rate:         {stats.get('success_rate', '0.00%')}")
        print(f"   Total rewards:        {stats.get('total_rewards_uclaw', '0 uclaw')}")
    else:
        print("   ❌ Not registered or chain node unreachable")

    # Chain stats
    if chain:
        print(f"\n🌐 Chain Statistics:")
        print(f"   Active miners:    {chain.get('active_miners', 0)}")
        print(f"   Total challenges: {chain.get('total_challenges', 0)}")
        print(f"   Completed:        {chain.get('completed_challenges', 0)}")
        print(f"   Total rewards:    {chain.get('total_rewards_uclaw', '0 uclaw')}")
        print(f"   Current block:    {chain.get('current_block_height', 0)}")
        print(f"   Current reward:   {chain.get('current_reward_uclaw', '0 uclaw')}")

    # Local logs
    recent = logs[-args.logs:] if logs else []
    print(f"\n📝 Last {len(recent)} mining records:")
    if not recent:
        print("   No records yet")
    else:
        for i, log in enumerate(recent, 1):
            ts = log.get("timestamp", "")[:19]
            ctype = log.get("type", log.get("challenge_type", "?"))
            method = log.get("method", "?")
            status = log.get("status", "?")
            status_emoji = "✅" if status in ("complete", "reveal") else "⏳" if status == "pending" else "❌"
            print(f"   {i}. {status_emoji} [{ts}] {ctype} ({method}) → {status}")

    print()


if __name__ == "__main__":
    main()
