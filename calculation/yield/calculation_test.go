package yield

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/SundaeSwap-finance/ogmigo/v6/ouroboros/chainsync/compatibility"
	"github.com/SundaeSwap-finance/ogmigo/v6/ouroboros/chainsync/num"
	"github.com/SundaeSwap-finance/ogmigo/v6/ouroboros/shared"
	"github.com/SundaeSwap-finance/sundae-yield-v2/calculation/utilities"
	"github.com/SundaeSwap-finance/sundae-yield-v2/types"
	"github.com/tj/assert"
)

func Test_TotalDelegations(t *testing.T) {
	program := utilities.SampleYieldProgram(500000_000_000)

	// The simplest case
	positions := []types.Position{
		utilities.SamplePosition("Me", 100_000, types.Delegation{Program: program.ID, PoolIdent: "01", Weight: 1}),
	}
	delegationsByPool, totalDelegations, err := CalculateTotalDelegations(context.Background(), program, positions, utilities.MockLookup{})
	assert.Nil(t, err)
	assert.EqualValues(t, 100_000, delegationsByPool["01"])
	assert.EqualValues(t, 100_000, totalDelegations)

	positions = []types.Position{
		utilities.SamplePosition("Me", 100_000),
	}
	delegationsByPool, totalDelegations, err = CalculateTotalDelegations(context.Background(), program, positions, utilities.MockLookup{})
	assert.Nil(t, err)
	assert.EqualValues(t, 100_000, delegationsByPool[""])
	assert.EqualValues(t, 100_000, totalDelegations)

	// Should split evenly between delegations
	positions = []types.Position{
		utilities.SamplePosition("Me", 100_000, types.Delegation{Program: program.ID, PoolIdent: "01", Weight: 1}, types.Delegation{Program: program.ID, PoolIdent: "02", Weight: 1}),
	}
	delegationsByPool, totalDelegations, err = CalculateTotalDelegations(context.Background(), program, positions, utilities.MockLookup{})
	assert.Nil(t, err)
	assert.EqualValues(t, 50_000, delegationsByPool["01"])
	assert.EqualValues(t, 50_000, delegationsByPool["02"])
	assert.EqualValues(t, 100_000, totalDelegations)

	// Should handle bankers rounding
	positions = []types.Position{
		utilities.SamplePosition("Me", 100_000, types.Delegation{Program: program.ID, PoolIdent: "01", Weight: 1}, types.Delegation{Program: program.ID, PoolIdent: "02", Weight: 2}),
	}
	delegationsByPool, totalDelegations, err = CalculateTotalDelegations(context.Background(), program, positions, utilities.MockLookup{})
	assert.Nil(t, err)
	assert.EqualValues(t, 33_334, delegationsByPool["01"])
	assert.EqualValues(t, 66_666, delegationsByPool["02"])
	assert.EqualValues(t, 100_000, totalDelegations)

	// Should handle multiple positions
	positions = []types.Position{
		utilities.SamplePosition("Me", 100_000, types.Delegation{Program: program.ID, PoolIdent: "01", Weight: 1}, types.Delegation{Program: program.ID, PoolIdent: "02", Weight: 1}),
		utilities.SamplePosition("Me", 200_000, types.Delegation{Program: program.ID, PoolIdent: "02", Weight: 1}, types.Delegation{Program: program.ID, PoolIdent: "03", Weight: 1}),
	}
	delegationsByPool, totalDelegations, err = CalculateTotalDelegations(context.Background(), program, positions, utilities.MockLookup{})
	assert.Nil(t, err)
	assert.EqualValues(t, 50_000, delegationsByPool["01"])
	assert.EqualValues(t, 150_000, delegationsByPool["02"])
	assert.EqualValues(t, 100_000, delegationsByPool["03"])
	assert.EqualValues(t, 300_000, totalDelegations)

	// Should handle positions with LP tokens
	pools := utilities.MockLookup{
		"01": {
			PoolIdent:      "01",
			TotalLPTokens:  100_000,
			LPAsset:        "LP_01",
			AssetA:         "",
			AssetB:         program.StakedAsset,
			AssetAQuantity: 200_000,
			AssetBQuantity: 100_000,
		},
	}
	positions = []types.Position{
		utilities.SamplePosition("Me", 100_000, types.Delegation{Program: program.ID, PoolIdent: "01", Weight: 1}, types.Delegation{Program: program.ID, PoolIdent: "02", Weight: 1}),
	}
	value := shared.Value(positions[0].Value)
	value.AddAsset(shared.Coin{AssetId: "LP_01", Amount: num.Uint64(50_000)})
	positions[0].Value = compatibility.CompatibleValue(value)
	delegationsByPool, totalDelegations, err = CalculateTotalDelegations(context.Background(), program, positions, pools)
	assert.Nil(t, err)
	assert.EqualValues(t, 75_000, int64(delegationsByPool["01"]))
	assert.EqualValues(t, 75_000, delegationsByPool["02"])
	assert.EqualValues(t, 150_000, totalDelegations)

	// Should handle delegations to multiple programs

	positions = []types.Position{
		utilities.SamplePosition("Me", 100_000, types.Delegation{Program: program.ID, PoolIdent: "01", Weight: 1}, types.Delegation{Program: "OTHER_PROGRAM", PoolIdent: "99", Weight: 100}, types.Delegation{Program: program.ID, PoolIdent: "02", Weight: 1}),
		utilities.SamplePosition("Me", 200_000, types.Delegation{Program: program.ID, PoolIdent: "02", Weight: 1}, types.Delegation{Program: program.ID, PoolIdent: "03", Weight: 1}),
	}

	delegationsByPool, totalDelegations, err = CalculateTotalDelegations(context.Background(), program, positions, utilities.MockLookup{})
	assert.Nil(t, err)
	assert.EqualValues(t, 50_000, delegationsByPool["01"])
	assert.EqualValues(t, 150_000, delegationsByPool["02"])
	assert.EqualValues(t, 100_000, delegationsByPool["03"])
	assert.EqualValues(t, 300_000, totalDelegations)

	// Should handle delegations in programs with pool remappings
	program.DelegationRemap = map[string]string{
		"01": "01V3",
		"02": "02V3",
	}
	positions = []types.Position{
		utilities.SamplePosition("Me", 123_000, types.Delegation{Program: program.ID, PoolIdent: "01", Weight: 1}),
		utilities.SamplePosition("Me", 456_000, types.Delegation{Program: program.ID, PoolIdent: "02", Weight: 1}),
		utilities.SamplePosition("Me", 222_000, types.Delegation{Program: program.ID, PoolIdent: "03", Weight: 1}),
		utilities.SamplePosition("Me", 100_000, types.Delegation{Program: program.ID, PoolIdent: "02", Weight: 1}, types.Delegation{Program: program.ID, PoolIdent: "03", Weight: 1}),
		utilities.SamplePosition("Me", 200_000, types.Delegation{Program: program.ID, PoolIdent: "01V3", Weight: 1}),
	}
	delegationsByPool, totalDelegations, err = CalculateTotalDelegations(context.Background(), program, positions, utilities.MockLookup{})
	assert.Nil(t, err)
	assert.EqualValues(t, 123_000+200_000, delegationsByPool["01V3"])
	assert.EqualValues(t, 456_000+50_000, delegationsByPool["02V3"])
	assert.EqualValues(t, 222_000+50_000, delegationsByPool["03"])
	assert.EqualValues(t, 0, delegationsByPool["03V3"])
	assert.EqualValues(t, totalDelegations, 123_000+456_000+222_000+100_000+200_000)
}

