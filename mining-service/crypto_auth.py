"""
ClawChain Miner Identity — secp256k1 Signature Verification
============================================================

Specification (canonical, authoritative):

ADDRESS DERIVATION
  1. Generate a random 32-byte secp256k1 private key
  2. Derive the uncompressed public key (64 bytes, no 04 prefix)
  3. Compute compressed public key (33 bytes, 02/03 prefix)
  4. address_bytes = RIPEMD160(SHA256(compressed_pubkey))
  5. address = bech32_encode("claw", address_bytes)

  This matches the standard Cosmos SDK / Tendermint address derivation.
  The server verifies this binding at registration time.

CANONICAL SIGNING PAYLOAD
  Fields are joined with the ASCII pipe character "|" in this exact order:
    challenge_id | answer | miner_address | nonce

  - challenge_id: UTF-8 string (e.g. "ch-42-0")
  - answer: UTF-8 string (the miner's answer, verbatim as submitted)
  - miner_address: UTF-8 string (bech32 claw1... address)
  - nonce: decimal integer as string (no leading zeros except "0" itself)

  The concatenated string is encoded as UTF-8 bytes, then:
    msg_hash = SHA256(utf8_bytes)

  The miner signs msg_hash with secp256k1_sign (recoverable signature).

SIGNATURE FORMAT
  65-byte recoverable secp256k1 signature:
    - bytes [0:32]  = r
    - bytes [32:64] = s
    - byte  [64]    = v (recovery id, 0 or 1)
  Transmitted as lowercase hex string, optional "0x" prefix.

REPLAY PROTECTION
  - nonce: a strictly increasing integer per miner (millisecond Unix timestamp recommended)
  - The server stores last_nonce per miner in durable SQLite storage (survives restart)
  - Rules:
      nonce <= last_nonce           → rejected (replay)
      nonce > server_time_ms + 300000 → rejected (too far in future, max 5 min)
  - last_nonce is updated atomically within the same DB transaction as submission insert
  - Concurrent submissions from the same miner are serialized by SQLite row-level locking
  - On server restart, last_nonce persists (stored in miners table)

VERIFICATION FLOW (server-side)
  1. Parse signature (65 bytes from hex)
  2. Reconstruct msg_hash = SHA256(challenge_id|answer|miner_address|nonce)
  3. Recover public key from (msg_hash, signature)
  4. Compare recovered pubkey to registered pubkey → reject if mismatch
  5. Check nonce > last_nonce and nonce <= now_ms + 300000 → reject if violated
  6. Atomically: insert submission + update last_nonce → commit

FAILURE RESPONSES
  - Missing signature (pubkey miner): 403 "signature required: miner has registered public key"
  - Bad signature format:             403 "signature must be 65 bytes, got N"
  - Signature/pubkey mismatch:        403 "signature does not match registered public key"
  - Address/pubkey mismatch:          400 "address does not match public key (derivation mismatch)"
  - Replayed nonce:                   403 "nonce N is not greater than last nonce M (replay rejected)"
  - Future nonce:                     403 "nonce too far in future (max 5 min ahead)"
"""

import hashlib
import logging
import time

from eth_keys import keys as eth_keys

logger = logging.getLogger("clawchain.crypto_auth")


# ═══════════════════════════════════════════════
# Address derivation (Cosmos SDK compatible)
# ═══════════════════════════════════════════════

def pubkey_to_compressed(pubkey_hex: str) -> bytes:
    """Convert 64-byte uncompressed public key to 33-byte compressed form.

    Args:
        pubkey_hex: 128-char hex string (64 bytes, no 04 prefix)

    Returns:
        33-byte compressed public key (02/03 prefix)
    """
    pub_bytes = bytes.fromhex(pubkey_hex.removeprefix("0x"))
    if len(pub_bytes) != 64:
        raise ValueError(f"expected 64-byte uncompressed key, got {len(pub_bytes)}")
    x = pub_bytes[:32]
    y = pub_bytes[32:]
    prefix = b'\x02' if y[-1] % 2 == 0 else b'\x03'
    return prefix + x


def derive_address_from_pubkey(pubkey_hex: str) -> str:
    """Derive a claw1... bech32 address from a 64-byte uncompressed public key.

    Derivation: bech32("claw", RIPEMD160(SHA256(compressed_pubkey)))

    This matches standard Cosmos SDK address derivation.
    """
    compressed = pubkey_to_compressed(pubkey_hex)
    sha = hashlib.sha256(compressed).digest()
    ripemd = hashlib.new("ripemd160", sha).digest()
    return _bech32_encode("claw", ripemd)


def verify_address_pubkey_binding(address: str, pubkey_hex: str) -> tuple[bool, str]:
    """Verify that a claw1... address is correctly derived from the given public key.

    Returns:
        (valid: bool, error_message: str)
    """
    try:
        expected_address = derive_address_from_pubkey(pubkey_hex)
        if address != expected_address:
            return False, (
                f"address does not match public key: "
                f"expected {expected_address}, got {address}"
            )
        return True, ""
    except Exception as e:
        return False, f"address/pubkey verification failed: {e}"


