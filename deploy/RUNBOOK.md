# ClawChain 3-Validator Testnet Deployment Runbook

---

## 部署前 Checklist

逐项确认后打勾：

- [ ] 3 台 VPS 已开通，OS 为 Ubuntu 22.04
- [ ] 3 台 VPS 有 static IP，已记录
- [ ] 每台 VPS 已安装 Go 1.22+（或你会从 Mac 交叉编译）
- [ ] 每台 VPS 的 26656 端口已开放（P2P）
- [ ] val1 VPS 的 26657（RPC）和 9090（gRPC）端口已开放
- [ ] 本机 `init-testnet.sh` 已跑通，`deploy/testnet-artifacts/` 目录存在
- [ ] `persistent_peers.txt` 中的 `.example.com` 已替换为真实 VPS IP
- [ ] 每台 VPS 有 SSH 访问权限
- [ ] 本机有 `clawchaind` 的 linux/amd64 编译产物（或计划在 VPS 上编译）

---

## 第 0 步：本机准备（你的 Mac）

### 0.1 生成 testnet artifacts（如果还没跑过）

```bash
cd /path/to/clawchain
bash deploy/init-testnet.sh
```

输出：`deploy/testnet-artifacts/{val1,val2,val3}` + `persistent_peers.txt`

### 0.2 交叉编译 linux/amd64 binary

```bash
cd chain
GOOS=linux GOARCH=amd64 go build -mod=vendor -o ../build/clawchaind-linux ./cmd/clawchaind
```

验证：`file ../build/clawchaind-linux` 应显示 `ELF 64-bit LSB executable, x86-64`

### 0.3 替换 persistent_peers IP

编辑 `deploy/testnet-artifacts/persistent_peers.txt`，把 `val1.example.com` 等替换为真实 IP：

```
<val1-node-id>@<VAL1_IP>:26656,<val2-node-id>@<VAL2_IP>:26656,<val3-node-id>@<VAL3_IP>:26656
```

---

## 第 1 步：部署 val1

### 1.1 上传文件到 val1 VPS

```bash
VAL1_IP="<your-val1-ip>"

scp build/clawchaind-linux root@$VAL1_IP:/usr/local/bin/clawchaind
ssh root@$VAL1_IP "chmod +x /usr/local/bin/clawchaind"

scp -r deploy/testnet-artifacts/val1/* root@$VAL1_IP:~/.clawchain/
scp deploy/deploy-node.sh root@$VAL1_IP:~/
scp deploy/health-check.sh root@$VAL1_IP:~/
```

### 1.2 配置并启动

```bash
ssh root@$VAL1_IP

# 读取 persistent_peers（你在本机已替换好的完整字符串）
PEERS="<完整persistent_peers字符串>"

bash deploy-node.sh val1 "$PEERS"
systemctl start clawchain
systemctl status clawchain
```

### 1.3 验证 val1

```bash
# 在 val1 上
journalctl -u clawchain --no-pager | tail -5
curl localhost:26657/status | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['result']['sync_info']['latest_block_height'])"
```

期望：高度在增长（val1 单节点先出块）

---

## 第 2 步：部署 val2

### 2.1 上传

```bash
VAL2_IP="<your-val2-ip>"

scp build/clawchaind-linux root@$VAL2_IP:/usr/local/bin/clawchaind
ssh root@$VAL2_IP "chmod +x /usr/local/bin/clawchaind"

scp -r deploy/testnet-artifacts/val2/* root@$VAL2_IP:~/.clawchain/
scp deploy/deploy-node.sh root@$VAL2_IP:~/
scp deploy/health-check.sh root@$VAL2_IP:~/
```

### 2.2 配置并启动

```bash
ssh root@$VAL2_IP
PEERS="<同样的persistent_peers字符串>"
bash deploy-node.sh val2 "$PEERS"
systemctl start clawchain
```

### 2.3 验证 val2 连接 val1

```bash
# 在 val2 上
journalctl -u clawchain --no-pager | tail -5
# 应该看到 "Connected to peer" 或 "Peer connected"

# 在 val1 上
curl localhost:26657/net_info | python3 -c "import sys,json; d=json.load(sys.stdin); print(f'peers: {d[\"result\"][\"n_peers\"]}')"
```

期望：val1 显示 `peers: 1`

---

## 第 3 步：部署 val3

完全同 val2，替换 `val2` → `val3`，IP 替换。

### 3.1 上传

```bash
VAL3_IP="<your-val3-ip>"
scp build/clawchaind-linux root@$VAL3_IP:/usr/local/bin/clawchaind
ssh root@$VAL3_IP "chmod +x /usr/local/bin/clawchaind"
scp -r deploy/testnet-artifacts/val3/* root@$VAL3_IP:~/.clawchain/
scp deploy/deploy-node.sh root@$VAL3_IP:~/
scp deploy/health-check.sh root@$VAL3_IP:~/
```

### 3.2 启动

