package calculation

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/SundaeSwap-finance/ogmigo/ouroboros/chainsync"
	"github.com/SundaeSwap-finance/ogmigo/ouroboros/chainsync/num"
	"github.com/SundaeSwap-finance/sundae-yield-v2/types"
	"github.com/tj/assert"
)

func sampleProgram(emissions uint64) types.Program {
	return types.Program{
		ID:                  "Test",
		FirstDailyRewards:   "2001-01-01",
		LastDailyRewards:    "2099-01-01", // If this code is still in use in 2099, call the police (after updating tests)
		StakedAsset:         chainsync.AssetID("Staked"),
		MinLPIntegerPercent: 1,
		EmittedAsset:        chainsync.AssetID("Emitted"),
		DailyEmission:       emissions,
	}
}

func samplePosition(owner string, staked int64, delegations ...types.Delegation) types.Position {
	return types.Position{
		OwnerID:   owner,
		Slot:      0,
		SpentSlot: 0,
		Value: chainsync.Value{
			Coins: num.Int64(0),
			Assets: map[chainsync.AssetID]num.Int{
				chainsync.AssetID("Staked"): num.Int64(staked),
			},
		},
		Delegation: delegations,
	}
}

type MockLookup map[string]types.Pool

func (m MockLookup) PoolByIdent(ctx context.Context, poolIdent string) (types.Pool, error) {
	if pool, ok := m[poolIdent]; ok {
		return pool, nil
	}
	return types.Pool{}, fmt.Errorf("pool not found")
}
func (m MockLookup) PoolByLPToken(ctx context.Context, lpToken chainsync.AssetID) (types.Pool, error) {
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
func (m MockLookup) IsLPToken(assetId chainsync.AssetID) bool {
	return strings.HasPrefix(assetId.String(), "LP_")
}
func (m MockLookup) LPTokenToPoolIdent(lpToken chainsync.AssetID) (string, error) {
	if pool, err := m.PoolByLPToken(context.Background(), lpToken); err != nil {
		return "", err
	} else {
		return pool.PoolIdent, nil
	}
}

func Test_TotalDelegations(t *testing.T) {
	program := sampleProgram(500000_000_000)

	// The simplest case
	positions := []types.Position{
		samplePosition("Me", 100_000, types.Delegation{Program: program.ID, PoolIdent: "01", Weight: 1}),
	}
	delegationsByPool, totalDelegations, err := CalculateTotalDelegations(context.Background(), program, positions, MockLookup{})
	assert.Nil(t, err)
	assert.EqualValues(t, 100_000, delegationsByPool["01"])
	assert.EqualValues(t, 100_000, totalDelegations)

	positions = []types.Position{
		samplePosition("Me", 100_000),
	}
	delegationsByPool, totalDelegations, err = CalculateTotalDelegations(context.Background(), program, positions, MockLookup{})
	assert.Nil(t, err)
	assert.EqualValues(t, 100_000, delegationsByPool[""])
	assert.EqualValues(t, 100_000, totalDelegations)

	// Should split evenly between delegations
	positions = []types.Position{
		samplePosition("Me", 100_000, types.Delegation{Program: program.ID, PoolIdent: "01", Weight: 1}, types.Delegation{Program: program.ID, PoolIdent: "02", Weight: 1}),
	}
	delegationsByPool, totalDelegations, err = CalculateTotalDelegations(context.Background(), program, positions, MockLookup{})
	assert.Nil(t, err)
	assert.EqualValues(t, 50_000, delegationsByPool["01"])
	assert.EqualValues(t, 50_000, delegationsByPool["02"])
	assert.EqualValues(t, 100_000, totalDelegations)

	// Should handle bankers rounding
	positions = []types.Position{
		samplePosition("Me", 100_000, types.Delegation{Program: program.ID, PoolIdent: "01", Weight: 1}, types.Delegation{Program: program.ID, PoolIdent: "02", Weight: 2}),
	}
	delegationsByPool, totalDelegations, err = CalculateTotalDelegations(context.Background(), program, positions, MockLookup{})
	assert.Nil(t, err)
	assert.EqualValues(t, 33_334, delegationsByPool["01"])
	assert.EqualValues(t, 66_666, delegationsByPool["02"])
	assert.EqualValues(t, 100_000, totalDelegations)

	// Should handle multiple positions
	positions = []types.Position{
		samplePosition("Me", 100_000, types.Delegation{Program: program.ID, PoolIdent: "01", Weight: 1}, types.Delegation{Program: program.ID, PoolIdent: "02", Weight: 1}),
		samplePosition("Me", 200_000, types.Delegation{Program: program.ID, PoolIdent: "02", Weight: 1}, types.Delegation{Program: program.ID, PoolIdent: "03", Weight: 1}),
	}
	delegationsByPool, totalDelegations, err = CalculateTotalDelegations(context.Background(), program, positions, MockLookup{})
	assert.Nil(t, err)
	assert.EqualValues(t, 50_000, delegationsByPool["01"])
	assert.EqualValues(t, 150_000, delegationsByPool["02"])
	assert.EqualValues(t, 100_000, delegationsByPool["03"])
	assert.EqualValues(t, 300_000, totalDelegations)

	// Should handle positions with LP tokens
	pools := MockLookup{
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
		samplePosition("Me", 100_000, types.Delegation{Program: program.ID, PoolIdent: "01", Weight: 1}, types.Delegation{Program: program.ID, PoolIdent: "02", Weight: 1}),
	}
	positions[0].Value.Assets["LP_01"] = num.Int64(50_000)
	delegationsByPool, totalDelegations, err = CalculateTotalDelegations(context.Background(), program, positions, pools)
	assert.Nil(t, err)
	assert.EqualValues(t, 75_000, delegationsByPool["01"])
	assert.EqualValues(t, 75_000, delegationsByPool["02"])
	assert.EqualValues(t, 150_000, totalDelegations)

	// Should handle delegations to multiple programs

	positions = []types.Position{
		samplePosition("Me", 100_000, types.Delegation{Program: program.ID, PoolIdent: "01", Weight: 1}, types.Delegation{Program: "OTHER_PROGRAM", PoolIdent: "99", Weight: 100}, types.Delegation{Program: program.ID, PoolIdent: "02", Weight: 1}),
		samplePosition("Me", 200_000, types.Delegation{Program: program.ID, PoolIdent: "02", Weight: 1}, types.Delegation{Program: program.ID, PoolIdent: "03", Weight: 1}),
	}

	delegationsByPool, totalDelegations, err = CalculateTotalDelegations(context.Background(), program, positions, MockLookup{})
	assert.Nil(t, err)
	assert.EqualValues(t, 50_000, delegationsByPool["01"])
	assert.EqualValues(t, 150_000, delegationsByPool["02"])
	assert.EqualValues(t, 100_000, delegationsByPool["03"])
	assert.EqualValues(t, 300_000, totalDelegations)
}

func Test_SummationConstraint(t *testing.T) {
	program := sampleProgram(500000_000_000)

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
		samplePosition("Me", initialSundae, delegations...),
	}
	delegationsByPool, totalDelegation, err := CalculateTotalDelegations(context.Background(), program, positions, MockLookup{})
	assert.Nil(t, err)
	actualSum := uint64(0)
	for _, s := range delegationsByPool {
		actualSum += s
	}
	assert.EqualValues(t, initialSundae, actualSum)
	assert.EqualValues(t, totalDelegation, actualSum)
}