func Test_SummationConstraint(t *testing.T) {
	program := utilities.SampleYieldProgram(500000_000_000)

	// Should always add up to initial sundae
	initialSundae := rand.Int63()
	delegationCount := rand.Intn(30)
	delegations := []types.Delegation{}
	for i := 0; i < delegationCount; i++ {
		pool := rand.Intn(10)
		weight := rand.Int63()
		delegations = append(delegations, types.Delegation{PoolIdent: fmt.Sprintf("%v", pool), Weight: uint32(weight)})
	}
	positions := []types.Position{
		utilities.SamplePosition("Me", initialSundae, delegations...),
	}
	delegationsByPool, totalDelegation, err := CalculateTotalDelegations(context.Background(), program, positions, utilities.MockLookup{})
	assert.Nil(t, err)
	actualSum := uint64(0)
	for _, s := range delegationsByPool {
		actualSum += s
	}
	assert.EqualValues(t, initialSundae, actualSum)
	assert.EqualValues(t, totalDelegation, actualSum)
}

func Test_SumWindow(t *testing.T) {
	program := utilities.SampleYieldProgram(500000_000_000)
	program.ConsecutiveDelegationWindow = 2
	qualifyingDelegations := map[string]uint64{
		"A": 100,
		"B": 200,
	}
	previousDays := []CalculationOutputs{
		{
			QualifyingDelegationByPool: map[string]uint64{
				"B": 300,
				"C": 400,
			},
		},
	}
	window, err := SumDelegationWindow(program, qualifyingDelegations, previousDays)
	assert.Nil(t, err)
	assert.Len(t, window, 3)
	assert.EqualValues(t, window["A"], 100)
	assert.EqualValues(t, window["B"], 500)
	assert.EqualValues(t, window["C"], 400)
}

func Test_AtLeastOnePercent(t *testing.T) {
	assert.False(t, atLeastIntegerPercent(0, 15000, 1))
	assert.False(t, atLeastIntegerPercent(1, 15000, 1))
	assert.False(t, atLeastIntegerPercent(149, 15000, 1))
	assert.False(t, atLeastIntegerPercent(1499, 150000, 1))
	assert.False(t, atLeastIntegerPercent(1234, 15000, 9))
	assert.False(t, atLeastIntegerPercent(33698506090921, 42448490781434, 80))
	assert.True(t, atLeastIntegerPercent(0, 15000, 0))
	assert.True(t, atLeastIntegerPercent(150, 15000, 1))
	assert.True(t, atLeastIntegerPercent(151, 15000, 1))
	assert.True(t, atLeastIntegerPercent(9000, 15000, 1))
	assert.True(t, atLeastIntegerPercent(15000, 15000, 1))
	assert.True(t, atLeastIntegerPercent(1234, 15000, 8))
}

