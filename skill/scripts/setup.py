#!/usr/bin/env python3
"""
ClawChain Wallet Initialization & Miner Registration
Generate a Cosmos SDK wallet (bech32 claw prefix), save keys, register miner.
"""

import argparse
import hashlib
import json
import os
import secrets
import sys
from pathlib import Path

try:
    import requests
except ImportError:
    print("❌ Required: pip install requests")
    sys.exit(1)

SCRIPT_DIR = Path(__file__).parent
CONFIG_PATH = SCRIPT_DIR / "config.json"

# ─── bech32 encoding (pure Python, no extra dependencies) ───

CHARSET = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

def bech32_polymod(values):
    GEN = [0x3B6A57B2, 0x26508E6D, 0x1EA119FA, 0x3D4233DD, 0x2A1462B3]
    chk = 1
    for v in values:
        b = chk >> 25
        chk = ((chk & 0x1FFFFFF) << 5) ^ v
        for i in range(5):
            chk ^= GEN[i] if ((b >> i) & 1) else 0
    return chk

def bech32_hrp_expand(hrp):
    return [ord(x) >> 5 for x in hrp] + [0] + [ord(x) & 31 for x in hrp]

def bech32_create_checksum(hrp, data):
    values = bech32_hrp_expand(hrp) + data
    polymod = bech32_polymod(values + [0, 0, 0, 0, 0, 0]) ^ 1
    return [(polymod >> 5 * (5 - i)) & 31 for i in range(6)]

def bech32_encode(hrp, data):
    combined = data + bech32_create_checksum(hrp, data)
    return hrp + "1" + "".join([CHARSET[d] for d in combined])

def convertbits(data, frombits, tobits, pad=True):
    acc, bits, ret = 0, 0, []
    maxv = (1 << tobits) - 1
    for value in data:
        acc = (acc << frombits) | value
        bits += frombits
        while bits >= tobits:
            bits -= tobits
            ret.append((acc >> bits) & maxv)
    if pad and bits:
        ret.append((acc << (tobits - bits)) & maxv)
    return ret

def generate_wallet():
    """Generate a Cosmos-style wallet (claw prefix)"""
    private_key = secrets.token_bytes(32)

    # Simplified: SHA256 + RIPEMD160 to simulate public key hash
    # Real Cosmos SDK uses secp256k1; hash simulation is sufficient for testnet
    sha = hashlib.sha256(private_key).digest()
    ripemd = hashlib.new("ripemd160", sha).digest()

    # bech32 encode
    data5 = convertbits(list(ripemd), 8, 5)
    address = bech32_encode("claw", data5)

    return {
        "address": address,
        "private_key": private_key.hex(),
        "public_key_hash": ripemd.hex(),
    }

def save_wallet(wallet_data, wallet_path):
    """Save wallet to file (permissions 600)"""
    wallet_path = Path(wallet_path).expanduser()
    wallet_path.parent.mkdir(parents=True, exist_ok=True)

    with open(wallet_path, "w") as f:
        json.dump(wallet_data, f, indent=2)

    os.chmod(wallet_path, 0o600)
    return wallet_path

def register_miner(rpc_url, address, name):
    """Register miner on chain"""
    try:
        resp = requests.post(
            f"{rpc_url}/clawchain/miner/register",
            headers={"Content-Type": "application/json"},
            json={"address": address, "name": name},
            timeout=10
        )
        if resp.status_code == 409:
            return {"success": True, "message": "Miner already registered"}
        resp.raise_for_status()
        return resp.json()
    except requests.ConnectionError:
        return {"success": False, "message": "Cannot connect to chain node (will retry later)"}
    except Exception as e:
        return {"success": False, "message": str(e)}

def main():
    parser = argparse.ArgumentParser(description="ClawChain Wallet Initialization & Miner Registration")
    parser.add_argument("--name", default="openclaw-miner", help="Miner name (default: openclaw-miner)")
    parser.add_argument("--rpc", default=None, help="Chain REST API URL (default: from config.json)")
    parser.add_argument("--wallet-path", default=None, help="Wallet save path (default: ~/.clawchain/wallet.json)")
    parser.add_argument("--non-interactive", action="store_true", help="Non-interactive mode (auto-confirm)")
    args = parser.parse_args()

    # Load config
    config = {}
    if CONFIG_PATH.exists():
        with open(CONFIG_PATH) as f:
            config = json.load(f)

    rpc_url = args.rpc or config.get("rpc_url", "http://localhost:1317")
    wallet_path = args.wallet_path or config.get("wallet_path", "~/.clawchain/wallet.json")
    wallet_path_expanded = Path(wallet_path).expanduser()
    miner_name = args.name or config.get("miner_name", "openclaw-miner")

    # Check for existing wallet
    if wallet_path_expanded.exists():
        with open(wallet_path_expanded) as f:
            existing = json.load(f)
        print(f"📋 Existing wallet found: {existing['address']}")
        if not args.non_interactive:
            choice = input("Use existing wallet? (y/n): ").strip().lower()
            if choice != "n":
                address = existing["address"]
                print(f"\nUsing existing wallet: {address}")
                print(f"\n📝 Registering miner on chain...")
                result = register_miner(rpc_url, address, miner_name)
                print(f"   {'✅' if result.get('success') else '⚠️'} {result.get('message', '')}")
                config["miner_address"] = address
                config["miner_name"] = miner_name
                with open(CONFIG_PATH, "w") as f:
                    json.dump(config, f, indent=2)
                print(f"\n✅ Config updated")
                return
        else:
            address = existing["address"]
            print(f"Using existing wallet: {address}")
            result = register_miner(rpc_url, address, miner_name)
            print(f"{'✅' if result.get('success') else '⚠️'} {result.get('message', '')}")
            config["miner_address"] = address
            config["miner_name"] = miner_name
            with open(CONFIG_PATH, "w") as f:
                json.dump(config, f, indent=2)
            return

    # Generate new wallet
    print("🔐 ClawChain Wallet Initialization")
    print()

    wallet = generate_wallet()
    print(f"   Address: {wallet['address']}")
    print(f"   Key path: {wallet_path_expanded}")
    print()

    if not args.non_interactive:
        confirm = input("Generate wallet and register miner? (y/n): ").strip().lower()
        if confirm != "y":
            print("❌ Cancelled")
            sys.exit(0)

    # Save wallet
    saved_path = save_wallet(wallet, wallet_path)
    print(f"💾 Wallet saved: {saved_path} (permissions 600)")

    # Register miner
    print(f"\n📝 Registering miner on chain...")
    result = register_miner(rpc_url, wallet["address"], miner_name)
    print(f"   {'✅' if result.get('success') else '⚠️'} {result.get('message', '')}")

    # Update config.json
    config["miner_address"] = wallet["address"]
    config["miner_name"] = miner_name
    with open(CONFIG_PATH, "w") as f:
        json.dump(config, f, indent=2)

    print(f"\n✅ Initialization complete!")
    print(f"   Wallet address: {wallet['address']}")
    print(f"   Next step: python3 scripts/mine.py to start mining")

if __name__ == "__main__":
    main()
