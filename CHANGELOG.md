# Changelog

All notable changes to ClawChain will be documented in this file.

## [v0.2.0] - 2026-03-20

### Added
- Independent mining service (Python/SQLite) — fully decoupled from chain binary
- Mining skill for OpenClaw agents (`skill/scripts/`)
- 11 challenge types: math, hash, sentiment, classification, translation, text_summary, entity_extraction, logic, text_transform, json_extract, format_convert
- Commit-reveal two-phase submission protocol
- Reputation system with spot-check verification (10% known-answer challenges)
- Reward calculation with early-bird multiplier (3x for first 1,000 miners)
- Streak bonuses (7d +10%, 30d +25%, 90d +50%)
- Epoch scheduler (10-minute epochs, 50 CLAW per epoch)
- Landing page (Next.js 14) with EN/ZH bilingual support
- Cosmos SDK chain skeleton (x/poa, x/challenge, x/reputation modules)
- `doctor.py` — pre-flight diagnostic tool
- Safe math evaluator (AST-based, replaces `eval()`)
- Wallet key obfuscation (base64 encoding at rest)
- `solver_mode` config: `local_only` / `llm` / `auto`
- HTTPS RPC endpoint with insecure-HTTP warnings
- Backward compatibility for `node_url` → `rpc_url` config migration

### Security
- Removed `eval()` from math solver — replaced with AST-based safe evaluator
- Private keys stored with obfuscation + file permissions 600
- Support for `CLAWCHAIN_PRIVATE_KEY` environment variable
- HTTPS warnings for non-localhost HTTP endpoints
- Security model documentation (`docs/security-model.md`)

### Documentation
- Whitepaper (EN + ZH)
- Product spec (EN + ZH)
- Mining design document
- Setup guide (SETUP.md)
- This changelog

## [Unreleased]
- Public testnet endpoint
- secp256k1 signature verification
- Full Cosmos SDK module implementation
- Task Marketplace
l Cosmos SDK module implementation
- Task Marketplace
