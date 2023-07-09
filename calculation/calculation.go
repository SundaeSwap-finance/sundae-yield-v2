package calculation

import (
	"fmt"
	"math/big"
	"sort"
	"time"

	"github.com/SundaeSwap-finance/ogmigo/ouroboros/chainsync"
	"github.com/SundaeSwap-finance/ogmigo/ouroboros/chainsync/num"
	"github.com/SundaeSwap-finance/sundae-yield-v2/types"
)

// Calculate the total amount of sundae delegated to each pool, according to each users chosen "weighting"
func CalculateTotalDelegations(program types.Program, positions []types.Position, poolsByIdent map[string]types.Pool) (map[string]uint64, uint64) {
	totalDelegationsByPoolIdent := map[string]uint64{}
	for _, position := range positions {
		totalDelegationAsset := position.Value.Assets[program.StakedAsset]

		poolsByLPToken := map[chainsync.AssetID]types.Pool{}
		for _, pool := range poolsByIdent {
			poolsByLPToken[pool.LPAsset] = pool
		}

		// Add in the value of LP tokens, according to the ratio of the pools at the snapshot
		for assetId, amt := range position.Value.Assets {
			if pool, ok := poolsByLPToken[assetId]; ok {
				if pool.AssetA == program.StakedAsset || pool.AssetB == program.StakedAsset {
					frac := big.NewInt(amt.Int64())
					if pool.AssetA == program.StakedAsset {
						frac = frac.Mul(frac, big.NewInt(int64(pool.AssetAQuantity)))
					} else if pool.AssetB == program.StakedAsset {
						frac = frac.Mul(frac, big.NewInt(int64(pool.AssetBQuantity)))
					}
					frac = frac.Div(frac, big.NewInt(int64(pool.TotalLPTokens)))
					totalDelegationAsset = totalDelegationAsset.Add(num.Int(*frac))
				}
			}
		}

		// Each UTXO of locked SUNDAE may encode a weighting for a set of pools, as described above;
		totalWeight := uint64(0)
		for _, w := range position.Delegation {
			// Skip delegations for other programs
			if w.Program != program.ID {
				continue
			}
			totalWeight += uint64(w.Weight)
		}
		// The absence of such a list will exclude all SUNDAE at that UTXO from consideration.
		if totalWeight == 0 {
			totalDelegationsByPoolIdent[""] += uint64(totalDelegationAsset.Int64())
			continue
		}

		// ... then divide the SUNDAE at the UTXO among the selected options in accordance to the weight
		delegatedAssetAmount := uint64(0)
		for _, delegation := range position.Delegation {
			// Skip delegations for other programs
			if delegation.Program != program.ID {
				continue
			}
			// rounding down
			frac := big.NewInt(totalDelegationAsset.Int64())
			frac = frac.Mul(frac, big.NewInt(int64(delegation.Weight)))
			frac = frac.Div(frac, num.Uint64(totalWeight).BigInt())
			allocation := frac.Uint64()
			delegatedAssetAmount += allocation
			totalDelegationsByPoolIdent[delegation.PoolIdent] += allocation
		}

		// ... and distributing millionths of a SUNDAE among the options in order until the total SUNDAE allocated equals the SUNDAE held at the UTXO.
		// Note: this is guaranteed to be small because of high precision arithmetic above
		remainder := int(totalDelegationAsset.Uint64() - delegatedAssetAmount)
		if remainder < 0 {
			panic(fmt.Sprintf("allocated more asset (%v) to a pool than in the stake position (%v), somehow", delegatedAssetAmount, totalDelegationAsset))
		} else if remainder > 0 {
			for i := 0; remainder > 0; i++ {
				idx := i % len(position.Delegation)
				// Skip over delegations for other programs
				if position.Delegation[idx].Program != program.ID {
					continue
				}
				delegation := position.Delegation[idx]
				totalDelegationsByPoolIdent[delegation.PoolIdent] += 1
				delegatedAssetAmount += 1
				remainder -= 1
			}
		}
		if totalDelegationAsset.Uint64() != delegatedAssetAmount {
			// There's a bug in the round-robin distribution code, panic so we fix the bug
			panic("round-robin distribution wasn't succesful")
		}
	}

	totalDelegations := uint64(0)
	for _, amt := range totalDelegationsByPoolIdent {
		totalDelegations += amt
	}

	return totalDelegationsByPoolIdent, totalDelegations
}

