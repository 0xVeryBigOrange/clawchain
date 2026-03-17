package types

const (
	ModuleName = "reputation"
	StoreKey   = ModuleName
	RouterKey  = ModuleName
)

var (
	ScoreKeyPrefix = []byte{0x01}
)

func GetScoreKey(addr string) []byte {
	return append(ScoreKeyPrefix, []byte(addr)...)
}
