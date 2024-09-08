package utilities

import (
	"context"
	"fmt"
	"strings"

	"github.com/SundaeSwap-finance/ogmigo/v6/ouroboros/chainsync/compatibility"
	"github.com/SundaeSwap-finance/ogmigo/v6/ouroboros/chainsync/num"
	"github.com/SundaeSwap-finance/ogmigo/v6/ouroboros/shared"
	"github.com/SundaeSwap-finance/sundae-yield-v2/types"
)

func SampleYieldProgram(emissions uint64) types.YieldProgram {
	return types.YieldProgram{
		ID:                  "TestYield",
		FirstDailyRewards:   "2001-01-01",
		LastDailyRewards:    "2099-01-01", // If this code is still in use in 2099, call the police (after updating tests)
		StakedAsset:         shared.AssetID("Staked"),
		MinLPIntegerPercent: 1,
		EmittedAsset:        shared.AssetID("Emitted"),
		DailyEmission:       emissions,
	}
}

func SampleIncentiveProgram() types.IncentiveProgram {
	return types.IncentiveProgram{
		ID:                   "TestIncentive",
		FirstDailyRewards:    "2001-01-01",
		LastDailyRewards:     "2099-01-01",
		StakedAsset:          shared.AssetID("Staked"),
		EmittedAsset:         shared.AssetID("Emitted"),
		StakedReferencePool:  "X",
		EmittedReferencePool: "Y",
	}
}

func SamplePosition(owner string, staked int64, delegations ...types.Delegation) types.Position {
	return types.Position{
		OwnerID: owner,
		Owner: types.MultisigScript{
			Signature: &types.Signature{
				KeyHash: []byte(owner),
			},
		},
		Slot:       0,
		SpentSlot:  0,
		Value:      compatibility.CompatibleValue(shared.ValueFromCoins(shared.Coin{AssetId: shared.AssetID("Staked"), Amount: num.Int64(staked)})),
		Delegation: delegations,
	}
}

func SampleTimedPosition(owner string, staked int64, start, end uint64) types.Position {
	spentTx := ""
	if end > 0 {
		spentTx = "SPENT"
	}
	return types.Position{
		OwnerID: owner,
		Owner: types.MultisigScript{
			Signature: &types.Signature{
				KeyHash: []byte(owner),
			},
		},
		Slot:             start,
		SpentSlot:        end,
		SpentTransaction: spentTx,
		Value:            compatibility.CompatibleValue(shared.ValueFromCoins(shared.Coin{AssetId: shared.AssetID("Staked"), Amount: num.Int64(staked)})),
		Delegation:       []types.Delegation{},
	}
}

type MockLookup map[string]types.Pool

func (m MockLookup) PoolByIdent(ctx context.Context, poolIdent string) (types.Pool, error) {
	if pool, ok := m[poolIdent]; ok {
		return pool, nil
	}
	return types.Pool{}, fmt.Errorf("pool not found")
}
func (m MockLookup) PoolByLPToken(ctx context.Context, lpToken shared.AssetID) (types.Pool, error) {
	if !m.IsLPToken(lpToken) {
		return types.Pool{}, fmt.Errorf("not an lp token")
	}
	for _, pool := range m {
		if pool.LPAsset == lpToken {
			return pool, nil
		}
	}
	return types.Pool{}, fmt.Errorf("pool not found")
}
func (m MockLookup) IsLPToken(assetId shared.AssetID) bool {
	return strings.HasPrefix(assetId.String(), "LP_")
}
func (m MockLookup) LPTokenToPoolIdent(lpToken shared.AssetID) (string, error) {
	if pool, err := m.PoolByLPToken(context.Background(), lpToken); err != nil {
		return "", err
	} else {
		return pool.PoolIdent, nil
	}
}