func Test_QualifiedPools(t *testing.T) {
	program := utilities.SampleYieldProgram(500_000)
	poolA := types.Pool{Version: "V1", PoolIdent: "A", AssetA: "A", AssetB: "X", TotalLPTokens: 1500}
	poolB := types.Pool{Version: "V1", PoolIdent: "B", AssetA: "B", AssetB: "X", TotalLPTokens: 1500}
	poolC := types.Pool{Version: "V1", PoolIdent: "C", AssetA: "C", AssetB: "Y", TotalLPTokens: 1500}
	poolD := types.Pool{Version: "V3", PoolIdent: "D", AssetA: "A", AssetB: "X", TotalLPTokens: 1500}
	poolE := types.Pool{Version: "V3-stable", PoolIdent: "D", AssetA: "A", AssetB: "X", TotalLPTokens: 1500}

	assertQualified := func(pool types.Pool, qty uint64) {
		actual, _ := isPoolQualified(program, pool, qty)
		assert.True(t, actual)
	}
	assertDisqualified := func(pool types.Pool, qty uint64) {
		actual, _ := isPoolQualified(program, pool, qty)
		assert.False(t, actual)
	}

	assertQualified(poolA, 150)
	assertQualified(poolA, 500)
	assertDisqualified(poolA, 10)

	program.EligibleVersions = []string{"V1"}
	assertQualified(poolA, 150)
	assertDisqualified(poolD, 150)
	assertDisqualified(poolE, 150)

	program.EligibleVersions = []string{"V1", "V3"}
	assertQualified(poolA, 150)
	assertQualified(poolD, 150)
	assertDisqualified(poolE, 150)

	program.EligibleVersions = nil

	program.EligiblePools = []string{"A"}
	assertQualified(poolA, 500)
	assertDisqualified(poolB, 500)
	program.EligiblePools = nil

	program.EligibleAssets = []shared.AssetID{"A"}
	assertQualified(poolA, 500)
	assertDisqualified(poolB, 500)
	program.EligibleAssets = nil

	program.EligibleAssets = []shared.AssetID{"X"}
	assertQualified(poolA, 500)
	assertQualified(poolB, 500)
	assertDisqualified(poolC, 500)
	program.EligibleAssets = nil

	program.EligiblePairs = []struct {
		AssetA shared.AssetID
		AssetB shared.AssetID
	}{
		{AssetA: "A", AssetB: "X"},
	}
	assertQualified(poolA, 500)
	assertDisqualified(poolB, 500)
	assertDisqualified(poolC, 500)
	program.EligiblePairs = nil

	program.DisqualifiedVersions = []string{"V1"}
	assertDisqualified(poolA, 500)
	assertQualified(poolD, 500)
	assertQualified(poolE, 500)

	program.DisqualifiedVersions = []string{"V1", "V3-stable"}
	assertDisqualified(poolA, 500)
	assertQualified(poolD, 500)
	assertDisqualified(poolE, 500)

	program.DisqualifiedVersions = nil

	program.DisqualifiedPools = []string{"A"}
	assertDisqualified(poolA, 500)
	assertQualified(poolB, 500)
	program.DisqualifiedPools = nil

	program.DisqualifiedAssets = []shared.AssetID{"X"}
	assertDisqualified(poolA, 500)
	assertDisqualified(poolB, 500)
	assertQualified(poolC, 500)
	program.DisqualifiedAssets = nil

	program.DisqualifiedPairs = []struct {
		AssetA shared.AssetID
		AssetB shared.AssetID
	}{
		{AssetA: "B", AssetB: "X"},
	}
	assertQualified(poolA, 500)
	assertDisqualified(poolB, 500)
	assertQualified(poolC, 500)
	program.DisqualifiedPairs = nil
}

func Test_CalculateTotalLP(t *testing.T) {
	positions := []types.Position{
		{OwnerID: "A", Value: compatibility.CompatibleValue(shared.ValueFromCoins(shared.Coin{AssetId: "LP_X", Amount: num.Uint64(100)}))},
		{OwnerID: "B", Value: compatibility.CompatibleValue(shared.ValueFromCoins(shared.Coin{AssetId: "LP_X", Amount: num.Uint64(200)}))},
		{OwnerID: "C", Value: compatibility.CompatibleValue(shared.ValueFromCoins(shared.Coin{AssetId: "LP_Y", Amount: num.Uint64(500)}))},
	}
	pools := utilities.MockLookup{
		"X": {PoolIdent: "X", LPAsset: "LP_X", TotalLPTokens: 500, AssetAQuantity: 1000},
		"Y": {PoolIdent: "Y", LPAsset: "LP_Y", TotalLPTokens: 1000, AssetAQuantity: 100},
	}
	lockedLP, totalLP, totalValueByPool, totalValue, err := CalculateTotalLPAtSnapshot(context.Background(), 0, positions, nil, pools)
	assert.Nil(t, err)
	assert.EqualValues(t, 300, lockedLP["X"])
	assert.EqualValues(t, 500, lockedLP["Y"])
	assert.EqualValues(t, 500, totalLP["X"])
	assert.EqualValues(t, 1000, totalLP["Y"])
	assert.EqualValues(t, 1200, totalValueByPool["X"])
	assert.EqualValues(t, 100, totalValueByPool["Y"])
	assert.EqualValues(t, 1300, totalValue)
}

