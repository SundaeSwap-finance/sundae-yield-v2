package incentive

import (
	"context"
	"testing"

	"github.com/SundaeSwap-finance/ogmigo/v6/ouroboros/chainsync/compatibility"
	"github.com/SundaeSwap-finance/ogmigo/v6/ouroboros/chainsync/num"
	"github.com/SundaeSwap-finance/ogmigo/v6/ouroboros/shared"
	"github.com/SundaeSwap-finance/sundae-yield-v2/calculation/utilities"
	"github.com/SundaeSwap-finance/sundae-yield-v2/types"
	"github.com/tj/assert"
)

func Test_PositionsToOwners(t *testing.T) {
	positions := []types.Position{
		utilities.SamplePosition("A", 100),
		utilities.SamplePosition("A", 200),
		utilities.SamplePosition("B", 150),
	}

	owners := PositionsToOwners(positions)
	assert.Len(t, owners, 2)
	assert.Contains(t, owners, "A")
	assert.Contains(t, owners, "B")
	assert.NotContains(t, owners, "C")
	assert.EqualValues(t, "A", owners["A"].Signature.KeyHash)
}

func Test_CalculateDelegationWeights(t *testing.T) {
	delegation := types.Delegation{Program: "A", PoolIdent: "B", Weight: 10}
	program := utilities.SampleIncentiveProgram()
	positions := []types.Position{
		utilities.SamplePosition("A", 100, delegation),
		utilities.SamplePosition("A", 200, delegation),
		utilities.SamplePosition("B", 150, delegation),
		utilities.SamplePosition("C", 150),
		utilities.SamplePosition("D", 0, delegation),
	}

	pools := utilities.MockLookup{}

	weights, total, err := CalculateDelegationWeights(context.Background(), program, positions, 0, 2592000, pools)
	assert.Nil(t, err)
	assert.EqualValues(t, 450, total.Uint64())
	assert.Len(t, weights, 2)
	assert.Contains(t, weights, "A")
	assert.EqualValues(t, 300, weights["A"])
	assert.Contains(t, weights, "B")
	assert.EqualValues(t, 150, weights["B"])
}

func Test_CalculateDelegationWeights_WithLP(t *testing.T) {
	delegation := types.Delegation{Program: "A", PoolIdent: "B", Weight: 10}
	program := utilities.SampleIncentiveProgram()
	positions := []types.Position{
		utilities.SamplePosition("A", 100, delegation),
		utilities.SamplePosition("A", 200, delegation),
		utilities.SamplePosition("B", 150, delegation),
	}

	positions[0].Value = compatibility.CompatibleValue(shared.ValueFromCoins(
		shared.CreateAdaCoin(num.Uint64(100)),
		shared.Coin{AssetId: shared.AssetID("Staked"), Amount: num.Uint64(100)},
		shared.Coin{AssetId: shared.AssetID("LP_X"), Amount: num.Uint64(100)},
	))

	pools := utilities.MockLookup{
		"X": {PoolIdent: "X", LPAsset: "LP_X", AssetA: shared.AdaAssetID, AssetB: shared.AssetID("Staked"), TotalLPTokens: 500, AssetAQuantity: 1000, AssetBQuantity: 1000},
	}

	weights, total, err := CalculateDelegationWeights(context.Background(), program, positions, 0, 2592000, pools)
	assert.Nil(t, err)
	assert.EqualValues(t, 650, total.Uint64())
	assert.Len(t, weights, 2)
	assert.Contains(t, weights, "A")
	assert.EqualValues(t, 500, weights["A"])
	assert.Contains(t, weights, "B")
	assert.EqualValues(t, 150, weights["B"])
}

func Test_CalculationDelegationWeights_WithTimedPositions(t *testing.T) {
	delegation := types.Delegation{Program: "A", PoolIdent: "B", Weight: 10}
	program := utilities.SampleIncentiveProgram()
	positions := []types.Position{
		utilities.SampleTimedPosition("A", 100, 0, 648000, delegation),
		utilities.SampleTimedPosition("A", 200, 648000, 1296000, delegation),
		utilities.SampleTimedPosition("B", 150, 0, 4592000, delegation),
	}

	positions[0].Value = compatibility.CompatibleValue(shared.ValueFromCoins(
		shared.CreateAdaCoin(num.Uint64(100)),
		shared.Coin{AssetId: shared.AssetID("Staked"), Amount: num.Uint64(100)},
		shared.Coin{AssetId: shared.AssetID("LP_X"), Amount: num.Uint64(100)},
	))

	pools := utilities.MockLookup{
		"X": {PoolIdent: "X", LPAsset: "LP_X", AssetA: shared.AdaAssetID, AssetB: shared.AssetID("Staked"), TotalLPTokens: 500, AssetAQuantity: 1000, AssetBQuantity: 1000},
	}

	weights, total, err := CalculateDelegationWeights(context.Background(), program, positions, 0, 2592000, pools)
	assert.Nil(t, err)
	assert.EqualValues(t, 100/4+200/4+150+200/4, total.Uint64())
	assert.Len(t, weights, 2)
	assert.Contains(t, weights, "A")
	assert.EqualValues(t, 100/4+200/4+200/4, weights["A"])
	assert.Contains(t, weights, "B")
	assert.EqualValues(t, 150, weights["B"])
}

func Test_SplitEmissionsByOwner(t *testing.T) {
	weightByOwner := map[string]uint64{
		"A": 150,
		"B": 125,
	}
	emissionsByOwner := SplitEmissionPerOwner(100_000_000, weightByOwner, num.Uint64(275))
	assert.Contains(t, emissionsByOwner, "A")
	assert.Contains(t, emissionsByOwner, "B")
	assert.EqualValues(t, emissionsByOwner["A"], 54545454)
	assert.EqualValues(t, emissionsByOwner["B"], 45454546)
}

func Test_CalculateLovelaceValue(t *testing.T) {
	pools := utilities.MockLookup{
		"X": {
			PoolIdent:      "X",
			LPAsset:        "LP_X",
			AssetA:         shared.AdaAssetID,
			AssetB:         shared.AssetID("Staked"),
			TotalLPTokens:  500,
			AssetAQuantity: 2_116_632_505_378,
			AssetBQuantity: 153_408_311_896_675,
		},
	}
	value, err := EstimateLovelaceValue(context.Background(), 191_000_000_000_000, "Staked", "X", pools)
	assert.Nil(t, err)
	assert.EqualValues(t, 2_635_299_245_059, value)
}