func CalculateTotalLP(positions []types.Position, poolsByIdent map[string]types.Pool) (map[string]uint64, map[string]uint64, map[string]uint64, uint64) {
	poolsByLPToken := map[chainsync.AssetID]types.Pool{}
	for _, pool := range poolsByIdent {
		poolsByLPToken[pool.LPAsset] = pool
	}
	lockedLPByIdent := map[string]uint64{}
	valueByIdent := map[string]uint64{}
	totalValue := uint64(0)
	for _, position := range positions {
		for assetId, amount := range position.Value.Assets {
			if pool, ok := poolsByLPToken[assetId]; ok {
				lockedLPByIdent[pool.PoolIdent] += amount.Uint64()
				if pool.AssetA == "" {
					lockedLP := amount.BigInt()
					totalLP := num.Uint64(pool.TotalLPTokens).BigInt()
					numerator := big.NewInt(0).Mul(lockedLP, num.Uint64(pool.AssetAQuantity).BigInt())
					assetA := big.NewInt(0).Div(numerator, totalLP)
					valueByIdent[pool.PoolIdent] += assetA.Uint64() * 2
					totalValue += assetA.Uint64() * 2
				}
			}
		}
	}
	totalLPByIdent := map[string]uint64{}
	for pool := range lockedLPByIdent {
		totalLPByIdent[pool] = poolsByIdent[pool].TotalLPTokens
	}
	return lockedLPByIdent, totalLPByIdent, valueByIdent, totalValue
}

// Check, that `portion“ is at least `percent` of `total“
func atLeastIntegerPercent(portion uint64, total uint64, percent int) bool {
	if percent == 0 {
		return true
	}
	if portion == 0 {
		return false
	}
	return 100*portion/total >= uint64(percent)
}

// Check if a pool is even *qualified* for rewards
func isPoolQualified(program types.Program, pool types.Pool, locked uint64) (bool, string) {
	if !atLeastIntegerPercent(locked, pool.TotalLPTokens, program.MinLPIntegerPercent) {
		return false, fmt.Sprintf("less than %v%% of LP tokens locked", program.MinLPIntegerPercent)
	}
	if program.EligiblePools != nil {
		for _, poolIdent := range program.EligiblePools {
			if poolIdent == pool.PoolIdent {
				return true, ""
			}
		}
		return false, "Program lists eligible pools, but doesn't list this pool"
	} else if program.DisqualifiedPools != nil {
		for _, poolIdent := range program.DisqualifiedPools {
			if poolIdent == pool.PoolIdent {
				return false, "Pool is explicitly disqualified"
			}
		}
		return true, ""
	}
	return true, ""
}

// Select the top pools according to the program criteria
func SelectPoolsForEmission(program types.Program, delegationsByPool map[string]uint64, poolsByIdent map[string]types.Pool) map[string]uint64 {
	// Convert the map into a list of candidates, so we can sort them
	type Candidate struct {
		PoolIdent string
		Total     uint64
	}
	var candidates []Candidate

	totalDelegation := uint64(0)
	for poolIdent, amt := range delegationsByPool {
		totalDelegation += amt
		candidates = append(candidates, Candidate{PoolIdent: poolIdent, Total: amt})
	}

	sort.Slice(candidates, func(i, j int) bool {
		// In the case of an exact tie (very unlikely), prefer the one with less liquidity
		// under the hypothesis that less liquidity needs to attract more liquidity providers
		// (technically wasn't part of the spec, and so we make a reasonable choice)
		if candidates[i].Total == candidates[j].Total {
			iLP := poolsByIdent[candidates[i].PoolIdent].TotalLPTokens
			jLP := poolsByIdent[candidates[j].PoolIdent].TotalLPTokens
			if iLP == jLP {
				return candidates[i].PoolIdent < candidates[j].PoolIdent
			}
			return iLP < jLP
		}
		return candidates[i].Total > candidates[j].Total
	})

	// Then select either the top N pools, or the top covering percent,
	// whichever is fewer
	poolsReceivingEmissionsByIdent := map[string]uint64{}
	totalQualifyingDelegation := uint64(0)
	for _, delegation := range candidates {
		if delegation.PoolIdent == "" {
			continue
		}
		poolsReceivingEmissionsByIdent[delegation.PoolIdent] = delegation.Total
		totalQualifyingDelegation += delegation.Total
		if len(poolsReceivingEmissionsByIdent) == program.MaxPoolCount {
			break
		}
		if atLeastIntegerPercent(totalQualifyingDelegation, totalDelegation, program.MaxPoolIntegerPercent) {
			break
		}
	}

	return poolsReceivingEmissionsByIdent
}

