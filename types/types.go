package types

import (
	"time"

	"github.com/SundaeSwap-finance/ogmigo/ouroboros/chainsync"
)

const DateFormat = "2006-01-02" // Format dates like this, so they can be compared lexographically
type Date = string

type Program struct {
	ID                string
	FirstDailyRewards Date
	LastDailyRewards  Date

	DailyEmission uint64 // TODO: generalize to asset?
	EmittedAsset  chainsync.AssetID
	StakedAsset   chainsync.AssetID
	ReferencePool string // Which pool should we use as a reference when estimating locked value?

	EarningExpiration *time.Duration

	EligiblePools     []string
	DisqualifiedPools []string
	NepotismPools     []string

	MinLPIntegerPercent   int
	MaxPoolCount          int
	MaxPoolIntegerPercent int
}

type Pool struct {
	PoolIdent      string
	TotalLPTokens  uint64
	LPAsset        chainsync.AssetID
	AssetA         chainsync.AssetID
	AssetAQuantity uint64
	AssetB         chainsync.AssetID
	AssetBQuantity uint64
}

type Delegation struct {
	Program   string
	PoolIdent string
	Weight    uint32
}

type Position struct {
	OwnerID string `dynamodbav:"OwnerID" ddb:"gsi_hash:ByOwner"`
	Owner   MultisigScript

	Value      chainsync.Value
	Delegation []Delegation
}

type Earning struct {
	OwnerID        string
	Owner          MultisigScript
	Program        string
	EarnedDate     Date
	ExpirationDate *time.Time
	Value          chainsync.Value
}