func Test_CalculateTotalLPWithAssetNames(t *testing.T) {
	positions := []types.Position{
		{OwnerID: "A", Value: compatibility.CompatibleValue(shared.ValueFromCoins(shared.Coin{AssetId: "LP_X.Asset1", Amount: num.Uint64(100)}))},
		{OwnerID: "B", Value: compatibility.CompatibleValue(shared.ValueFromCoins(shared.Coin{AssetId: "LP_X.Asset1", Amount: num.Uint64(200)}))},
		{OwnerID: "C", Value: compatibility.CompatibleValue(shared.ValueFromCoins(shared.Coin{AssetId: "LP_Y.Asset1", Amount: num.Uint64(500)}))},
	}
	pools := utilities.MockLookup{
		"X": {PoolIdent: "X", LPAsset: "LP_X.Asset1", TotalLPTokens: 500, AssetAQuantity: 1000},
		"Y": {PoolIdent: "Y", LPAsset: "LP_Y.Asset1", TotalLPTokens: 1000, AssetAQuantity: 100},
	}

	lockedLP, totalLP, totalValueByPool, totalValue, err := CalculateTotalLPAtSnapshot(context.Background(), 0, positions, nil, pools)
	assert.Nil(t, err)
	assert.EqualValues(t, 300, lockedLP["X"])
	assert.EqualValues(t, 500, lockedLP["Y"])
	assert.EqualValues(t, 500, totalLP["X"])
	assert.EqualValues(t, 1000, totalLP["Y"])
	assert.EqualValues(t, 1200, totalValueByPool["X"])
	assert.EqualValues(t, 100, totalValueByPool["Y"])
	assert.EqualValues(t, 1300, totalValue)
}

func Test_PoolForEmissions(t *testing.T) {
	program := utilities.SampleYieldProgram(500_000)
	program.MaxPoolCount = 2
	program.MaxPoolIntegerPercent = 100
	pools := utilities.MockLookup{"A": types.Pool{PoolIdent: "A"}, "B": types.Pool{PoolIdent: "B"}, "C": types.Pool{PoolIdent: "C"}}
	selectedPools, err := SelectEligiblePoolsForEmission(context.Background(), program, map[string]uint64{
		"A": 100,
		"B": 200,
		"C": 300,
	}, pools)
	assert.Nil(t, err)
	assert.EqualValues(t, map[string]uint64{"C": 300, "B": 200}, selectedPools)

	program.MaxPoolIntegerPercent = 30
	selectedPools, err = SelectEligiblePoolsForEmission(context.Background(), program, map[string]uint64{
		"A": 100,
		"B": 101,
		"C": 202,
	}, pools)
	assert.Nil(t, err)
	assert.EqualValues(t, map[string]uint64{"C": 202}, selectedPools)

	program.MaxPoolCount = 10
	program.MaxPoolIntegerPercent = 33
	pools = utilities.MockLookup{
		"A": types.Pool{},
		"B": types.Pool{},
		"C": types.Pool{},
		"D": types.Pool{},
		"E": types.Pool{},
		"F": types.Pool{},
	}
	selectedPools, err = SelectEligiblePoolsForEmission(context.Background(), program, map[string]uint64{
		"A": 997,
		"B": 998,
		"C": 999,
		"D": 1000,
		"E": 1001,
		"F": 1002, // Ensures that F+E are *just* slightly over 33%, A-D should get excluded
	}, pools)
	assert.Nil(t, err)
	assert.EqualValues(t, map[string]uint64{"F": 1002, "E": 1001}, selectedPools)
}

func Test_PoolsForEmissions_WithNepotism(t *testing.T) {
	program := utilities.SampleYieldProgram(500_000)
	program.NepotismPools = []string{"B"}
	program.MaxPoolCount = 2
	program.MaxPoolIntegerPercent = 20
	selectedPools, err := SelectEligiblePoolsForEmission(context.Background(), program, map[string]uint64{
		"A": 50,
		"B": 100,
		"C": 200,
		"D": 300,
	}, utilities.MockLookup{"A": types.Pool{}, "B": types.Pool{}, "C": types.Pool{}, "D": types.Pool{}})
	assert.Nil(t, err)
	assert.EqualValues(t, map[string]uint64{"D": 300, "B": 100}, selectedPools)
}