func DistributeEmissionsToPools(program types.Program, poolsReceivingEmissionsByIdent map[string]uint64) map[string]uint64 {
	// We'll need to loop over pools round-robin by largest value; ordering of maps is non-deterministic
	type Pairs struct {
		PoolIdent string
		Amount    uint64
	}
	poolWeights := []Pairs{}
	totalWeight := uint64(0)
	for poolIdent, weight := range poolsReceivingEmissionsByIdent {
		totalWeight += weight
		poolWeights = append(poolWeights, Pairs{PoolIdent: poolIdent, Amount: weight})
	}
	// No pool has received weight
	// We then divide the daily emissions among these pools in proportion to their weight, rounding down
	emissionsByPool := map[string]uint64{}
	if totalWeight == 0 {
		return emissionsByPool
	}

	allocatedAmount := uint64(0)
	for poolIdent, weight := range poolsReceivingEmissionsByIdent {
		frac := big.NewInt(0).SetUint64(program.DailyEmission)
		frac = frac.Mul(frac, big.NewInt(0).SetUint64(weight))
		frac = frac.Div(frac, big.NewInt(0).SetUint64(totalWeight))
		allocation := frac.Uint64()
		allocatedAmount += allocation
		emissionsByPool[poolIdent] += allocation
	}
	// and distributing [diminutive tokens] among them until the daily emission is accounted for.
	remainder := int(program.DailyEmission - allocatedAmount)
	if remainder < 0 {
		panic("emitted more to pools than the daily emissions, somehow")
	} else if remainder > 0 {
		sort.Slice(poolWeights, func(i, j int) bool {
			if poolWeights[i].Amount == poolWeights[j].Amount {
				return poolWeights[i].PoolIdent < poolWeights[j].PoolIdent
			}
			return poolWeights[i].Amount > poolWeights[j].Amount
		})
		for i := 0; i < remainder; i++ {
			pool := poolWeights[i%len(poolWeights)]
			emissionsByPool[pool.PoolIdent] += 1
			allocatedAmount += 1
		}
		if allocatedAmount != program.DailyEmission {
			// There's a bug in the round-robin distribution code, panic so we fix the bug
			panic("round-robin distribution wasn't succesful")
		}
	}

	return emissionsByPool
}

func TotalLPByOwnerAndAsset(positions []types.Position, poolsByIdent map[string]types.Pool) (map[string]chainsync.Value, map[chainsync.AssetID]uint64) {
	poolsByLP := map[chainsync.AssetID]types.Pool{}
	for _, pool := range poolsByIdent {
		poolsByLP[pool.LPAsset] = pool
	}

	lpByOwner := map[string]chainsync.Value{}
	lpByAsset := map[chainsync.AssetID]uint64{}
	for _, p := range positions {
		for assetId, amount := range p.Value.Assets {
			if _, ok := poolsByLP[assetId]; ok {
				lpValue := chainsync.Value{Coins: num.Int64(0), Assets: map[chainsync.AssetID]num.Int{assetId: amount}}
				lpByOwner[p.OwnerID] = chainsync.Add(lpByOwner[p.OwnerID], lpValue)
				lpByAsset[assetId] += amount.Uint64()
			}
		}
	}
	return lpByOwner, lpByAsset
}

func RegroupByAsset(byPool map[string]uint64, poolsByIdent map[string]types.Pool) map[chainsync.AssetID]uint64 {
	byLPAsset := map[chainsync.AssetID]uint64{}
	for poolIdent, amount := range byPool {
		if amount == 0 {
			continue
		}
		pool := poolsByIdent[poolIdent]
		byLPAsset[pool.LPAsset] = amount
	}
	return byLPAsset
}

func RegroupByPool(byAsset map[chainsync.AssetID]uint64, poolsByIdent map[string]types.Pool) map[string]uint64 {
	// Note: assumes bijection
	byIdent := map[string]uint64{}
	for poolIdent, pool := range poolsByIdent {
		if byAsset[pool.LPAsset] == 0 {
			continue
		}
		byIdent[poolIdent] = byAsset[pool.LPAsset]
	}
	return byIdent
}

