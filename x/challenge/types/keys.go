package types

const (
	// ModuleName 模块名称
	ModuleName = "challenge"

	// StoreKey 主存储 key
	StoreKey = ModuleName

	// RouterKey 消息路由 key
	RouterKey = ModuleName

	// MemStoreKey 内存存储 key
	MemStoreKey = "mem_challenge"
)

// =====================================
// KV Store 前缀 — 用于区分不同数据类型
// =====================================

var (
	// ChallengeKeyPrefix 挑战存储前缀: 0x01 + challengeID -> Challenge
	ChallengeKeyPrefix = []byte{0x01}

	// CommitKeyPrefix commit 存储前缀: 0x02 + challengeID + minerAddr -> ChallengeCommit
	CommitKeyPrefix = []byte{0x02}

	// RevealKeyPrefix reveal 存储前缀: 0x03 + challengeID + minerAddr -> ChallengeReveal
	RevealKeyPrefix = []byte{0x03}

	// MinerRecordKeyPrefix 矿工记录前缀: 0x04 + minerAddr -> MinerChallengeRecord
	MinerRecordKeyPrefix = []byte{0x04}

	// EpochChallengeKeyPrefix epoch 挑战索引前缀: 0x05 + epoch -> []challengeID
	EpochChallengeKeyPrefix = []byte{0x05}

	// ActiveChallengeKeyPrefix 活跃挑战前缀: 0x06 + challengeID -> empty (索引)
	ActiveChallengeKeyPrefix = []byte{0x06}

	// ParamsKey 模块参数 key
	ParamsKey = []byte{0x07}
)

// =====================================
// Key 构造函数
// =====================================

// ChallengeKey 返回挑战的完整存储 key
func ChallengeKey(challengeID string) []byte {
	return append(ChallengeKeyPrefix, []byte(challengeID)...)
}

// CommitKey 返回 commit 的完整存储 key
func CommitKey(challengeID, minerAddr string) []byte {
	key := append(CommitKeyPrefix, []byte(challengeID)...)
	key = append(key, byte('/'))
	key = append(key, []byte(minerAddr)...)
	return key
}

// RevealKey 返回 reveal 的完整存储 key
func RevealKey(challengeID, minerAddr string) []byte {
	key := append(RevealKeyPrefix, []byte(challengeID)...)
	key = append(key, byte('/'))
	key = append(key, []byte(minerAddr)...)
	return key
}

// MinerRecordKey 返回矿工记录的完整存储 key
func MinerRecordKey(minerAddr string) []byte {
	return append(MinerRecordKeyPrefix, []byte(minerAddr)...)
}

// EpochChallengeKey 返回 epoch 挑战索引的完整存储 key
func EpochChallengeKey(epoch uint64) []byte {
	bz := make([]byte, 8)
	bz[0] = byte(epoch >> 56)
	bz[1] = byte(epoch >> 48)
	bz[2] = byte(epoch >> 40)
	bz[3] = byte(epoch >> 32)
	bz[4] = byte(epoch >> 24)
	bz[5] = byte(epoch >> 16)
	bz[6] = byte(epoch >> 8)
	bz[7] = byte(epoch)
	return append(EpochChallengeKeyPrefix, bz...)
}

// ActiveChallengeKey 返回活跃挑战索引的完整存储 key
func ActiveChallengeKey(challengeID string) []byte {
	return append(ActiveChallengeKeyPrefix, []byte(challengeID)...)
}
