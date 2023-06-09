package types

import (
	"github.com/SundaeSwap-finance/ogmigo/ouroboros/chainsync"
)

const DateFormat = "2006-01-02" // Format dates like this, so they can be compared lexographically
type Date = string

type Program struct {
	FirstDailyRewards Date
	LastDailyRewards  Date

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
	Owner   MultisigScript

	Value      chainsync.Value
	Delegation []Delegation
}

type Earning struct {
	OwnerID    string
	Owner      MultisigScript
	Program    string
	EarnedDate Date
	Value      chainsync.Value
}
