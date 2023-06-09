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
		FirstDailyRewards:   "2001-01-01",
		LastDailyRewards:    "2099-01-01", // If this code is still in use in 2099, call the police (after updating tests)
		StakedAsset:         chainsync.AssetID("Staked"),
		MinLPIntegerPercent: 1,
		EmittedAsset:        chainsync.AssetID("Emitted"),
		DailyEmission:       emissions,
	}
}

func samplePosition(staked int64, delegations ...types.Delegation) types.Position {
	return types.Position{
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
		samplePosition(100_000, types.Delegation{PoolIdent: "01", Weight: 1}),
	}
	totalDelegations := CalculateTotalDelegations(program, positions)
	assert.EqualValues(t, 100_000, totalDelegations["01"])

	positions = []types.Position{
		samplePosition(100_000),
	}
	totalDelegations = CalculateTotalDelegations(program, positions)
	assert.EqualValues(t, 100_000, totalDelegations[""])

	// Should split evenly between delegations
	positions = []types.Position{
		samplePosition(100_000, types.Delegation{PoolIdent: "01", Weight: 1}, types.Delegation{PoolIdent: "02", Weight: 1}),
	}
	totalDelegations = CalculateTotalDelegations(program, positions)
	assert.EqualValues(t, 50_000, totalDelegations["01"])
	assert.EqualValues(t, 50_000, totalDelegations["02"])

	// Should handle bankers rounding
	positions = []types.Position{
		samplePosition(100_000, types.Delegation{PoolIdent: "01", Weight: 1}, types.Delegation{PoolIdent: "02", Weight: 2}),
	}
	totalDelegations = CalculateTotalDelegations(program, positions)
	assert.EqualValues(t, 33_334, totalDelegations["01"])
	assert.EqualValues(t, 66_666, totalDelegations["02"])

	// Should handle multiple positions
	positions = []types.Position{
		samplePosition(100_000, types.Delegation{PoolIdent: "01", Weight: 1}, types.Delegation{PoolIdent: "02", Weight: 1}),
		samplePosition(200_000, types.Delegation{PoolIdent: "02", Weight: 1}, types.Delegation{PoolIdent: "03", Weight: 1}),
	}
	totalDelegations = CalculateTotalDelegations(program, positions)
	assert.EqualValues(t, 50_000, totalDelegations["01"])
	assert.EqualValues(t, 150_000, totalDelegations["02"])
	assert.EqualValues(t, 100_000, totalDelegations["03"])

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
		samplePosition(initialSundae, delegations...),
	}
	totalDelegations := CalculateTotalDelegations(program, positions)
	actualSum := uint64(0)
	for _, s := range totalDelegations {
		actualSum += s
	}
	assert.EqualValues(t, initialSundae, actualSum)
}