func DistributeEmissionsToOwners(lpTokensByOwner map[string]chainsync.Value, emissionsByAsset map[chainsync.AssetID]uint64, lpTokensByAsset map[chainsync.AssetID]uint64) map[string]uint64 {
	// expand out the lpTokensByOwner, so we can sort them canonically for the round-robin
	type OwnerStake struct {
		OwnerID string
		Value   chainsync.Value
	}
	var ownerStakes []OwnerStake
	for ownerId, value := range lpTokensByOwner {
		ownerStakes = append(ownerStakes, OwnerStake{OwnerID: ownerId, Value: value})
	}
	// We sort by owner key here; in theory we could sort by "total value staked", but it's
	// very difficult to compare that here, so we just sort by owner;
	// This can result in at most one diminutive unit of a token, ex. 1 millionth of a SUNDAE
	// if we ever distribute a token that has a high diminutive value (like an XDIAMOND), we may need to revist this
	sort.Slice(ownerStakes, func(i, j int) bool {
		return ownerStakes[i].OwnerID < ownerStakes[j].OwnerID
	})

	// Now loop over each, and allocate a portion of the emissions
	emissionsByOwner := map[string]uint64{}
	allocatedByAsset := map[chainsync.AssetID]uint64{}
	for _, ownerStake := range ownerStakes {
		for assetId, amount := range ownerStake.Value.Assets {
			emission := emissionsByAsset[assetId]
			totalLP := lpTokensByAsset[assetId]
			frac := big.NewInt(0).SetUint64(emission)
			frac = frac.Mul(frac, amount.BigInt())
			frac = frac.Div(frac, big.NewInt(0).SetUint64(totalLP))
			allocation := frac.Uint64()
			emissionsByOwner[ownerStake.OwnerID] += allocation
			allocatedByAsset[assetId] += allocation
		}
	}
	// The emissions for each owner will be rounded down, and millionths of a SUNDAE distributed round-robin until
	// the total user emissions match the pool emissions.
	for assetId, allocatedAmount := range allocatedByAsset {
		remainder := int(emissionsByAsset[assetId] - allocatedAmount)
		if remainder < 0 {
			panic("emitted more to users than the allocated emissions for a pool, somehow")
		} else if remainder > 0 {
			for i := 0; i < remainder; i++ {
				owner := ownerStakes[i%len(ownerStakes)]
				emissionsByOwner[owner.OwnerID] += 1
				allocatedAmount += 1
			}
			if emissionsByAsset[assetId] != allocatedAmount {
				panic("round-robin distribution wasn't succesful")
			}
		}
	}
	return emissionsByOwner
}

func EmissionsByOwnerToEarnings(date types.Date, program types.Program, emissionsByOwner map[string]uint64, ownersByID map[string]types.MultisigScript) []types.Earning {
	var ret []types.Earning
	for ownerID, amount := range emissionsByOwner {
		if amount == 0 {
			continue
		}
		earning := types.Earning{
			OwnerID:    ownerID,
			Owner:      ownersByID[ownerID],
			Program:    program.ID,
			EarnedDate: date,
			Value: chainsync.Value{
				Coins: num.Int64(0),
				Assets: map[chainsync.AssetID]num.Int{
					program.EmittedAsset: num.Uint64(amount),
				},
			},
		}
		if program.EarningExpiration != nil {
			time, err := time.Parse(types.DateFormat, date)
			if err != nil {
				panic(fmt.Sprintf("invalid date %v", date))
			}
			expiration := time.Add(*program.EarningExpiration)
			earning.ExpirationDate = &expiration
		}
		ret = append(ret, earning)
	}
	// Order them by OwnerID, for testability; no impact on the outcome
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].OwnerID < ret[j].OwnerID
	})
	return ret
}

type CalculationOutputs struct {
	Timestamp string

	TotalDelegations uint64
	DelegationByPool map[string]uint64

	QualifyingDelegationByPool  map[string]uint64
	PoolDisqualificationReasons map[string]string

	LockedLPByPool map[string]uint64
	TotalLPByPool  map[string]uint64

	EstimatedLockedADAValue       uint64
	EstimatedLockedADAValueByPool map[string]uint64

	TotalEmissions   uint64
	EmissionsByPool  map[string]uint64
	EmissionsByOwner map[string]uint64

	Earnings []types.Earning
}

