package types

// =====================================
// 消息响应类型
// =====================================

// MsgSubmitCommitResponse 是 MsgSubmitCommit 的响应
type MsgSubmitCommitResponse struct{}

// MsgSubmitRevealResponse 是 MsgSubmitReveal 的响应
type MsgSubmitRevealResponse struct{}

// =====================================
// MsgServer 接口定义
// =====================================

// MsgServer 定义 challenge 模块的消息处理接口
type MsgServer interface {
	// SubmitCommit 处理矿工提交 commit
	SubmitCommit(ctx interface{}, msg *MsgSubmitCommit) (*MsgSubmitCommitResponse, error)
	// SubmitReveal 处理矿工揭示答案
	SubmitReveal(ctx interface{}, msg *MsgSubmitReveal) (*MsgSubmitRevealResponse, error)
}

// RegisterMsgServer 注册消息服务器
func RegisterMsgServer(_ interface{}, _ MsgServer) {
	// 占位实现，实际注册在 module.go 的 RegisterServices 中
}
