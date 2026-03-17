package types

// 事件类型
const (
	EventTypeScoreUpdate  = "score_update"
	EventTypeSuspend      = "miner_suspended"
	AttributeKeyMiner     = "miner"
	AttributeKeyOldScore  = "old_score"
	AttributeKeyNewScore  = "new_score"
	AttributeKeyReason    = "reason"
)