# ═══════════════════════════════════════════════
# Canonical signing payload
# ═══════════════════════════════════════════════

def build_sign_payload(challenge_id: str, answer: str, miner_address: str, nonce: int) -> bytes:
    """Build the canonical 32-byte message hash for signing.

    Canonical format:
        SHA256("{challenge_id}|{answer}|{miner_address}|{nonce}".encode("utf-8"))

    Field order is fixed: challenge_id, answer, miner_address, nonce.
    Separator is the ASCII pipe "|" (0x7C).
    Nonce is formatted as a decimal integer string with no leading zeros.
    """
    payload = f"{challenge_id}|{answer}|{miner_address}|{nonce}"
    return hashlib.sha256(payload.encode("utf-8")).digest()


# ═══════════════════════════════════════════════
# Signature verification
# ═══════════════════════════════════════════════

def verify_signature(
    challenge_id: str,
    answer: str,
    miner_address: str,
    nonce: int,
    signature_hex: str,
    expected_pubkey_hex: str,
) -> tuple[bool, str]:
    """Verify a secp256k1 signature against the expected public key.

    Returns:
        (valid: bool, error_message: str)
    """
    try:
        msg_hash = build_sign_payload(challenge_id, answer, miner_address, nonce)

        sig_hex = signature_hex.removeprefix("0x")
        try:
            sig_bytes = bytes.fromhex(sig_hex)
        except ValueError:
            return False, "signature is not valid hex"

        if len(sig_bytes) != 65:
            return False, f"signature must be 65 bytes, got {len(sig_bytes)}"

        sig = eth_keys.Signature(sig_bytes)
        recovered_pub = sig.recover_public_key_from_msg_hash(msg_hash)

        expected_pub_hex = expected_pubkey_hex.removeprefix("0x")
        try:
            expected_pub = eth_keys.PublicKey(bytes.fromhex(expected_pub_hex))
        except Exception:
            return False, "registered public key is invalid"

        if recovered_pub != expected_pub:
            return False, "signature does not match registered public key"

        return True, ""

    except Exception as e:
        logger.warning(f"Signature verification error: {e}")
        return False, f"signature verification failed: {str(e)}"


# ═══════════════════════════════════════════════
# Replay protection
# ═══════════════════════════════════════════════

def check_nonce(db, miner_address: str, nonce: int) -> tuple[bool, str]:
    """Check nonce for replay protection.

    Rules:
      - nonce must be > last_nonce (strictly increasing)
      - nonce must be <= current_time_ms + 300_000 (max 5 min future)
      - last_nonce is persisted in SQLite (survives server restart)
    """
    row = db.execute(
        "SELECT last_nonce FROM miners WHERE address=?", (miner_address,)
    ).fetchone()

    if row is None:
        return False, "miner not found"

    last_nonce = row["last_nonce"] if row["last_nonce"] is not None else 0

    if nonce <= last_nonce:
        return False, (
            f"nonce {nonce} is not greater than last nonce {last_nonce} "
            f"(replay rejected)"
        )

    now_ms = int(time.time() * 1000)
    if nonce > now_ms + 300_000:
        return False, "nonce too far in future (max 5 min ahead)"

    return True, ""


def update_nonce(db, miner_address: str, nonce: int):
    """Update the last seen nonce for a miner.

    This MUST be called within the same transaction as the submission insert
    to ensure atomicity. The caller is responsible for calling db.commit().
    """
    db.execute(
        "UPDATE miners SET last_nonce=? WHERE address=?",
        (nonce, miner_address),
    )
    # NOTE: do NOT commit here — caller commits after submission insert


# ═══════════════════════════════════════════════
# Bech32 encoding (Cosmos SDK compatible)
# ═══════════════════════════════════════════════

_BECH32_CHARSET = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"


def _bech32_polymod(values):
    GEN = [0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3]
    chk = 1
    for v in values:
        b = chk >> 25
        chk = ((chk & 0x1FFFFFF) << 5) ^ v
        for i in range(5):
            chk ^= GEN[i] if ((b >> i) & 1) else 0
    return chk


def _bech32_hrp_expand(hrp):
    return [ord(x) >> 5 for x in hrp] + [0] + [ord(x) & 31 for x in hrp]


def _bech32_create_checksum(hrp, data):
    values = _bech32_hrp_expand(hrp) + data
    polymod = _bech32_polymod(values + [0, 0, 0, 0, 0, 0]) ^ 1
    return [(polymod >> 5 * (5 - i)) & 31 for i in range(6)]


def _convertbits(data, frombits, tobits, pad=True):
    acc = 0
    bits = 0
    ret = []
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


def _bech32_encode(hrp: str, data_bytes: bytes) -> str:
    """Encode raw bytes as a bech32 address with the given HRP."""
    data5 = _convertbits(list(data_bytes), 8, 5)
    combined = data5 + _bech32_create_checksum(hrp, data5)
    return hrp + "1" + "".join([_BECH32_CHARSET[d] for d in combined])
