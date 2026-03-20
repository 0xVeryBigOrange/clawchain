#!/usr/bin/env python3
"""
ClawChain Wallet Encryption Module

Provides PBKDF2 + Fernet encryption for wallet private keys.
Falls back to base64 obfuscation if `cryptography` is not installed.

Wallet file format v2 (encrypted):
{
  "version": 2,
  "encrypted": true,
  "address": "claw1...",
  "public_key_hash": "...",
  "data": "<Fernet-encrypted base64 string>",
  "kdf": "pbkdf2",
  "salt": "<hex salt>",
  "_warning": "..."
}

Wallet file format v1 (obfuscated, legacy):
{
  "address": "claw1...",
  "private_key_encoded": "<base64 obfuscated>",
  "public_key_hash": "...",
  "_warning": "..."
}
"""

import base64
import getpass
import json
import os
import sys
from pathlib import Path

# Try importing cryptography; set flag for fallback
try:
    from cryptography.fernet import Fernet
    from cryptography.hazmat.primitives.kdf.pbkdf2 import PBKDF2HMAC
    from cryptography.hazmat.primitives import hashes
    HAS_CRYPTO = True
except ImportError:
    HAS_CRYPTO = False

WALLET_VERSION = 2
_LEGACY_MARKER = b"CLAWCHAIN_TESTNET_KEY_V1:"


# ─── Passphrase Handling ───

def _get_passphrase(confirm=False, purpose="wallet"):
    """Get passphrase from env var or interactive prompt.

    Priority:
      1. CLAWCHAIN_WALLET_PASSPHRASE env var
      2. Interactive prompt (if stdin is a TTY)
      3. None (caller decides: insecure mode or error)
    """
    env_pass = os.getenv("CLAWCHAIN_WALLET_PASSPHRASE")
    if env_pass:
        return env_pass

    if sys.stdin.isatty():
        passphrase = getpass.getpass(f"🔑 Enter passphrase for {purpose}: ")
        if not passphrase:
            return None
        if confirm:
            confirm_pass = getpass.getpass(f"🔑 Confirm passphrase: ")
            if passphrase != confirm_pass:
                print("❌ Passphrases do not match")
                return None
        return passphrase

    return None


def _derive_key(passphrase: str, salt: bytes) -> bytes:
    """Derive Fernet key from passphrase using PBKDF2."""
    kdf = PBKDF2HMAC(
        algorithm=hashes.SHA256(),
        length=32,
        salt=salt,
        iterations=480000,
    )
    key = base64.urlsafe_b64encode(kdf.derive(passphrase.encode()))
    return key


# ─── Legacy (v1) obfuscation ───

def _obfuscate_key(private_key_hex: str) -> str:
    """Base64 obfuscation (v1 legacy, NOT real encryption)."""
    return base64.b64encode(_LEGACY_MARKER + bytes.fromhex(private_key_hex)).decode()


def _deobfuscate_key(encoded: str) -> str:
    """Reverse v1 obfuscation."""
    raw = base64.b64decode(encoded)
    if raw.startswith(_LEGACY_MARKER):
        return raw[len(_LEGACY_MARKER):].hex()
    return raw.hex()


# ─── Encrypt / Decrypt ───

def encrypt_private_key(private_key_hex: str, passphrase: str) -> dict:
    """Encrypt private key with PBKDF2 + Fernet. Returns dict with encrypted data + salt."""
    if not HAS_CRYPTO:
        raise RuntimeError("cryptography library not installed")

    salt = os.urandom(16)
    key = _derive_key(passphrase, salt)
    f = Fernet(key)
    encrypted = f.encrypt(private_key_hex.encode()).decode()

    return {
        "data": encrypted,
        "salt": salt.hex(),
        "kdf": "pbkdf2",
    }


def decrypt_private_key(encrypted_data: str, salt_hex: str, passphrase: str) -> str:
    """Decrypt private key with PBKDF2 + Fernet. Returns hex private key."""
    if not HAS_CRYPTO:
        raise RuntimeError("cryptography library not installed")

    salt = bytes.fromhex(salt_hex)
    key = _derive_key(passphrase, salt)
    f = Fernet(key)
    try:
        decrypted = f.decrypt(encrypted_data.encode()).decode()
    except Exception:
        raise ValueError("Decryption failed — wrong passphrase or corrupted wallet")
    return decrypted


# ─── Save / Load ───