func Test_SumWindow(t *testing.T) {
	program := sampleProgram(500000_000_000)
	program.ConsecutiveDelegationWindow = 1
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
	program := sampleProgram(500_000)
	poolA := types.Pool{PoolIdent: "A", AssetA: "A", AssetB: "X", TotalLPTokens: 1500}
	poolB := types.Pool{PoolIdent: "B", AssetA: "B", AssetB: "X", TotalLPTokens: 1500}
	poolC := types.Pool{PoolIdent: "C", AssetA: "C", AssetB: "Y", TotalLPTokens: 1500}

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

	program.EligiblePools = []string{"A"}
	assertQualified(poolA, 500)
	assertDisqualified(poolB, 500)
	program.EligiblePools = nil

	program.EligibleAssets = []chainsync.AssetID{"A"}
	assertQualified(poolA, 500)
	assertDisqualified(poolB, 500)
	program.EligibleAssets = nil

	program.EligibleAssets = []chainsync.AssetID{"X"}
	assertQualified(poolA, 500)
	assertQualified(poolB, 500)
	assertDisqualified(poolC, 500)
	program.EligibleAssets = nil

	program.EligiblePairs = []struct {
		AssetA chainsync.AssetID
		AssetB chainsync.AssetID
	}{
		{AssetA: "A", AssetB: "X"},
	}
	assertQualified(poolA, 500)
	assertDisqualified(poolB, 500)
	assertDisqualified(poolC, 500)
	program.EligiblePairs = nil

	program.DisqualifiedPools = []string{"A"}
	assertDisqualified(poolA, 500)
	assertQualified(poolB, 500)
	program.DisqualifiedPools = nil

	program.DisqualifiedAssets = []chainsync.AssetID{"X"}
	assertDisqualified(poolA, 500)
	assertDisqualified(poolB, 500)
	assertQualified(poolC, 500)
	program.DisqualifiedAssets = nil

	program.DisqualifiedPairs = []struct {
		AssetA chainsync.AssetID
		AssetB chainsync.AssetID
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
		{OwnerID: "A", Value: chainsync.Value{Assets: map[chainsync.AssetID]num.Int{"LP_X": num.Uint64(100)}}},
		{OwnerID: "B", Value: chainsync.Value{Assets: map[chainsync.AssetID]num.Int{"LP_X": num.Uint64(200)}}},
		{OwnerID: "C", Value: chainsync.Value{Assets: map[chainsync.AssetID]num.Int{"LP_Y": num.Uint64(500)}}},
	}
	pools := MockLookup{
		"X": {PoolIdent: "X", LPAsset: "LP_X", TotalLPTokens: 500, AssetAQuantity: 1000},
		"Y": {PoolIdent: "Y", LPAsset: "LP_Y", TotalLPTokens: 1000, AssetAQuantity: 100},
	}
	lockedLP, totalLP, totalValueByPool, totalValue, err := CalculateTotalLPAtSnapshot(context.Background(), 0, positions, pools)
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
	program := sampleProgram(500_000)
	program.MaxPoolCount = 2
	program.MaxPoolIntegerPercent = 100
	pools := MockLookup{"A": types.Pool{PoolIdent: "A"}, "B": types.Pool{PoolIdent: "B"}, "C": types.Pool{PoolIdent: "C"}}
	selectedPools, err := SelectPoolsForEmission(context.Background(), program, map[string]uint64{
		"A": 100,
		"B": 200,
		"C": 300,
	}, pools)
	assert.Nil(t, err)
	assert.EqualValues(t, map[string]uint64{"C": 300, "B": 200}, selectedPools)

	program.MaxPoolIntegerPercent = 30
	selectedPools, err = SelectPoolsForEmission(context.Background(), program, map[string]uint64{
		"A": 100,
		"B": 101,
		"C": 202,
	}, pools)
	assert.Nil(t, err)
	assert.EqualValues(t, map[string]uint64{"C": 202}, selectedPools)

	program.MaxPoolCount = 10
	program.MaxPoolIntegerPercent = 33
	pools = MockLookup{
		"A": types.Pool{},
		"B": types.Pool{},
		"C": types.Pool{},
		"D": types.Pool{},
		"E": types.Pool{},
		"F": types.Pool{},
	}
	selectedPools, err = SelectPoolsForEmission(context.Background(), program, map[string]uint64{
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
	program := sampleProgram(500_000)
	program.NepotismPools = []string{"B"}
	program.MaxPoolCount = 2
	program.MaxPoolIntegerPercent = 20
	selectedPools, err := SelectPoolsForEmission(context.Background(), program, map[string]uint64{
		"A": 50,
		"B": 100,
		"C": 200,
		"D": 300,
	}, MockLookup{"A": types.Pool{}, "B": types.Pool{}, "C": types.Pool{}, "D": types.Pool{}})
	assert.Nil(t, err)
	assert.EqualValues(t, map[string]uint64{"D": 300, "B": 100}, selectedPools)
}

func Test_EmissionsToPools(t *testing.T) {
	program := sampleProgram(500_000_000_000)
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

	program.EmissionCap = 200_000_000_000
	emissions = DistributeEmissionsToPools(program, map[string]uint64{
		"A": 1000,
		"B": 2000,
	})
	assert.EqualValues(t, map[string]uint64{"A": 166_333_333_333, "B": 200_000_000_000, "C": 1_000_000_000}, emissions)
}

func Test_OwnerByLPAndAsset(t *testing.T) {
	pools := MockLookup{
		"X": {PoolIdent: "X", LPAsset: "LP_X"},
		"Y": {PoolIdent: "Y", LPAsset: "LP_Y"},
	}
	byOwner, byAsset := TotalLPDaysByOwnerAndAsset([]types.Position{
		{OwnerID: "A", Value: chainsync.Value{Assets: map[chainsync.AssetID]num.Int{"LP_X": num.Uint64(100)}}},
	}, pools, 0, 86400)
	assert.EqualValues(t, map[string]map[chainsync.AssetID]uint64{
		"A": {"LP_X": 100},
	}, byOwner)
	assert.EqualValues(t, map[chainsync.AssetID]uint64{
		"LP_X": 100,
	}, byAsset)

	byOwner, byAsset = TotalLPDaysByOwnerAndAsset([]types.Position{
		{OwnerID: "A", Value: chainsync.Value{Assets: map[chainsync.AssetID]num.Int{"LP_X": num.Uint64(100)}}},
		{OwnerID: "B", Value: chainsync.Value{Assets: map[chainsync.AssetID]num.Int{"LP_X": num.Uint64(200)}}},
	}, pools, 0, 86400)
	assert.EqualValues(t, map[string]map[chainsync.AssetID]uint64{
		"A": {"LP_X": 100},
		"B": {"LP_X": 200},
	}, byOwner)
	assert.EqualValues(t, map[chainsync.AssetID]uint64{
		"LP_X": 300,
	}, byAsset)

	byOwner, byAsset = TotalLPDaysByOwnerAndAsset([]types.Position{
		{OwnerID: "A", Value: chainsync.Value{Assets: map[chainsync.AssetID]num.Int{"LP_X": num.Uint64(100)}}},
		{OwnerID: "B", Value: chainsync.Value{Assets: map[chainsync.AssetID]num.Int{"LP_X": num.Uint64(200)}}},
		{OwnerID: "B", Value: chainsync.Value{Assets: map[chainsync.AssetID]num.Int{"LP_X": num.Uint64(300)}}},
	}, pools, 0, 86400)
	assert.EqualValues(t, map[string]map[chainsync.AssetID]uint64{
		"A": {"LP_X": 100},
		"B": {"LP_X": 500},
	}, byOwner)
	assert.EqualValues(t, map[chainsync.AssetID]uint64{
		"LP_X": 600,
	}, byAsset)

	byOwner, byAsset = TotalLPDaysByOwnerAndAsset([]types.Position{
		{OwnerID: "A", Value: chainsync.Value{Assets: map[chainsync.AssetID]num.Int{"LP_X": num.Uint64(100)}}},
		{OwnerID: "B", Value: chainsync.Value{Assets: map[chainsync.AssetID]num.Int{"LP_X": num.Uint64(200), "LP_Y": num.Uint64(150)}}},
		{OwnerID: "B", Value: chainsync.Value{Assets: map[chainsync.AssetID]num.Int{"LP_X": num.Uint64(300)}}},
	}, pools, 0, 86400)
	assert.EqualValues(t, map[string]map[chainsync.AssetID]uint64{
		"A": {"LP_X": 100},
		"B": {"LP_X": 500, "LP_Y": 150},
	}, byOwner)
	assert.EqualValues(t, map[chainsync.AssetID]uint64{
		"LP_X": 600,
		"LP_Y": 150,
	}, byAsset)

	// Test the time-weighting bits
	byOwner, byAsset = TotalLPDaysByOwnerAndAsset([]types.Position{
		// Half day
		{OwnerID: "A", Slot: 143200, Value: chainsync.Value{Assets: map[chainsync.AssetID]num.Int{"LP_X": num.Uint64(100)}}},
		// Quarter day, with rounding down
		{OwnerID: "B", Slot: 143200, SpentTransaction: "A", SpentSlot: 164800, Value: chainsync.Value{Assets: map[chainsync.AssetID]num.Int{"LP_X": num.Uint64(200), "LP_Y": num.Uint64(150)}}},
		// Lockup before the day starts
		{OwnerID: "C", Slot: 12, Value: chainsync.Value{Assets: map[chainsync.AssetID]num.Int{"LP_X": num.Uint64(300)}}},
		// Consecutive positions, constituting half, plus after day ends
		{OwnerID: "D", Slot: 143200, SpentTransaction: "B", SpentSlot: 164800, Value: chainsync.Value{Assets: map[chainsync.AssetID]num.Int{"LP_X": num.Uint64(300)}}},
		{OwnerID: "D", Slot: 164800, SpentTransaction: "C", SpentSlot: 264800, Value: chainsync.Value{Assets: map[chainsync.AssetID]num.Int{"LP_X": num.Uint64(300)}}},
	}, pools, 100000, 186400)
	assert.EqualValues(t, map[string]map[chainsync.AssetID]uint64{
		"A": {"LP_X": 50},
		"B": {"LP_X": 50, "LP_Y": 37},
		"C": {"LP_X": 300},
		"D": {"LP_X": 150},
	}, byOwner)
	assert.EqualValues(t, map[chainsync.AssetID]uint64{
		"LP_X": 550,
		"LP_Y": 37,
	}, byAsset)
}

func Test_RegroupByAsset(t *testing.T) {
	pools := MockLookup{
		"X": {PoolIdent: "X", LPAsset: "LP_X"},
		"Y": {PoolIdent: "Y", LPAsset: "LP_Y"},
	}
	byAsset, err := RegroupByAsset(context.Background(), map[string]uint64{"X": 100}, pools)
	assert.Nil(t, err)
	assert.EqualValues(t, map[chainsync.AssetID]uint64{"LP_X": 100}, byAsset)

	byAsset, err = RegroupByAsset(context.Background(), map[string]uint64{"X": 100, "Y": 200}, pools)
	assert.Nil(t, err)
	assert.EqualValues(t, map[chainsync.AssetID]uint64{"LP_X": 100, "LP_Y": 200}, byAsset)
}

type Alloc struct {
	string
	chainsync.AssetID
	uint64
}

func LPByOwners(alloc ...Alloc) map[string]map[chainsync.AssetID]uint64 {
	lpByOwners := map[string]map[chainsync.AssetID]uint64{}
	for _, a := range alloc {
		if _, ok := lpByOwners[a.string]; !ok {
			lpByOwners[a.string] = map[chainsync.AssetID]uint64{}
		}
		lpByOwners[a.string][a.AssetID] = lpByOwners[a.string][a.AssetID] + a.uint64
	}
	return lpByOwners
}

func Test_EmissionsToOwners(t *testing.T) {
	lpByOwners := LPByOwners(
		Alloc{"A", "LP_X", 100},
	)
	emissionsByAsset := map[chainsync.AssetID]uint64{
		"LP_X": 1000,
	}
	lpTokensByAsset := map[chainsync.AssetID]uint64{
		"LP_X": 100,
	}
	emissionsByOwner := DistributeEmissionsToOwners(lpByOwners, emissionsByAsset, lpTokensByAsset)
	assert.EqualValues(t, map[string]map[string]uint64{"A": {"LP_X": 1000}}, emissionsByOwner)

	lpByOwners = LPByOwners(
		Alloc{"A", "LP_X", 100},
		Alloc{"B", "LP_X", 200},
	)
	lpTokensByAsset = map[chainsync.AssetID]uint64{"LP_X": 300}
	emissionsByOwner = DistributeEmissionsToOwners(lpByOwners, emissionsByAsset, lpTokensByAsset)
	assert.EqualValues(t, map[string]map[string]uint64{"A": {"LP_X": 334}, "B": {"LP_X": 666}}, emissionsByOwner)

	lpByOwners = LPByOwners(
		Alloc{"A", "LP_X", 100},
		Alloc{"B", "LP_X", 200},
		Alloc{"A", "LP_Y", 300},
	)
	emissionsByAsset = map[chainsync.AssetID]uint64{"LP_X": 1000, "LP_Y": 500}
	lpTokensByAsset = map[chainsync.AssetID]uint64{"LP_X": 300, "LP_Y": 300}
	emissionsByOwner = DistributeEmissionsToOwners(lpByOwners, emissionsByAsset, lpTokensByAsset)
	assert.EqualValues(t, map[string]map[string]uint64{"A": {"LP_X": 334, "LP_Y": 500}, "B": {"LP_X": 666}}, emissionsByOwner)

	// Test the case where one of the owners isn't qualified for *any* emissions, but round-robin calcs happen
	lpByOwners = LPByOwners(
		Alloc{"z", "LP_Z", 100},
		Alloc{"A", "LP_X", 100},
		Alloc{"B", "LP_X", 200},
		Alloc{"A", "LP_Y", 300},
	)
	emissionsByAsset = map[chainsync.AssetID]uint64{"LP_X": 1000, "LP_Y": 500}
	lpTokensByAsset = map[chainsync.AssetID]uint64{"LP_X": 300, "LP_Y": 300, "LP_Z": 500}
	emissionsByOwner = DistributeEmissionsToOwners(lpByOwners, emissionsByAsset, lpTokensByAsset)
	assert.EqualValues(t, map[string]map[string]uint64{"A": {"LP_X": 334, "LP_Y": 500}, "B": {"LP_X": 666}}, emissionsByOwner)
}

func makeValue(token string, amt uint64) chainsync.Value {
	return chainsync.Value{Assets: map[chainsync.AssetID]num.Int{chainsync.AssetID(token): num.Uint64(amt)}}
}

func Test_EmissionsToEarnings(t *testing.T) {
	now := types.Date(time.Now().Format(types.DateFormat))
	program := sampleProgram(500_000)
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
			ValueByLPToken: map[string]chainsync.Value{
				"LP_X": makeValue("Emitted", 900),
				"LP_Y": makeValue("Emitted", 100),
			},
		},
		{
			OwnerID: "B", Program: program.ID, Owner: ownerB, EarnedDate: now,
			Value: makeValue("Emitted", 1500),
			ValueByLPToken: map[string]chainsync.Value{
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
	seed := int64(1697588422052097852) //time.Now().UnixNano()
	rand.Seed(seed)
	numPositions := rand.Intn(3000) + 1000
	numOwners := rand.Intn(numPositions-1) + 1
	numPools := rand.Intn(300) + 100
	program := sampleProgram(500_000_000_000)
	program.ConsecutiveDelegationWindow = 3
	previousDays := []CalculationOutputs{}
	for i := 0; i < 10; i++ {
		program, calcOutputs, err := Random_Calc_Earnings(program, numPositions, numOwners, numPools, previousDays)
		assert.Nil(t, err)
		total := uint64(0)
		previousDays = append(previousDays, calcOutputs)
		for _, e := range calcOutputs.Earnings {
			total += e.Value.Assets[program.EmittedAsset].Uint64()
		}
		if total == 0 {
			assert.Empty(t, calcOutputs.Earnings)
		} else if program.EmissionCap == 0 {
			if total != program.DailyEmission {
				fmt.Printf("seed: %v\n", seed)
			}
			assert.Equal(t, total, program.DailyEmission)
		} else {
			assert.LessOrEqual(t, total, program.DailyEmission)
			for _, amt := range calcOutputs.EmissionsByPool {
				assert.LessOrEqual(t, amt, program.EmissionCap)
			}
		}
	}
}

func Benchmark_Calculate_Earnings(b *testing.B) {
	program := sampleProgram(500_000_000_000)
	for i := 0; i < b.N; i++ {
		numPositions := 100_000
		numOwners := 90_000
		numPools := 1500
		Random_Calc_Earnings(program, numPositions, numOwners, numPools, []CalculationOutputs{})
	}
}

func Random_Calc_Earnings(program types.Program, numPositions, numOwners, numPools int, previousDays []CalculationOutputs) (types.Program, CalculationOutputs, error) {
	now := types.Date(time.Now().Format(types.DateFormat))
	var positions []types.Position
	pools := MockLookup{}

	lockedByPool := map[int]uint64{}

	for i := 0; i < numPositions; i++ {
		numSundae := rand.Int63n(50_000_000_000_000)
		owner := fmt.Sprintf("Owner_%v", rand.Intn(numOwners))
		position := types.Position{
			OwnerID: owner,
			Owner:   types.MultisigScript{Signature: &types.Signature{KeyHash: []byte(owner)}},
			Value: chainsync.Value{
				Assets: map[chainsync.AssetID]num.Int{
					program.StakedAsset: num.Int64(numSundae),
				},
			},
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
			lp := chainsync.AssetID(fmt.Sprintf("LP_%v", pool))
			amt := rand.Int63n(30_000_000)
			position.Value.Assets[lp] = num.Int64(amt)
			lockedByPool[pool] += uint64(amt)
		}
		numOtherTokens := rand.Intn(5)
		for j := 0; j < numOtherTokens; j++ {
			token := chainsync.AssetID(fmt.Sprintf("Random_%v", numOtherTokens))
			position.Value.Assets[token] = num.Int64(rand.Int63n(30_000_000_000))
		}

		positions = append(positions, position)
	}

	program.FixedEmissions = map[string]uint64{}
	for i := 0; i < numPools; i++ {
		poolIdent := fmt.Sprintf("Pool_%v", i)
		pools[poolIdent] = types.Pool{
			PoolIdent:     poolIdent,
			TotalLPTokens: lockedByPool[i] + uint64(rand.Int63n(100_000_000_000)),
			LPAsset:       chainsync.AssetID(fmt.Sprintf("LP_%v", i)),
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
