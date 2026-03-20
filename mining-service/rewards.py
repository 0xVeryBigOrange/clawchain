"""
ClawChain Mining Service — 奖励计算
参数与链代码（keeper.go）完全一致。
"""

# ─── 核心常量（白皮书 & 链代码一致）───

EPOCH_TOTAL_REWARD = 50_000_000       # 50 CLAW per epoch (uclaw)
MINER_POOL_RATIO = 1.00              # 100% fair launch → all to miners
VALIDATOR_POOL_RATIO = 0.00          # 0% (fair launch)
ECO_FUND_RATIO = 0.00                # 0% (fair launch)

INITIAL_MINER_POOL = 50_000_000      # 50 CLAW (uclaw) — 100% fair launch
INITIAL_VALIDATOR_POOL = 0           # 0 (fair launch)
INITIAL_ECO_FUND = 0                 # 0 (fair launch)

HALVING_EPOCHS = 210_000
MIN_REWARD = 1  # 最低 1 uclaw


def get_epoch_miner_pool(epoch: int) -> int:
    """
    获取指定 epoch 的矿工池奖励（带减半）。
    与 Go keeper.GetBlockReward 一致。
    """
    halvings = epoch // HALVING_EPOCHS
    reward = INITIAL_MINER_POOL
    for _ in range(halvings):
        reward //= 2
        if reward < MIN_REWARD:
            reward = MIN_REWARD
            break
    return reward


def get_epoch_validator_pool(epoch: int) -> int:
    """验证者池（带减半）"""
    halvings = epoch // HALVING_EPOCHS
    reward = INITIAL_VALIDATOR_POOL
    for _ in range(halvings):
        reward //= 2
        if reward < MIN_REWARD:
            reward = MIN_REWARD
            break
    return reward


def get_epoch_eco_fund(epoch: int) -> int:
    """生态基金（带减半）"""
    halvings = epoch // HALVING_EPOCHS
    reward = INITIAL_ECO_FUND
    for _ in range(halvings):
        reward //= 2
        if reward < MIN_REWARD:
            reward = MIN_REWARD
            break
    return reward


def calc_early_bird_multiplier(registration_index: int) -> int:
    """
    早鸟倍率（百分比，100 = 1x）。
    前1000人 3x, 前5000人 2x, 前10000人 1.5x, 其余 1x。
    """
    if registration_index <= 0:
        return 100
    if registration_index <= 1000:
        return 300
    elif registration_index <= 5000:
        return 200
    elif registration_index <= 10000:
        return 150
    return 100


def calc_streak_multiplier(consecutive_days: int) -> int:
    """
    签到倍率（百分比，100 = 1x）。
    7天+ → 110, 30天+ → 125, 90天+ → 150。
    """
    if consecutive_days >= 90:
        return 150
    elif consecutive_days >= 30:
        return 125
    elif consecutive_days >= 7:
        return 110
    return 100


def calculate_miner_reward(
    base_reward: int,
    registration_index: int,
    consecutive_days: int,
    challenges_completed: int,
) -> int:
    """
    计算单个矿工的实际奖励（含早鸟/签到/冷启动）。
    与 Go applyBonusMultipliers 完全一致。

    actual = base * earlyBird * streak / 10000
    冷启动: 前100次完成减半
    """
    early_bird = calc_early_bird_multiplier(registration_index)
    streak = calc_streak_multiplier(consecutive_days)

    actual = base_reward * early_bird * streak // 10000

    # 冷启动: 前100次减半
    if challenges_completed < 100:
        actual //= 2

    return max(actual, MIN_REWARD)


def settle_challenge(
    epoch: int,
    num_challenges_in_epoch: int,
    correct_miners: list,
    miner_info_map: dict,
) -> dict:
    """
    结算一道挑战的奖励。

    参数:
        epoch: 当前 epoch 号
        num_challenges_in_epoch: 本 epoch 的总挑战数
        correct_miners: 答对的矿工地址列表
        miner_info_map: {addr: {"registration_index": int, "consecutive_days": int, "challenges_completed": int}}

    返回:
        {addr: reward_amount} 每个矿工的奖励
    """
    if not correct_miners or num_challenges_in_epoch < 1:
        return {}

    miner_pool = get_epoch_miner_pool(epoch)
    per_challenge_pool = max(miner_pool // num_challenges_in_epoch, 1)
    reward_per_miner = max(per_challenge_pool // len(correct_miners), 1)

    rewards = {}
    for addr in correct_miners:
        info = miner_info_map.get(addr, {})
        actual = calculate_miner_reward(
            reward_per_miner,
            info.get("registration_index", 0),
            info.get("consecutive_days", 0),
            info.get("challenges_completed", 0),
        )
        rewards[addr] = actual

    return rewards
