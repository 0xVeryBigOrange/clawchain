# ClawChain Production Deployment Baseline

## Minimum Viable Production Architecture

### Required Infrastructure

| Component | Count | Purpose |
|-----------|:-----:|---------|
| Validator nodes | ≥3 | Consensus (2/3 fault tolerance requires 3+) |
| Sentry nodes | ≥2 | Public RPC/API endpoints, DDoS protection |
| Mining service | 1 | Off-chain challenge generation + caching (non-authoritative) |
| Monitoring | 1 | Prometheus + Grafana or equivalent |

### Validator Node Specification

- **OS**: Ubuntu 22.04 LTS or equivalent
- **CPU**: 4+ cores
- **RAM**: 16 GB minimum
- **Storage**: 500 GB NVMe SSD (chain data grows ~1 GB/month estimated)
- **Network**: 100 Mbps dedicated, static IP
- **Ports**: 26656 (P2P), 26657 (RPC, restricted), 9090 (gRPC, restricted)

### Deployment Topology

```
                    ┌─────────────┐
   Miners ─────────►│  Sentry 1   │◄──── Public RPC/API
                    │  (full node) │
                    └──────┬──────┘
                           │ P2P
         ┌─────────────────┼─────────────────┐
         │                 │                 │
    ┌────▼────┐      ┌────▼────┐      ┌────▼────┐
    │ Val 1   │◄────►│ Val 2   │◄────►│ Val 3   │
    │(private)│      │(private)│      │(private)│
    └─────────┘      └─────────┘      └─────────┘
```

### Key Management

- Validator keys stored in encrypted keyring (never on sentry nodes)
- Operator keys separate from validator consensus keys
- KMS integration recommended for production (tmkms or similar)

### Monitoring & Alerting

Required metrics:
- Block height progression (stall detection)
- Validator missed blocks (>10 = alert)
- P2P peer count (< 2 = alert)
- Disk usage
- Memory/CPU utilization
- gRPC query latency

### Backup & Recovery

- **State sync snapshots**: every 1000 blocks
- **Genesis backup**: stored off-machine
- **Validator key backup**: encrypted, stored separately
- **Recovery procedure**: documented, tested quarterly

### Restart Policy

- systemd service with `Restart=always`
- `RestartSec=5`
- Automatic state sync on major version changes
- Manual intervention for chain halt scenarios

## Current State vs Production Baseline

| Requirement | Current | Production |
|-------------|:-------:|:----------:|
| Validator count | 1 | ≥3 |
| Sentry nodes | 0 | ≥2 |
| DDoS protection | None | Sentry architecture |
| Monitoring | None | Prometheus + alerts |
| Key management | File-based test keyring | KMS or encrypted |
| Backups | None | Automated snapshots |
| Recovery plan | None | Documented + tested |
| Restart policy | Manual nohup | systemd service |

## Multi-Validator Setup

### Adding New Validators

```bash
# On new validator machine:
clawchaind init validator-2 --chain-id clawchain-mainnet-1
clawchaind keys add validator-2
# Transfer stake tokens to the new validator address
# Create validator tx:
clawchaind tx staking create-validator \
  --amount=10000000uclaw \
  --pubkey=$(clawchaind tendermint show-validator) \
  --moniker="validator-2" \
  --commission-rate="0.10" \
  --commission-max-rate="0.20" \
  --commission-max-change-rate="0.01" \
  --min-self-delegation="1" \
  --from=validator-2 \
  --chain-id=clawchain-mainnet-1
```

### Validator Set Requirements

- **Minimum for formal launch**: 3 validators (tolerate 1 failure)
- **Recommended**: 5-7 validators
- **Max for initial launch**: 21 (manageable coordination)

## Migration Path

1. **Current** → **Pre-launch testnet**: Add 2 more validator nodes
2. **Pre-launch testnet** → **Genesis launch**: Fresh genesis with 3+ validators, distribute genesis stake
3. **Post-launch**: Add validators through governance / staking
