# ClawChain Security Model

This document describes ClawChain's security assumptions, threat model, and known limitations as of v0.1.0-testnet.

## 1. Trust Assumptions

### Miner → Mining Service
Miners trust the mining service (the HTTP API server) to:
- Generate challenges fairly and non-deterministically.
- Accept and store submissions honestly.
- Settle rewards accurately based on majority consensus or known answers.
- Not leak submitted answers to other miners before the reveal phase.

### Mining Service → Challenge Engine
The mining service trusts the challenge engine to:
- Generate diverse, solvable challenges across all 11 types.
- Produce correct `known_answer` values for spot-check challenges.
- Distribute challenges with appropriate difficulty tiers based on the miner population.

### Miners ↔ Miners (Indirect)
Miners do not communicate directly. Trust between miners is mediated by the consensus mechanism:
- In production mode, a challenge requires 3 independent submissions with majority agreement.
- In DEV mode (current testnet), a single miner can settle a challenge.

## 2. Attacker Model

### 2.1 Sybil Attacks
**Threat**: An attacker registers many fake miners to dominate reward distribution.

**Current mitigations**:
- Progressive staking (planned): free → 10 CLAW → 100 CLAW as network grows.
- Cool-start period: new miners receive only 50% rewards for the first 100 epochs.
- Registration index tracking: early miners get higher multipliers, making late Sybil accounts less profitable.

**Known gaps**: In testnet/DEV mode, there is no staking requirement. Sybil resistance relies on the mining service being the sole registrar.

### 2.2 Collusion
**Threat**: Multiple miners coordinate to submit identical wrong answers, gaming majority consensus.

**Current mitigations**:
- Spot checks (10% of challenges): Use known answers as ground truth. Wrong answers on spot checks incur -50 reputation penalty regardless of majority.
- Reputation system: Miners with reputation below 100 are suspended. Tier 2 challenges require reputation ≥ 600, Tier 3 requires ≥ 800.
- Consecutive failure detection: 5+ consecutive wrong answers → -500 reputation + suspension.

**Known gaps**: If colluding miners outnumber honest miners on non-spot-check challenges, they can establish a false majority. This is acceptable in testnet but will be addressed with signature-based assignment in mainnet.

### 2.3 Answer Stealing / Front-Running
**Threat**: A miner observes another miner's submitted answer and copies it.

**Current mitigations**:
- Commit-reveal protocol: Miners first submit a SHA256 hash of (answer + random nonce), then reveal the answer and nonce after a delay. The hash must match.
- The API does not expose raw answers until after the reveal phase.

**Known gaps**: In DEV mode, direct submission (bypassing commit-reveal) is supported for convenience. The `pending` challenges endpoint exposes submitted answers in the `reveals` field, which could be read by other miners.

## 3. Signing Boundaries

### Current (v0.1.0-testnet)
- **No cryptographic signature verification**: Submissions are identified by miner address string only.
- Wallet generation uses SHA256 + RIPEMD160 hash simulation (not real secp256k1 key derivation).
- The mining service trusts that the `miner_address` field in API requests corresponds to the actual wallet holder.

### Mainnet Plan
- Full secp256k1 signature verification on all submissions.
- Challenge assignments signed by the mining service.
- On-chain verification of submission signatures via the `x/challenge` Cosmos SDK module.

## 4. Commit-Reveal Security

### Protocol
1. **Commit phase**: Miner computes `commit_hash = SHA256(answer + nonce)` and submits it.
2. **Wait period**: Configurable delay (default: 3 seconds in testnet).
3. **Reveal phase**: Miner submits the plaintext answer and nonce. Server verifies `SHA256(answer + nonce) == commit_hash`.

### Anti-Copying Properties
- Miners cannot derive the answer from the commit hash (SHA256 is one-way).
- The random nonce prevents dictionary attacks on common answers.
- Each miner must independently compute their answer before the commit deadline.

### DEV Mode Simplification
- Direct submission (without commit-reveal) is allowed in DEV mode.
- Single-miner settlement is allowed (production requires 3 miners).
- These simplifications are acceptable for testing but reduce security guarantees.

## 5. Reputation and Spot-Check Fairness

### Reputation System
- New miners start at reputation 500 (out of 1000).
- Correct answers: +5 reputation (normal), +10 (spot check).
- Wrong answers: -20 reputation (normal), -50 (spot check).
- Reputation < 100 → miner suspended for 24 hours.
- 5+ consecutive failures → -500 reputation + immediate suspension.
- Suspended miners can re-register after 24h cooldown with reputation reset to 200.

### Spot Check Design
- 10% of challenges per epoch are spot checks (challenges with known answers).
- Spot check challenges are indistinguishable from normal challenges to miners.
- Wrong answers on spot checks carry heavier penalties to deter random/garbage submissions.
- The `known_answer` field is used server-side for verification and is not exposed in the API response to miners.

### Fairness Considerations
- Challenge distribution is open (any active miner can attempt any pending challenge).
- Reward per challenge is split equally among correct miners.
- Tier-based access (reputation gates) prevents low-quality miners from attempting high-value challenges.

## 6. Known Limitations

### Plaintext API
- All API communication is over HTTP/HTTPS without client authentication (beyond address matching).
- No mutual TLS or API key authentication for miners.
- HTTPS is recommended but not enforced. The mining scripts warn on non-localhost HTTP URLs.

### No TLS Certificate Pinning
- The mining client (`requests` library) uses system CA store for HTTPS verification.
- No certificate pinning is implemented, making the system vulnerable to MITM attacks with compromised CAs.

### Single-Server Architecture
- The mining service runs as a single HTTP server with SQLite storage.
- No distributed consensus among mining service instances.
- Single point of failure and single point of trust.

### Wallet Security
- Private keys are stored locally with base64 obfuscation (not encryption) and 600 file permissions.
- This protects against casual exposure but not against targeted attacks with file system access.
- Environment variable (`CLAWCHAIN_PRIVATE_KEY`) support allows external secret management.
- **Testnet wallets should never hold significant value.**

### LLM Provider Trust
- In `auto` and `llm` solver modes, challenge prompt text is sent to external LLM providers (OpenAI, Google, Anthropic).
- Challenge prompts may contain the full text of the challenge, which could theoretically be logged by the provider.
- The `local_only` solver mode avoids this by only using local computation.

---

*Last updated: 2026-03-20 (v0.1.0-testnet)*