func Test_AtLeastOnePercent(t *testing.T) {
	assert.False(t, atLeastIntegerPercent(0, 15000, 1))
	assert.False(t, atLeastIntegerPercent(1, 15000, 1))
	assert.False(t, atLeastIntegerPercent(149, 15000, 1))
	assert.False(t, atLeastIntegerPercent(1499, 150000, 1))
	assert.False(t, atLeastIntegerPercent(1234, 15000, 9))
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
	assert.True(t, isPoolQualified(program, poolA, 150))
	assert.True(t, isPoolQualified(program, poolA, 500))
	assert.False(t, isPoolQualified(program, poolA, 10))

	program.EligiblePools = []string{"A"}
	assert.True(t, isPoolQualified(program, poolA, 500))
	assert.False(t, isPoolQualified(program, poolB, 500))
	program.EligiblePools = nil

	program.DisqualifiedPools = []string{"A"}
	assert.False(t, isPoolQualified(program, poolA, 500))
	assert.True(t, isPoolQualified(program, poolB, 500))
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
	program.MaxPoolIntegerPercent = 20
	selectedPools = SelectPoolsForEmission(program, map[string]uint64{
		"":  7899, // total of 10,000
		"A": 100,
		"B": 200,
		"C": 300,
		"D": 400,
		"E": 500,
		"F": 601, // Ensures that F-B are *just* slightly over 20%, A should get excluded
	}, nil)
	assert.EqualValues(t, map[string]uint64{"F": 601, "E": 500, "D": 400, "C": 300, "B": 200}, selectedPools)
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
	byOwner, byAsset := TotalLPByOwnerAndAsset([]types.Position{
		{OwnerID: "A", Value: chainsync.Value{Assets: map[chainsync.AssetID]num.Int{"LP_X": num.Uint64(100)}}},
	}, pools)
	assert.EqualValues(t, map[string]chainsync.Value{
		"A": {Assets: map[chainsync.AssetID]num.Int{"LP_X": num.Uint64(100)}},
	}, byOwner)
	assert.EqualValues(t, map[chainsync.AssetID]uint64{
		"LP_X": 100,
	}, byAsset)

	byOwner, byAsset = TotalLPByOwnerAndAsset([]types.Position{
		{OwnerID: "A", Value: chainsync.Value{Assets: map[chainsync.AssetID]num.Int{"LP_X": num.Uint64(100)}}},
		{OwnerID: "B", Value: chainsync.Value{Assets: map[chainsync.AssetID]num.Int{"LP_X": num.Uint64(200)}}},
	}, pools)
	assert.EqualValues(t, map[string]chainsync.Value{
		"A": {Assets: map[chainsync.AssetID]num.Int{"LP_X": num.Uint64(100)}},
		"B": {Assets: map[chainsync.AssetID]num.Int{"LP_X": num.Uint64(200)}},
	}, byOwner)
	assert.EqualValues(t, map[chainsync.AssetID]uint64{
		"LP_X": 300,
	}, byAsset)

	byOwner, byAsset = TotalLPByOwnerAndAsset([]types.Position{
		{OwnerID: "A", Value: chainsync.Value{Assets: map[chainsync.AssetID]num.Int{"LP_X": num.Uint64(100)}}},
		{OwnerID: "B", Value: chainsync.Value{Assets: map[chainsync.AssetID]num.Int{"LP_X": num.Uint64(200)}}},
		{OwnerID: "B", Value: chainsync.Value{Assets: map[chainsync.AssetID]num.Int{"LP_X": num.Uint64(300)}}},
	}, pools)
	assert.EqualValues(t, map[string]chainsync.Value{
		"A": {Assets: map[chainsync.AssetID]num.Int{"LP_X": num.Uint64(100)}},
		"B": {Assets: map[chainsync.AssetID]num.Int{"LP_X": num.Uint64(500)}},
	}, byOwner)
	assert.EqualValues(t, map[chainsync.AssetID]uint64{
		"LP_X": 600,
	}, byAsset)

	byOwner, byAsset = TotalLPByOwnerAndAsset([]types.Position{
		{OwnerID: "A", Value: chainsync.Value{Assets: map[chainsync.AssetID]num.Int{"LP_X": num.Uint64(100)}}},
		{OwnerID: "B", Value: chainsync.Value{Assets: map[chainsync.AssetID]num.Int{"LP_X": num.Uint64(200), "LP_Y": num.Uint64(150)}}},
		{OwnerID: "B", Value: chainsync.Value{Assets: map[chainsync.AssetID]num.Int{"LP_X": num.Uint64(300)}}},
	}, pools)
	assert.EqualValues(t, map[string]chainsync.Value{
		"A": {Assets: map[chainsync.AssetID]num.Int{"LP_X": num.Uint64(100)}},
		"B": {Assets: map[chainsync.AssetID]num.Int{"LP_X": num.Uint64(500), "LP_Y": num.Uint64(150)}},
	}, byOwner)
	assert.EqualValues(t, map[chainsync.AssetID]uint64{
		"LP_X": 600,
		"LP_Y": 150,
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

func LPByOwners(alloc ...Alloc) map[string]chainsync.Value {
	lpByOwners := map[string]chainsync.Value{}
	for _, a := range alloc {
		lpByOwners[a.string] = chainsync.Add(lpByOwners[a.string], chainsync.Value{
			Assets: map[chainsync.AssetID]num.Int{
				a.AssetID: num.Uint64(a.uint64),
			},
		})
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
	assert.EqualValues(t, map[string]uint64{"A": 1000}, emissionsByOwner)

	lpByOwners = LPByOwners(
		Alloc{"A", "LP_X", 100},
		Alloc{"B", "LP_X", 200},
	)
	lpTokensByAsset = map[chainsync.AssetID]uint64{"LP_X": 300}
	emissionsByOwner = DistributeEmissionsToOwners(lpByOwners, emissionsByAsset, lpTokensByAsset)
	assert.EqualValues(t, map[string]uint64{"A": 334, "B": 666}, emissionsByOwner)

	lpByOwners = LPByOwners(
		Alloc{"A", "LP_X", 100},
		Alloc{"B", "LP_X", 200},
		Alloc{"A", "LP_Y", 300},
	)
	emissionsByAsset = map[chainsync.AssetID]uint64{"LP_X": 1000, "LP_Y": 500}
	lpTokensByAsset = map[chainsync.AssetID]uint64{"LP_X": 300, "LP_Y": 300}
	emissionsByOwner = DistributeEmissionsToOwners(lpByOwners, emissionsByAsset, lpTokensByAsset)
	assert.EqualValues(t, map[string]uint64{"A": 334 + 500, "B": 666}, emissionsByOwner)
}

func Test_EmissionsToEarnings(t *testing.T) {
	now := types.Date(time.Now().Format(types.DateFormat))
	program := sampleProgram(500_000)
	ownerA := types.MultisigScript{Signature: &types.Signature{KeyHash: []byte("A")}}
	ownerB := types.MultisigScript{Signature: &types.Signature{KeyHash: []byte("B")}}
	emissions := EmissionsByOwnerToEarnings(now, program, map[string]uint64{
		"A": 1000,
		"B": 1500,
	}, map[string]types.MultisigScript{
		"A": ownerA,
		"B": ownerB,
	})
	assert.EqualValues(t, []types.Earning{
		{OwnerID: "A", Owner: ownerA, EarnedDate: now, Value: chainsync.Value{Assets: map[chainsync.AssetID]num.Int{"Emitted": num.Uint64(1000)}}},
		{OwnerID: "B", Owner: ownerB, EarnedDate: now, Value: chainsync.Value{Assets: map[chainsync.AssetID]num.Int{"Emitted": num.Uint64(1500)}}},
	}, emissions)
}

func Test_Calculate_Earnings(t *testing.T) {
	now := types.Date(time.Now().Format(types.DateFormat))
	program := sampleProgram(500_000_000_000)
	var positions []types.Position
	pools := map[string]types.Pool{}

	numPositions := rand.Intn(3000) + 1000
	numOwners := rand.Intn(numPositions-1) + 1
	numPools := rand.Intn(300) + 100

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
		numDelegations := rand.Intn(30)
		for j := 0; j < numDelegations; j++ {
			poolIdent := fmt.Sprintf("Pool_%v", rand.Intn(numPools))
			weight := uint32(rand.Intn(50_000))
			position.Delegation = append(position.Delegation, types.Delegation{PoolIdent: poolIdent, Weight: weight})
		}

		numLP := rand.Intn(15)
		for j := 0; j < numLP; j++ {
			lp := chainsync.AssetID(fmt.Sprintf("LP_%v", rand.Intn(numPools)))
			position.Value.Assets[lp] = num.Int64(rand.Int63n(30_000_000))
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
			TotalLPTokens: uint64(rand.Int63n(100_000_000_000_000)) + 30_000_000,
			LPAsset:       chainsync.AssetID(fmt.Sprintf("LP_%v", i)),
		}
	}

	earnings := CalculateEarnings(now, program, positions, pools)
	total := uint64(0)
	for _, e := range earnings {
		total += e.Value.Assets[program.EmittedAsset].Uint64()
	}
	if total == 0 {
		assert.Empty(t, earnings)
	} else {
		assert.Equal(t, total, program.DailyEmission)
	}
}