def save_wallet(wallet_data: dict, wallet_path, passphrase=None, insecure=False):
    """Save wallet to file.

    Args:
        wallet_data: dict with address, private_key, public_key_hash
        wallet_path: file path
        passphrase: encryption passphrase (None = try to get interactively)
        insecure: if True, save in plaintext v1 format (not recommended)

    Returns:
        Path to saved wallet file
    """
    wallet_path = Path(wallet_path).expanduser()
    wallet_path.parent.mkdir(parents=True, exist_ok=True)

    private_key = wallet_data["private_key"]

    if insecure:
        # v1 obfuscated format (not recommended)
        stored = {
            "version": 1,
            "address": wallet_data["address"],
            "private_key_encoded": _obfuscate_key(private_key),
            "public_key_hash": wallet_data["public_key_hash"],
            "_warning": "INSECURE: key is only obfuscated, not encrypted. Use --insecure to suppress.",
        }
    elif HAS_CRYPTO:
        # Get passphrase if not provided
        if passphrase is None:
            passphrase = _get_passphrase(confirm=True, purpose="new wallet")
        if passphrase is None:
            print("⚠️  No passphrase provided. Falling back to obfuscated (insecure) storage.")
            print("   To encrypt, set CLAWCHAIN_WALLET_PASSPHRASE or run interactively.")
            stored = {
                "version": 1,
                "address": wallet_data["address"],
                "private_key_encoded": _obfuscate_key(private_key),
                "public_key_hash": wallet_data["public_key_hash"],
                "_warning": "Key is only obfuscated. Re-run setup to encrypt with passphrase.",
            }
        else:
            enc = encrypt_private_key(private_key, passphrase)
            stored = {
                "version": WALLET_VERSION,
                "encrypted": True,
                "address": wallet_data["address"],
                "public_key_hash": wallet_data["public_key_hash"],
                "data": enc["data"],
                "kdf": enc["kdf"],
                "salt": enc["salt"],
                "_warning": "This is a mining/test wallet only. Do not store significant value.",
            }
    else:
        # No cryptography library — fall back to obfuscation
        print("⚠️  WARNING: `cryptography` library not installed. Private key stored with base64 obfuscation only.")
        print("   Install it for real encryption: pip install cryptography")
        stored = {
            "version": 1,
            "address": wallet_data["address"],
            "private_key_encoded": _obfuscate_key(private_key),
            "public_key_hash": wallet_data["public_key_hash"],
            "_warning": "Key is only obfuscated (cryptography lib not installed). pip install cryptography for encryption.",
        }

    with open(wallet_path, "w") as f:
        json.dump(stored, f, indent=2)
    os.chmod(wallet_path, 0o600)

    return wallet_path


def load_wallet(wallet_path, passphrase=None):
    """Load wallet, supporting v1 (obfuscated) and v2 (encrypted) formats.

    Also supports CLAWCHAIN_PRIVATE_KEY env var override.

    Returns:
        dict with address, private_key (hex), public_key_hash
    """
    wallet_path = Path(wallet_path).expanduser()
    with open(wallet_path) as f:
        data = json.load(f)

    address = data["address"]

    # Env var override (highest priority)
    env_key = os.getenv("CLAWCHAIN_PRIVATE_KEY")
    if env_key:
        print("🔑 Using private key from CLAWCHAIN_PRIVATE_KEY environment variable")
        return {"address": address, "private_key": env_key, "public_key_hash": data.get("public_key_hash", "")}

    version = data.get("version", 1)

    # v2 encrypted format
    if version >= 2 and data.get("encrypted"):
        if not HAS_CRYPTO:
            print("❌ Wallet is encrypted but `cryptography` library is not installed.")
            print("   Install it: pip install cryptography")
            sys.exit(1)

        if passphrase is None:
            passphrase = _get_passphrase(purpose="wallet decryption")
        if passphrase is None:
            print("❌ Encrypted wallet requires a passphrase.")
            print("   Set CLAWCHAIN_WALLET_PASSPHRASE or run interactively.")
            sys.exit(1)

        pk = decrypt_private_key(data["data"], data["salt"], passphrase)
        return {"address": address, "private_key": pk, "public_key_hash": data.get("public_key_hash", "")}

    # v1 obfuscated format
    if "private_key_encoded" in data:
        pk = _deobfuscate_key(data["private_key_encoded"])
        return {"address": address, "private_key": pk, "public_key_hash": data.get("public_key_hash", "")}

    # Very old format (plaintext)
    if "private_key" in data:
        return data

    raise ValueError("Wallet file has no recognizable private key field")


def detect_wallet_version(wallet_path) -> dict:
    """Detect wallet format version and encryption status.

    Returns:
        dict with version, encrypted, needs_migration
    """
    wallet_path = Path(wallet_path).expanduser()
    if not wallet_path.exists():
        return {"exists": False}

    with open(wallet_path) as f:
        data = json.load(f)

    version = data.get("version", 0)
    encrypted = data.get("encrypted", False)

    # No version field = v0 (very old plaintext)
    if version == 0 and "private_key" in data:
        return {"exists": True, "version": 0, "encrypted": False, "needs_migration": True, "format": "plaintext"}

    if version == 1:
        return {"exists": True, "version": 1, "encrypted": False, "needs_migration": True, "format": "obfuscated"}

    if version >= 2 and encrypted:
        return {"exists": True, "version": version, "encrypted": True, "needs_migration": False, "format": "encrypted"}

    return {"exists": True, "version": version, "encrypted": False, "needs_migration": True, "format": "unknown"}


def migrate_wallet(wallet_path, passphrase=None):
    """Migrate a v0/v1 wallet to v2 encrypted format.

    Returns True if migration succeeded.
    """
    wallet_path = Path(wallet_path).expanduser()
    info = detect_wallet_version(wallet_path)

    if not info.get("needs_migration"):
        print("ℹ️  Wallet is already v2 encrypted. No migration needed.")
        return True

    if not HAS_CRYPTO:
        print("⚠️  Cannot migrate: `cryptography` library not installed.")
        print("   Install it: pip install cryptography")
        return False

    # Load existing wallet
    wallet_data = load_wallet(wallet_path)

    # Get passphrase for encryption
    if passphrase is None:
        passphrase = _get_passphrase(confirm=True, purpose="wallet encryption (migration)")
    if passphrase is None:
        print("❌ Passphrase required for migration.")
        return False

    # Save in v2 format
    save_wallet(wallet_data, wallet_path, passphrase=passphrase)
    print(f"✅ Wallet migrated to v2 encrypted format: {wallet_path}")
    return True
