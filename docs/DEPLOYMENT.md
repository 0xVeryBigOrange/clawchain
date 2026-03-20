# ClawChain Deployment Guide — Public Alpha

## Architecture Overview

ClawChain Public Alpha consists of two independent components:

| Component | Hosts | Purpose |
|-----------|-------|---------|
| **Website** | GitHub Pages | Static marketing site, install guide, tokenomics |
| **Mining Service** | Your server (Mac mini, VPS, etc.) | Challenge generation, answer verification, reward settlement |

> ⚠️ GitHub Pages only hosts the website. Mining requires a running backend service.

## Website Deployment

The website is a static Next.js export deployed to GitHub Pages:

```bash
cd website && npm run build
git push origin $(git subtree split --prefix website/out main):gh-pages --force
```

Live at: https://0xverybigorange.github.io/clawchain/

## Mining Service Deployment

### Minimum Requirements
- Python 3.9+
- SQLite (included with Python)
- ~100MB RAM
- Stable internet connection
- Open port for HTTPS (via Cloudflare Tunnel or reverse proxy)

### Recommended Setups

| Scale | Setup | Notes |
|-------|-------|-------|
| **Dev/Testing** | Mac mini + Cloudflare quick tunnel | Current testnet setup. URL changes on restart. |
| **Internal Alpha** | Mac mini + Cloudflare named tunnel | Stable URL, free, good for <100 miners |
| **Public Alpha** | VPS ($5-10/mo) + own domain | DigitalOcean/Hetzner/Vultr, stable HTTPS |
| **Production** | Dedicated server + load balancer | For mainnet launch |

### Starting the Service

```bash
cd mining-service
python3 server.py  # Starts on port 1317
```

### With Cloudflare Tunnel (current)

```bash
# Quick tunnel (URL changes on restart)
cloudflared tunnel --url http://localhost:1317

# Or use the auto-update script
bash scripts/start_tunnel.sh
```

### With LaunchAgent (macOS auto-start)

```bash
# Mining service
launchctl load ~/Library/LaunchAgents/com.clawchain.mining-service.plist

# Tunnel
launchctl load ~/Library/LaunchAgents/com.clawchain.tunnel.plist
```

## Public Alpha Limitations

- Single server architecture (no P2P replication)
- RPC endpoint may change during alpha
- Testnet may reset — mining history could be cleared
- SQLite database (not distributed ledger)

## Monitoring

```bash
# Check service health
curl http://localhost:1317/clawchain/stats

# Check version
curl http://localhost:1317/clawchain/version

# Check tunnel URL
cat /tmp/clawchain-tunnel-url.txt
```

## Migration to Production

For mainnet launch, plan to:
1. Replace SQLite with on-chain state (Cosmos SDK modules)
2. Replace single server with validator network
3. Replace Cloudflare tunnel with dedicated domain + TLS
4. Add P2P challenge distribution
5. Implement majority-vote verification for non-deterministic tasks
