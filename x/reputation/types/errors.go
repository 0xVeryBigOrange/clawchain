package types

import "cosmossdk.io/errors"

var (
	ErrMinerNotFound = errors.Register(ModuleName, 1, "miner reputation not found")
)