func CalculateEarnings(date types.Date, program types.Program, positions []types.Position, poolsByIdent map[string]types.Pool) CalculationOutputs {
	// Check for start and end dates, inclusive
	if date < program.FirstDailyRewards {
		return CalculationOutputs{}
	}
	if program.LastDailyRewards != "" && date > program.LastDailyRewards {
		return CalculationOutputs{}
	}

	// To calculate the daily emissions, ... first take inventory of SUNDAE held at the Locking Contract
	// and factory in the users delegation
	delegationByPool, totalDelegation := CalculateTotalDelegations(program, positions, poolsByIdent)
	qualifyingDelegationsPerPool := map[string]uint64{}
	// Any pool that has less than 1% of the pools LP tokens held at the Locking Contract
	// will be considered an abstention and will not be eligible for rewards.
	lockedLPByPool, totalLPByPool, estimatedValue, totalEstimatedValue := CalculateTotalLP(positions, poolsByIdent)
	poolDisqualificationReasons := map[string]string{}
	for poolIdent, locked := range lockedLPByPool {
		qualified, reason := isPoolQualified(program, poolsByIdent[poolIdent], locked)
		if !qualified {
			if reason != "" {
				poolDisqualificationReasons[poolIdent] = reason
			}
			qualifyingDelegationsPerPool[""] += delegationByPool[poolIdent]
		} else {
			qualifyingDelegationsPerPool[poolIdent] += delegationByPool[poolIdent]
		}
	}

	// If no pools are qualified (extremely degenerate case, return no earnings, and reserve those tokens for the treasury)
	if _, ok := delegationByPool[""]; len(delegationByPool) == 0 || (ok && len(delegationByPool) == 1) {
		return CalculationOutputs{
			Timestamp:                     time.Now().Format(time.RFC3339),
			TotalDelegations:              totalDelegation,
			DelegationByPool:              delegationByPool,
			QualifyingDelegationByPool:    qualifyingDelegationsPerPool,
			PoolDisqualificationReasons:   poolDisqualificationReasons,
			LockedLPByPool:                lockedLPByPool,
			TotalLPByPool:                 totalLPByPool,
			EstimatedLockedADAValue:       totalEstimatedValue,
			EstimatedLockedADAValueByPool: estimatedValue,
		}

	}

	// The top pools ... will be eligible for yield farming rewards that day.
	poolsReceivingEmissions := SelectPoolsForEmission(program, qualifyingDelegationsPerPool, poolsByIdent)

	// We then divide the daily emissions among these pools ...
	emissionsByAsset := RegroupByAsset(DistributeEmissionsToPools(program, poolsReceivingEmissions), poolsByIdent)

	// For each pool, SundaeSwap labs will then calculate the allocation of rewards in proportion to the LP tokens held at the Locking Contract.
	lpTokensByOwner, lpTokensByAsset := TotalLPByOwnerAndAsset(positions, poolsByIdent)

	emissionsByOwner := DistributeEmissionsToOwners(lpTokensByOwner, emissionsByAsset, lpTokensByAsset)

	ownersByID := map[string]types.MultisigScript{}
	for _, position := range positions {
		ownersByID[position.OwnerID] = position.Owner
	}

	// Users will be able to claim these emitted tokens
	// we return a set of "earnings" for the day
	earnings := EmissionsByOwnerToEarnings(date, program, emissionsByOwner, ownersByID)
	return CalculationOutputs{
		Timestamp: time.Now().Format(time.RFC3339),

		TotalDelegations: totalDelegation,
		DelegationByPool: delegationByPool,

		QualifyingDelegationByPool:  qualifyingDelegationsPerPool,
		PoolDisqualificationReasons: poolDisqualificationReasons,

		LockedLPByPool: lockedLPByPool,
		TotalLPByPool:  totalLPByPool,

		EstimatedLockedADAValue:       totalEstimatedValue,
		EstimatedLockedADAValueByPool: estimatedValue,

		TotalEmissions:   program.DailyEmission,
		EmissionsByPool:  RegroupByPool(emissionsByAsset, poolsByIdent),
		EmissionsByOwner: emissionsByOwner,

		Earnings: earnings,
	}
}