```bash
ssh root@$VAL3_IP
PEERS="<同样的persistent_peers字符串>"
bash deploy-node.sh val3 "$PEERS"
systemctl start clawchain
```

### 3.3 验证 3 节点网络

```bash
# 在 val1 上
curl localhost:26657/net_info | python3 -c "import sys,json; d=json.load(sys.stdin); print(f'peers: {d[\"result\"][\"n_peers\"]}')"
# 期望：peers: 2

curl localhost:26657/validators | python3 -c "import sys,json; d=json.load(sys.stdin); print(f'validators: {len(d[\"result\"][\"validators\"])}')"
# 期望：validators: 3
```

---

## 第 4 步：连接 Miner

在矿工机器上（可以是你的 Mac 或第 4 台 VPS）：

```bash
# 编译 clawminer（如果本机 Mac）
cd miner && go build -o ../build/clawminer ./cmd/clawminer

# 创建 miner key
./build/clawchaind keys add miner1 --keyring-backend test --keyring-dir ~/.clawminer

# 记录 miner 地址（需要从 genesis 账户转 uclaw 过来，或者重新生成 genesis 包含 miner）
MINER_ADDR=$(./build/clawchaind keys show miner1 --keyring-backend test --keyring-dir ~/.clawminer --address)
echo "Miner: $MINER_ADDR"

# 注册 miner
./build/clawminer register \
  --node tcp://$VAL1_IP:26657 \
  --chain-binary ./build/clawchaind \
  --keyring-dir ~/.clawminer \
  --key miner1 \
  --chain-id clawchain-testnet-1

# 启动挖矿
./build/clawminer start \
  --node tcp://$VAL1_IP:26657 \
  --chain-binary ./build/clawchaind \
  --keyring-dir ~/.clawminer \
  --key miner1 \
  --chain-id clawchain-testnet-1
```

---

## 部署后 30 分钟验收命令

在 val1 VPS 上运行：

```bash
# 1. 高度
curl -s localhost:26657/status | python3 -c "import sys,json; d=json.load(sys.stdin); print(f'height: {d[\"result\"][\"sync_info\"][\"latest_block_height\"]}')"

# 2. Peers
curl -s localhost:26657/net_info | python3 -c "import sys,json; d=json.load(sys.stdin); print(f'peers: {d[\"result\"][\"n_peers\"]}')"

# 3. Validators
curl -s localhost:26657/validators | python3 -c "import sys,json; d=json.load(sys.stdin); vs=d['result']['validators']; print(f'validators: {len(vs)}')"

# 4. 日志（每个 VPS）
journalctl -u clawchain --no-pager -n 10

# 5. 健康检查
bash health-check.sh http://localhost:26657

# 6. Miner 连接验证（从矿工机器）
./build/clawminer status --node tcp://$VAL1_IP:26657 --chain-binary ./build/clawchaind --keyring-dir ~/.clawminer --key miner1
```

期望结果：
- height > 0 且持续增长
- peers: 2
- validators: 3
- 日志无 panic/error
- health-check 返回 ✅
- miner 能看到网络高度

---

## 故障排查

### 节点起不来

```bash
journalctl -u clawchain --no-pager -n 50
# 看最后的 panic 或 error
```

常见原因：
1. `genesis.json` 不一致 → 对比 3 个节点的 genesis hash
2. `priv_validator_key.json` 和 gentx 不匹配 → 确认 val1 用 val1 的 key
3. binary 版本不一致 → `clawchaind version` 对比

### 不连 peers

```bash
curl localhost:26657/net_info | python3 -m json.tool | grep n_peers
```

原因：
1. 防火墙 26656 没开 → `ufw allow 26656/tcp`
2. `persistent_peers` IP 错误 → 检查 config.toml
3. `external_address` 未设置 → deploy-node.sh 应该自动设

### 高度不增长

```bash
curl localhost:26657/consensus_state | python3 -m json.tool | head -20
```

原因：
1. 少于 2/3 validator 在线 → 检查另外两个节点
2. 时间不同步 → `timedatectl status` 确认 NTP 同步

### Miner 连不上

原因：
1. val1 的 26657 端口未开放 → `ufw allow 26657/tcp`
2. RPC 绑定 localhost → 检查 config.toml `laddr = "tcp://0.0.0.0:26657"`

---

## 高风险核对项

| # | 风险项 | 核对方法 |
|---|--------|---------|
| 1 | genesis 一致性 | `md5sum ~/.clawchain/config/genesis.json` 3 台必须相同 |
| 2 | key 不错位 | val1 机器上是 val1 的 priv_validator_key，不是 val2 的 |
| 3 | persistent_peers IP | 3 个 IP 都正确，node_id 和 IP 对应 |
| 4 | 端口开放 | `nc -zv <IP> 26656` 从外部测试 |
| 5 | systemd 权限 | service 里的 User 和 Home 路径正确 |
| 6 | 磁盘空间 | `df -h` 至少 20GB 可用 |
| 7 | 时间同步 | `timedatectl` 3 台都 NTP synchronized |
