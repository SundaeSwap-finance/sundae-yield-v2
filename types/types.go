package types

import (
	"time"

	"github.com/SundaeSwap-finance/ogmigo/v6/ouroboros/chainsync/compatibility"
	"github.com/SundaeSwap-finance/ogmigo/v6/ouroboros/shared"
	// "github.com/SundaeSwap-finance/ogmigo/v6/ouroboros/chainsync/compatibility"
)

const DateFormat = "2006-01-02" // Format dates like this, so they can be compared lexographically
type Date = string

type Program struct {
	ID                string
	FirstDailyRewards Date
	LastDailyRewards  Date

	DailyEmission uint64 // TODO: generalize to asset?
	EmittedAsset  shared.AssetID
	StakedAsset   shared.AssetID
	ReferencePool string // Which pool should we use as a reference when estimating locked value?

	// Sum up delegations from the last N days, to smooth out instantaneous changes in delegation
	// as per the following governance proposal: https://governance.sundaeswap.finance/#/proposal#fc3294e71a2141f2147b32a72299c0b0bb061d44409d498bc8063141d7b0c0e9
	ConsecutiveDelegationWindow int

	EarningExpiration *time.Duration

	// Any pools that received a fixed emission
	// for example, pool 08 receives exactly 133234.5 tokens per day
	// (30% of the daily emissions of 444115)
	FixedEmissions map[string]uint64

	// The maximum emissions, outside of the fixed emissions above,
	// that any pool may receive for its delegation
	// For example, this is set to 62176.1, as 14% of 444115
	// Any remaining emissions above this are *not* emitted, and instead rever to the treasury
	EmissionCap uint64

	// A list of pools for which a delegation is considered valid
	EligiblePools []string
	// A list of assets for which *any* pools will be considered valid
	EligibleAssets []shared.AssetID
	// A list of assets, for which *any* pool with these two assets will be considered valid
	EligiblePairs []struct {
		AssetA shared.AssetID
		AssetB shared.AssetID
	}
	// A list of pools for which delegation will be ignored
	DisqualifiedPools []string
	// A list of assets, for which *any* pools will be considered invalid
	DisqualifiedAssets []shared.AssetID
	// A list of assets, for which *any* pool with these two assets will be considered invalid
	DisqualifiedPairs []struct {
		AssetA shared.AssetID
		AssetB shared.AssetID
	}
	// A list of pools which are automatically considered to have crossed the percentile threshold
	NepotismPools []string

	MinLPIntegerPercent   int
	MaxPoolCount          int
	MaxPoolIntegerPercent int
}

type Pool struct {
	PoolIdent       string
	TransactionHash string
	Slot            uint64
	TotalLPTokens   uint64
	LPAsset         shared.AssetID
	AssetA          shared.AssetID
	AssetAQuantity  uint64
	AssetB          shared.AssetID
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

	Value      shared.Value
	Delegation []Delegation
}

type Earning struct {
	OwnerID        string
	Owner          MultisigScript
	Program        string
	EarnedDate     Date
	ExpirationDate *time.Time
	Value          compatibility.CompatibleValue
	ValueByLPToken map[string]compatibility.CompatibleValue
}
