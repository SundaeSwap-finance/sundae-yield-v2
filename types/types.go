package types

import (
	"time"

	"github.com/SundaeSwap-finance/ogmigo/ouroboros/chainsync"
)

type Program struct {
	DailyEmission     uint64 // TODO: generalize to asset?
	EmittedAsset      chainsync.AssetID
	StakedAsset       chainsync.AssetID
	EligiblePools     []string
	DisqualifiedPools []string

	MinLPIntegerPercent   int
	MaxPoolCount          int
	MaxPoolIntegerPercent int
}

type Pool struct {
	PoolIdent     string
	TotalLPTokens uint64
	LPAsset       chainsync.AssetID
}

type Delegation struct {
	PoolIdent string
	Weight    uint32
}

type Position struct {
	OwnerID string
	Owner   NativeScript

	Value      chainsync.Value
	Delegation []Delegation
}

type Earning struct {
	Owner string
	Date  time.Time
	Value chainsync.Value
}