func Test_EmissionsToPools(t *testing.T) {
	program := utilities.SampleYieldProgram(500_000_000_000)
	emissions := DistributeEmissionsToPools(program, map[string]uint64{
		"A": 1000,
	})
	assert.EqualValues(t, map[string]uint64{"A": 500_000_000_000}, emissions)

	emissions = DistributeEmissionsToPools(program, map[string]uint64{
		"A": 1000,
		"B": 1000,
	})
	assert.EqualValues(t, map[string]uint64{"A": 250_000_000_000, "B": 250_000_000_000}, emissions)

	emissions = DistributeEmissionsToPools(program, map[string]uint64{
		"A": 1000,
		"B": 2000,
	})
	assert.EqualValues(t, map[string]uint64{"A": 166_666_666_666, "B": 333_333_333_334}, emissions)

	program.FixedEmissions = map[string]uint64{
		"C": 1_000_000_000,
	}
	emissions = DistributeEmissionsToPools(program, map[string]uint64{
		"A": 1000,
		"B": 2000,
		"C": 1000,
	})
	assert.EqualValues(t, map[string]uint64{"A": 166_333_333_333, "B": 332_666_666_667, "C": 1_000_000_000}, emissions)
}

func Test_TruncateEmissions(t *testing.T) {
	program := utilities.SampleYieldProgram(500_000_000_000)
	program.EmissionCap = 200_000_000_000
	program.FixedEmissions = map[string]uint64{
		"C": 1_000_000_000,
	}
	rawEmissions := DistributeEmissionsToPools(program, map[string]uint64{
		"A": 1000,
		"B": 2000,
	})
	truncatedEmissions := TruncateEmissions(program, rawEmissions)
	assert.EqualValues(t, map[string]uint64{"A": 166_333_333_333, "B": 200_000_000_000, "C": 1_000_000_000}, truncatedEmissions)
}

func Test_OwnerByLPAndAsset(t *testing.T) {
	pools := utilities.MockLookup{
		"X": {PoolIdent: "X", LPAsset: "LP_X"},
		"Y": {PoolIdent: "Y", LPAsset: "LP_Y"},
	}
	byOwner, byAsset := TotalLPDaysByOwnerAndAsset([]types.Position{
		{OwnerID: "A", Value: compatibility.CompatibleValue(shared.ValueFromCoins(shared.Coin{AssetId: "LP_X", Amount: num.Uint64(100)}))},
	}, pools, 0, 86400)
	assert.EqualValues(t, map[string]map[shared.AssetID]uint64{
		"A": {"LP_X": 100},
	}, byOwner)
	assert.EqualValues(t, map[shared.AssetID]uint64{
		"LP_X": 100,
	}, byAsset)

	byOwner, byAsset = TotalLPDaysByOwnerAndAsset([]types.Position{
		{OwnerID: "A", Value: compatibility.CompatibleValue(shared.ValueFromCoins(shared.Coin{AssetId: "LP_X", Amount: num.Uint64(100)}))},
		{OwnerID: "B", Value: compatibility.CompatibleValue(shared.ValueFromCoins(shared.Coin{AssetId: "LP_X", Amount: num.Uint64(200)}))},
	}, pools, 0, 86400)
	assert.EqualValues(t, map[string]map[shared.AssetID]uint64{
		"A": {"LP_X": 100},
		"B": {"LP_X": 200},
	}, byOwner)
	assert.EqualValues(t, map[shared.AssetID]uint64{
		"LP_X": 300,
	}, byAsset)

	byOwner, byAsset = TotalLPDaysByOwnerAndAsset([]types.Position{
		{OwnerID: "A", Value: compatibility.CompatibleValue(shared.ValueFromCoins(shared.Coin{AssetId: "LP_X", Amount: num.Uint64(100)}))},
		{OwnerID: "B", Value: compatibility.CompatibleValue(shared.ValueFromCoins(shared.Coin{AssetId: "LP_X", Amount: num.Uint64(200)}))},
		{OwnerID: "B", Value: compatibility.CompatibleValue(shared.ValueFromCoins(shared.Coin{AssetId: "LP_X", Amount: num.Uint64(300)}))},
	}, pools, 0, 86400)
	assert.EqualValues(t, map[string]map[shared.AssetID]uint64{
		"A": {"LP_X": 100},
		"B": {"LP_X": 500},
	}, byOwner)
	assert.EqualValues(t, map[shared.AssetID]uint64{
		"LP_X": 600,
	}, byAsset)

	byOwner, byAsset = TotalLPDaysByOwnerAndAsset([]types.Position{
		{OwnerID: "A", Value: compatibility.CompatibleValue(shared.ValueFromCoins(shared.Coin{AssetId: "LP_X", Amount: num.Uint64(100)}))},
		{OwnerID: "B", Value: compatibility.CompatibleValue(shared.ValueFromCoins(shared.Coin{AssetId: "LP_X", Amount: num.Uint64(200)}, shared.Coin{AssetId: "LP_Y", Amount: num.Uint64(150)}))},
		{OwnerID: "B", Value: compatibility.CompatibleValue(shared.ValueFromCoins(shared.Coin{AssetId: "LP_X", Amount: num.Uint64(300)}))},
	}, pools, 0, 86400)
	assert.EqualValues(t, map[string]map[shared.AssetID]uint64{
		"A": {"LP_X": 100},
		"B": {"LP_X": 500, "LP_Y": 150},
	}, byOwner)
	assert.EqualValues(t, map[shared.AssetID]uint64{
		"LP_X": 600,
		"LP_Y": 150,
	}, byAsset)

	// Test the time-weighting bits
	byOwner, byAsset = TotalLPDaysByOwnerAndAsset([]types.Position{
		// Half day
		{OwnerID: "A", Slot: 143200, Value: compatibility.CompatibleValue(shared.ValueFromCoins(shared.Coin{AssetId: "LP_X", Amount: num.Uint64(100)}))},
		// Quarter day, with rounding down
		{OwnerID: "B", Slot: 143200, SpentTransaction: "A", SpentSlot: 164800, Value: compatibility.CompatibleValue(shared.ValueFromCoins(shared.Coin{AssetId: "LP_X", Amount: num.Uint64(200)}, shared.Coin{AssetId: "LP_Y", Amount: num.Uint64(150)}))},
		// Lockup before the day starts
		{OwnerID: "C", Slot: 12, Value: compatibility.CompatibleValue(shared.ValueFromCoins(shared.Coin{AssetId: "LP_X", Amount: num.Uint64(300)}))},
		// Consecutive positions, constituting half, plus after day ends
		{OwnerID: "D", Slot: 143200, SpentTransaction: "B", SpentSlot: 164800, Value: compatibility.CompatibleValue(shared.ValueFromCoins(shared.Coin{AssetId: "LP_X", Amount: num.Uint64(300)}))},
		{OwnerID: "D", Slot: 164800, SpentTransaction: "C", SpentSlot: 264800, Value: compatibility.CompatibleValue(shared.ValueFromCoins(shared.Coin{AssetId: "LP_X", Amount: num.Uint64(300)}))},
	}, pools, 100000, 186400)
	assert.EqualValues(t, map[string]map[shared.AssetID]uint64{
		"A": {"LP_X": 50},
		"B": {"LP_X": 50, "LP_Y": 37},
		"C": {"LP_X": 300},
		"D": {"LP_X": 150},
	}, byOwner)
	assert.EqualValues(t, map[shared.AssetID]uint64{
		"LP_X": 550,
		"LP_Y": 37,
	}, byAsset)
}

