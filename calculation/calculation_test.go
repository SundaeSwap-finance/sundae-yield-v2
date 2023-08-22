package calculation

import (
	"fmt"
	"math/rand"
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

func Test_TotalDelegations(t *testing.T) {
	program := sampleProgram(500000_000_000)

	// The simplest case
	positions := []types.Position{
		samplePosition("Me", 100_000, types.Delegation{Program: program.ID, PoolIdent: "01", Weight: 1}),
	}
	delegationsByPool, totalDelegations := CalculateTotalDelegations(program, positions, map[string]types.Pool{})
	assert.EqualValues(t, 100_000, delegationsByPool["01"])
	assert.EqualValues(t, 100_000, totalDelegations)

	positions = []types.Position{
		samplePosition("Me", 100_000),
	}
	delegationsByPool, totalDelegations = CalculateTotalDelegations(program, positions, map[string]types.Pool{})
	assert.EqualValues(t, 100_000, delegationsByPool[""])
	assert.EqualValues(t, 100_000, totalDelegations)

	// Should split evenly between delegations
	positions = []types.Position{
		samplePosition("Me", 100_000, types.Delegation{Program: program.ID, PoolIdent: "01", Weight: 1}, types.Delegation{Program: program.ID, PoolIdent: "02", Weight: 1}),
	}
	delegationsByPool, totalDelegations = CalculateTotalDelegations(program, positions, map[string]types.Pool{})
	assert.EqualValues(t, 50_000, delegationsByPool["01"])
	assert.EqualValues(t, 50_000, delegationsByPool["02"])
	assert.EqualValues(t, 100_000, totalDelegations)

	// Should handle bankers rounding
	positions = []types.Position{
		samplePosition("Me", 100_000, types.Delegation{Program: program.ID, PoolIdent: "01", Weight: 1}, types.Delegation{Program: program.ID, PoolIdent: "02", Weight: 2}),
	}
	delegationsByPool, totalDelegations = CalculateTotalDelegations(program, positions, map[string]types.Pool{})
	assert.EqualValues(t, 33_334, delegationsByPool["01"])
	assert.EqualValues(t, 66_666, delegationsByPool["02"])
	assert.EqualValues(t, 100_000, totalDelegations)

	// Should handle multiple positions
	positions = []types.Position{
		samplePosition("Me", 100_000, types.Delegation{Program: program.ID, PoolIdent: "01", Weight: 1}, types.Delegation{Program: program.ID, PoolIdent: "02", Weight: 1}),
		samplePosition("Me", 200_000, types.Delegation{Program: program.ID, PoolIdent: "02", Weight: 1}, types.Delegation{Program: program.ID, PoolIdent: "03", Weight: 1}),
	}
	delegationsByPool, totalDelegations = CalculateTotalDelegations(program, positions, map[string]types.Pool{})
	assert.EqualValues(t, 50_000, delegationsByPool["01"])
	assert.EqualValues(t, 150_000, delegationsByPool["02"])
	assert.EqualValues(t, 100_000, delegationsByPool["03"])
	assert.EqualValues(t, 300_000, totalDelegations)

	// Should handle positions with LP tokens
	pools := map[string]types.Pool{
		"01": {
			PoolIdent:      "01",
			TotalLPTokens:  100_000,
			LPAsset:        "LP",
			AssetA:         "",
			AssetB:         program.StakedAsset,
			AssetAQuantity: 200_000,
			AssetBQuantity: 100_000,
		},
	}
	positions = []types.Position{
		samplePosition("Me", 100_000, types.Delegation{Program: program.ID, PoolIdent: "01", Weight: 1}, types.Delegation{Program: program.ID, PoolIdent: "02", Weight: 1}),
	}
	positions[0].Value.Assets["LP"] = num.Int64(50_000)
	delegationsByPool, totalDelegations = CalculateTotalDelegations(program, positions, pools)
	assert.EqualValues(t, 75_000, delegationsByPool["01"])
	assert.EqualValues(t, 75_000, delegationsByPool["02"])
	assert.EqualValues(t, 150_000, totalDelegations)

	// Should handle delegations to multiple programs

	positions = []types.Position{
		samplePosition("Me", 100_000, types.Delegation{Program: program.ID, PoolIdent: "01", Weight: 1}, types.Delegation{Program: "OTHER_PROGRAM", PoolIdent: "99", Weight: 100}, types.Delegation{Program: program.ID, PoolIdent: "02", Weight: 1}),
		samplePosition("Me", 200_000, types.Delegation{Program: program.ID, PoolIdent: "02", Weight: 1}, types.Delegation{Program: program.ID, PoolIdent: "03", Weight: 1}),
	}

	delegationsByPool, totalDelegations = CalculateTotalDelegations(program, positions, map[string]types.Pool{})
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
	delegationsByPool, totalDelegation := CalculateTotalDelegations(program, positions, map[string]types.Pool{})
	actualSum := uint64(0)
	for _, s := range delegationsByPool {
		actualSum += s
	}
	assert.EqualValues(t, initialSundae, actualSum)
	assert.EqualValues(t, totalDelegation, actualSum)
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
	poolA := types.Pool{PoolIdent: "A", TotalLPTokens: 1500}
	poolB := types.Pool{PoolIdent: "B", TotalLPTokens: 1500}
	qualified, _ := isPoolQualified(program, poolA, 150)
	assert.True(t, qualified)
	qualified, _ = isPoolQualified(program, poolA, 500)
	assert.True(t, qualified)
	qualified, _ = isPoolQualified(program, poolA, 10)
	assert.False(t, qualified)

	program.EligiblePools = []string{"A"}
	qualified, _ = isPoolQualified(program, poolA, 500)
	assert.True(t, qualified)
	qualified, _ = isPoolQualified(program, poolB, 500)
	assert.False(t, qualified)
	program.EligiblePools = nil

	program.DisqualifiedPools = []string{"A"}
	qualified, _ = isPoolQualified(program, poolA, 500)
	assert.False(t, qualified)
	qualified, _ = isPoolQualified(program, poolB, 500)
	assert.True(t, qualified)
}

func Test_CalculateTotalLP(t *testing.T) {
	positions := []types.Position{
		{OwnerID: "A", Value: chainsync.Value{Assets: map[chainsync.AssetID]num.Int{"LP_X": num.Uint64(100)}}},
		{OwnerID: "B", Value: chainsync.Value{Assets: map[chainsync.AssetID]num.Int{"LP_X": num.Uint64(200)}}},
		{OwnerID: "C", Value: chainsync.Value{Assets: map[chainsync.AssetID]num.Int{"LP_Y": num.Uint64(500)}}},
	}
	pools := map[string]types.Pool{
		"X": {PoolIdent: "X", LPAsset: "LP_X", TotalLPTokens: 500, AssetAQuantity: 1000},
		"Y": {PoolIdent: "Y", LPAsset: "LP_Y", TotalLPTokens: 1000, AssetAQuantity: 100},
	}
	lockedLP, totalLP, totalValueByPool, totalValue := CalculateTotalLP(positions, pools)
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
	selectedPools := SelectPoolsForEmission(program, map[string]uint64{
		"A": 100,
		"B": 200,
		"C": 300,
	}, nil)
	assert.EqualValues(t, map[string]uint64{"C": 300, "B": 200}, selectedPools)

	program.MaxPoolIntegerPercent = 30
	selectedPools = SelectPoolsForEmission(program, map[string]uint64{
		"A": 100,
		"B": 101,
		"C": 202,
	}, nil)
	assert.EqualValues(t, map[string]uint64{"C": 202}, selectedPools)

	program.MaxPoolCount = 10
	program.MaxPoolIntegerPercent = 33
	selectedPools = SelectPoolsForEmission(program, map[string]uint64{
		"A": 997,
		"B": 998,
		"C": 999,
		"D": 1000,
		"E": 1001,
		"F": 1002, // Ensures that F+E are *just* slightly over 33%, A-D should get excluded
	}, nil)
	assert.EqualValues(t, map[string]uint64{"F": 1002, "E": 1001}, selectedPools)
}

func Test_PoolsForEmissions_WithNepotism(t *testing.T) {
	program := sampleProgram(500_000)
	program.NepotismPools = []string{"B"}
	program.MaxPoolCount = 2
	program.MaxPoolIntegerPercent = 20
	selectedPools := SelectPoolsForEmission(program, map[string]uint64{
		"A": 50,
		"B": 100,
		"C": 200,
		"D": 300,
	}, nil)
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
}

func Test_OwnerByLPAndAsset(t *testing.T) {
	pools := map[string]types.Pool{
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
	pools := map[string]types.Pool{
		"X": {PoolIdent: "X", LPAsset: "LP_X"},
		"Y": {PoolIdent: "Y", LPAsset: "LP_Y"},
	}
	byAsset := RegroupByAsset(map[string]uint64{"X": 100}, pools)
	assert.EqualValues(t, map[chainsync.AssetID]uint64{"LP_X": 100}, byAsset)

	byAsset = RegroupByAsset(map[string]uint64{"X": 100, "Y": 200}, pools)
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
	numPositions := rand.Intn(3000) + 1000
	numOwners := rand.Intn(numPositions-1) + 1
	numPools := rand.Intn(300) + 100
	program, calcOutputs := Random_Calc_Earnings(numPositions, numOwners, numPools)
	total := uint64(0)
	for _, e := range calcOutputs.Earnings {
		total += e.Value.Assets[program.EmittedAsset].Uint64()
	}
	if total == 0 {
		assert.Empty(t, calcOutputs.Earnings)
	} else {
		assert.Equal(t, total, program.DailyEmission)
	}
}

func Benchmark_Calculate_Earnings(b *testing.B) {
	for i := 0; i < b.N; i++ {
		numPositions := 100_000
		numOwners := 90_000
		numPools := 1500
		Random_Calc_Earnings(numPositions, numOwners, numPools)
	}
}

func Random_Calc_Earnings(numPositions, numOwners, numPools int) (types.Program, CalculationOutputs) {
	now := types.Date(time.Now().Format(types.DateFormat))
	program := sampleProgram(500_000_000_000)
	var positions []types.Position
	pools := map[string]types.Pool{}

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

	for i := 0; i < numPools; i++ {
		poolIdent := fmt.Sprintf("Pool_%v", i)
		pools[poolIdent] = types.Pool{
			PoolIdent:     poolIdent,
			TotalLPTokens: lockedByPool[i] + uint64(rand.Int63n(100_000_000_000)),
			LPAsset:       chainsync.AssetID(fmt.Sprintf("LP_%v", i)),
		}
	}

	return program, CalculateEarnings(now, 0, 86400, program, positions, pools)
}
