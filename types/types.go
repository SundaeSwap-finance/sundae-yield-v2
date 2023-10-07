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

	// Sum up delegations from the last N days, to smooth out instantaneous changes in delegation
	// as per the following governance proposal: https://governance.sundaeswap.finance/#/proposal#fc3294e71a2141f2147b32a72299c0b0bb061d44409d498bc8063141d7b0c0e9
	ConsecutiveDelegationWindow int

	EarningExpiration *time.Duration

	EligiblePools     []string
	DisqualifiedPools []string
	NepotismPools     []string

	MinLPIntegerPercent   int
	MaxPoolCount          int
	MaxPoolIntegerPercent int
}

type Pool struct {
	PoolIdent       string
	TransactionHash string
	Slot            uint64
	TotalLPTokens   uint64
	LPAsset         chainsync.AssetID
	AssetA          chainsync.AssetID
	AssetAQuantity  uint64
	AssetB          chainsync.AssetID
	AssetBQuantity  uint64
}

type Delegation struct {
	Program   string
	PoolIdent string
	Weight    uint32
}

type Position struct {
	OwnerID          string `dynamodbav:"OwnerID" ddb:"gsi_hash:ByOwner"`
	Owner            MultisigScript
	TransactionHash  string
	Slot             uint64
	SpentTransaction string
	SpentSlot        uint64

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
	ValueByLPToken map[string]chainsync.Value
}