func Test_RegroupByAsset(t *testing.T) {
	pools := utilities.MockLookup{
		"X": {PoolIdent: "X", LPAsset: "LP_X"},
		"Y": {PoolIdent: "Y", LPAsset: "LP_Y"},
	}
	byAsset, err := RegroupByAsset(context.Background(), map[string]uint64{"X": 100}, pools)
	assert.Nil(t, err)
	assert.EqualValues(t, map[shared.AssetID]uint64{"LP_X": 100}, byAsset)

	byAsset, err = RegroupByAsset(context.Background(), map[string]uint64{"X": 100, "Y": 200}, pools)
	assert.Nil(t, err)
	assert.EqualValues(t, map[shared.AssetID]uint64{"LP_X": 100, "LP_Y": 200}, byAsset)
}

type Alloc struct {
	string
	shared.AssetID
	uint64
}

func LPByOwners(alloc ...Alloc) map[string]map[shared.AssetID]uint64 {
	lpByOwners := map[string]map[shared.AssetID]uint64{}
	for _, a := range alloc {
		if _, ok := lpByOwners[a.string]; !ok {
			lpByOwners[a.string] = map[shared.AssetID]uint64{}
		}
		lpByOwners[a.string][a.AssetID] = lpByOwners[a.string][a.AssetID] + a.uint64
	}
	return lpByOwners
}

func Test_EmissionsToOwners(t *testing.T) {
	lpByOwners := LPByOwners(
		Alloc{"A", "LP_X", 100},
	)
	emissionsByAsset := map[shared.AssetID]uint64{
		"LP_X": 1000,
	}
	lpTokensByAsset := map[shared.AssetID]uint64{
		"LP_X": 100,
	}
	emissionsByOwner := DistributeEmissionsToOwners(lpByOwners, emissionsByAsset, lpTokensByAsset)
	assert.EqualValues(t, map[string]map[string]uint64{"A": {"LP_X": 1000}}, emissionsByOwner)

	lpByOwners = LPByOwners(
		Alloc{"A", "LP_X", 100},
		Alloc{"B", "LP_X", 200},
	)
	lpTokensByAsset = map[shared.AssetID]uint64{"LP_X": 300}
	emissionsByOwner = DistributeEmissionsToOwners(lpByOwners, emissionsByAsset, lpTokensByAsset)
	assert.EqualValues(t, map[string]map[string]uint64{"A": {"LP_X": 334}, "B": {"LP_X": 666}}, emissionsByOwner)

	lpByOwners = LPByOwners(
		Alloc{"A", "LP_X", 100},
		Alloc{"B", "LP_X", 200},
		Alloc{"A", "LP_Y", 300},
	)
	emissionsByAsset = map[shared.AssetID]uint64{"LP_X": 1000, "LP_Y": 500}
	lpTokensByAsset = map[shared.AssetID]uint64{"LP_X": 300, "LP_Y": 300}
	emissionsByOwner = DistributeEmissionsToOwners(lpByOwners, emissionsByAsset, lpTokensByAsset)
	assert.EqualValues(t, map[string]map[string]uint64{"A": {"LP_X": 334, "LP_Y": 500}, "B": {"LP_X": 666}}, emissionsByOwner)

	// Test the case where one of the owners isn't qualified for *any* emissions, but round-robin calcs happen
	lpByOwners = LPByOwners(
		Alloc{"z", "LP_Z", 100},
		Alloc{"A", "LP_X", 100},
		Alloc{"B", "LP_X", 200},
		Alloc{"A", "LP_Y", 300},
	)
	emissionsByAsset = map[shared.AssetID]uint64{"LP_X": 1000, "LP_Y": 500}
	lpTokensByAsset = map[shared.AssetID]uint64{"LP_X": 300, "LP_Y": 300, "LP_Z": 500}
	emissionsByOwner = DistributeEmissionsToOwners(lpByOwners, emissionsByAsset, lpTokensByAsset)
	assert.EqualValues(t, map[string]map[string]uint64{"A": {"LP_X": 334, "LP_Y": 500}, "B": {"LP_X": 666}}, emissionsByOwner)
}

func makeValue(token string, amt uint64) compatibility.CompatibleValue {
	return compatibility.CompatibleValue(shared.ValueFromCoins(shared.Coin{AssetId: shared.AssetID(token), Amount: num.Uint64(amt)}))
}

func Test_EmissionsToEarnings(t *testing.T) {
	now := types.Date(time.Now().Format(types.DateFormat))
	program := utilities.SampleYieldProgram(500_000)
	ownerA := types.MultisigScript{Signature: &types.Signature{KeyHash: []byte("A")}}
	ownerB := types.MultisigScript{Signature: &types.Signature{KeyHash: []byte("B")}}
	ownerC := types.MultisigScript{Signature: &types.Signature{KeyHash: []byte("B")}}
	emissions, perOwnerTotal := EmissionsByOwnerToEarnings(now, program, map[string]map[string]uint64{
		"A": {"LP_X": 900, "LP_Y": 100},
		"B": {"LP_X": 1000, "LP_Y": 200, "LP_Z": 300},
		"C": {},
	}, map[string]types.MultisigScript{
		"A": ownerA,
		"B": ownerB,
		"C": ownerC,
	})
	assert.EqualValues(t, []types.Earning{
		{
			OwnerID: "A", Program: program.ID, Owner: ownerA, EarnedDate: now,
			Value: makeValue("Emitted", 1000),
			ValueByLPToken: map[string]compatibility.CompatibleValue{
				"LP_X": makeValue("Emitted", 900),
				"LP_Y": makeValue("Emitted", 100),
			},
		},
		{
			OwnerID: "B", Program: program.ID, Owner: ownerB, EarnedDate: now,
			Value: makeValue("Emitted", 1500),
			ValueByLPToken: map[string]compatibility.CompatibleValue{
				"LP_X": makeValue("Emitted", 1000),
				"LP_Y": makeValue("Emitted", 200),
				"LP_Z": makeValue("Emitted", 300),
			},
		},
	}, emissions)
	assert.EqualValues(t, map[string]uint64{
		"A": 1000,
		"B": 1500,
	}, perOwnerTotal)
}

func Test_Calculate_Earnings(t *testing.T) {
	seed := time.Now().UnixNano()
	rand.Seed(seed)
	numPositions := rand.Intn(3000) + 1000
	numOwners := rand.Intn(numPositions-1) + 1
	numPools := rand.Intn(300) + 100
	program := utilities.SampleYieldProgram(500_000_000_000)
	program.ConsecutiveDelegationWindow = 3
	previousDays := []CalculationOutputs{}
	for i := 0; i < 10; i++ {
		program, calcOutputs, err := Random_Calc_Earnings(program, numPositions, numOwners, numPools, previousDays)
		assert.Nil(t, err)
		totalEarnings := uint64(0)
		previousDays = append(previousDays, calcOutputs)
		if len(previousDays) > program.ConsecutiveDelegationWindow-1 {
			previousDays = previousDays[1:]
		}
		for _, e := range calcOutputs.Earnings {
			totalEarnings += shared.Value(e.Value).AssetAmount(program.EmittedAsset).Uint64()
		}
		totalFixedEmissions := uint64(0)
		for _, amt := range program.FixedEmissions {
			totalFixedEmissions += amt
		}
		everyEligiblePoolReceivingFixedEmissions := true
		for pool := range calcOutputs.PoolsEligibleForEmissions {
			if _, ok := program.FixedEmissions[pool]; !ok {
				everyEligiblePoolReceivingFixedEmissions = false
				break
			}
		}
		// Checking the results is a bit nuanced, because of the randomly generated cases
		if totalEarnings == 0 {
			// If the total is zero, then we better have zero earnings
			assert.Empty(t, calcOutputs.Earnings)
		} else if everyEligiblePoolReceivingFixedEmissions || program.EmissionCap > 0 {
			// If every eligible pool was assigned fixed emissions, then we had
			// no pools leftover to distribute the remaining emissions to, so we can only check that it's less than the daily emissions
			// this is also true if we have an emission cap, since that may have truncated some rewards
			assert.LessOrEqual(t, totalEarnings, program.DailyEmission)
			// If we do have an emission cap, then check that each pool receiving emissions got correctly capped
			if program.EmissionCap > 0 {
				for _, amt := range calcOutputs.EmissionsByPool {
					assert.LessOrEqual(t, amt, program.EmissionCap)
				}
			}
			// And if every eligible pool was already assigned fixed emissions, check that that total is at least correct
			if everyEligiblePoolReceivingFixedEmissions {
				assert.Equal(t, totalEarnings, totalFixedEmissions)
			}
		} else {
			// Otherwise, if we didn't have a cap on emissions, *and* there was at least one eligible pool to soak up the remainder
			// so in that case, every coin should be accounted for
			assert.Equal(t, totalEarnings, program.DailyEmission)
		}
	}
}

func Benchmark_Calculate_Earnings(b *testing.B) {
	program := utilities.SampleYieldProgram(500_000_000_000)
	for i := 0; i < b.N; i++ {
		numPositions := 100_000
		numOwners := 90_000
		numPools := 1500
		Random_Calc_Earnings(program, numPositions, numOwners, numPools, []CalculationOutputs{})
	}
}

func Random_Calc_Earnings(program types.YieldProgram, numPositions, numOwners, numPools int, previousDays []CalculationOutputs) (types.YieldProgram, CalculationOutputs, error) {
	now := types.Date(time.Now().Format(types.DateFormat))
	var positions []types.Position
	pools := utilities.MockLookup{}

	lockedByPool := map[int]uint64{}

	for i := 0; i < numPositions; i++ {
		numSundae := rand.Int63n(50_000_000_000_000)
		owner := fmt.Sprintf("Owner_%v", rand.Intn(numOwners))
		position := types.Position{
			OwnerID: owner,
			Owner:   types.MultisigScript{Signature: &types.Signature{KeyHash: []byte(owner)}},
			Value:   compatibility.CompatibleValue(shared.ValueFromCoins(shared.Coin{AssetId: program.StakedAsset, Amount: num.Int64(numSundae)})),
		}
		numDelegations := rand.Intn(40)
		for j := 0; j < numDelegations; j++ {
			poolIdent := fmt.Sprintf("Pool_%v", rand.Intn(numPools))
			weight := uint32(rand.Intn(50_000))
			forProgram := rand.Int()%4 < 3
			if forProgram {
				position.Delegation = append(position.Delegation, types.Delegation{Program: program.ID, PoolIdent: poolIdent, Weight: weight})
			} else {
				position.Delegation = append(position.Delegation, types.Delegation{Program: "OTHER PROGRAM", PoolIdent: poolIdent, Weight: weight})
			}
		}

		numLP := rand.Intn(15)
		for j := 0; j < numLP; j++ {
			pool := rand.Intn(numPools)
			lp := shared.AssetID(fmt.Sprintf("LP_%v", pool))
			amt := rand.Int63n(30_000_000)
			value := shared.Value(position.Value)
			value.AddAsset(shared.Coin{AssetId: lp, Amount: num.Int64(amt)})

			position.Value = compatibility.CompatibleValue(value)
			lockedByPool[pool] += uint64(amt)
		}
		numOtherTokens := rand.Intn(5)
		for j := 0; j < numOtherTokens; j++ {
			token := shared.AssetID(fmt.Sprintf("Random_%v", numOtherTokens))
			value := shared.Value(position.Value)
			value.AddAsset(shared.Coin{AssetId: token, Amount: num.Int64(rand.Int63n(30_000_000_000))})

			position.Value = compatibility.CompatibleValue(value)
		}

		positions = append(positions, position)
	}

	program.FixedEmissions = map[string]uint64{}
	for i := 0; i < numPools; i++ {
		poolIdent := fmt.Sprintf("Pool_%v", i)
		pools[poolIdent] = types.Pool{
			PoolIdent:     poolIdent,
			TotalLPTokens: lockedByPool[i] + uint64(rand.Int63n(100_000_000_000)),
			LPAsset:       shared.AssetID(fmt.Sprintf("LP_%v", i)),
		}
		if rand.Intn(100) == 0 && len(program.FixedEmissions) < 10 {
			program.FixedEmissions[poolIdent] = program.DailyEmission / uint64(numPools)
		}
	}
	if rand.Intn(10) == 0 {
		program.EmissionCap = program.DailyEmission / 5
	}

	window := []CalculationOutputs{}
	for i := 0; i < program.ConsecutiveDelegationWindow; i++ {
		if len(previousDays) > i {
			window = append(window, previousDays[len(previousDays)-1-i])
		}
	}

	results, err := CalculateEarnings(context.Background(), now, 0, 86400, program, window, positions, pools)
	return program, results, err
}
