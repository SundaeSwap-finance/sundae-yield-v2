package calculation

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"time"

	"github.com/SundaeSwap-finance/ogmigo/v6/ouroboros/chainsync/compatibility"
	"github.com/SundaeSwap-finance/ogmigo/v6/ouroboros/chainsync/num"
	"github.com/SundaeSwap-finance/ogmigo/v6/ouroboros/shared"
	"github.com/SundaeSwap-finance/sundae-yield-v2/types"
)

type PoolLookup interface {
	PoolByIdent(ctx context.Context, poolIdent string) (types.Pool, error)
	PoolByLPToken(ctx context.Context, lpToken shared.AssetID) (types.Pool, error)
	IsLPToken(assetId shared.AssetID) bool
	LPTokenToPoolIdent(lpToken shared.AssetID) (string, error)
}

// Calculate the total amount of sundae delegated to each pool, according to each users chosen "weighting"
func CalculateTotalDelegations(
	ctx context.Context,
	program types.Program,
	positions []types.Position,
	poolLookup PoolLookup,
) (map[string]uint64, uint64, error) {
	totalDelegationsByPoolIdent := map[string]uint64{}
	if program.StakedAsset == "" {
		for _, pool := range program.EligiblePools {
			totalDelegationsByPoolIdent[pool] = 1
		}
		return totalDelegationsByPoolIdent, uint64(len(program.EligiblePools)), nil
	}

	for _, position := range positions {
		totalDelegationAsset := shared.Value(position.Value).AssetAmount(program.StakedAsset)

		// Add in the value of LP tokens, according to the ratio of the pools at the snapshot

		for policy, policyMap := range position.Value {
			for assetName, amount := range policyMap {

				assetId := shared.FromSeparate(policy, assetName)

				if poolLookup.IsLPToken(assetId) {
					pool, err := poolLookup.PoolByLPToken(ctx, assetId)
					if err != nil {
						return nil, 0, fmt.Errorf("failed to lookup pool for LP token %v: %w", assetId, err)
					}
					if pool.TotalLPTokens == 0 {
						// The pool has since been deleted.
						// So, as a corner case, we just skip this LP asset
						continue
					}
					if pool.AssetA == program.StakedAsset || pool.AssetB == program.StakedAsset {
						frac := big.NewInt(amount.Int64())
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

	return totalDelegationsByPoolIdent, totalDelegations, nil
}

// Calculate the locked LP, total LP, estimated lovelace value per pool and globally, as of the final snapshot
func CalculateTotalLPAtSnapshot(
	ctx context.Context,
	maxSlot uint64,
	positions []types.Position,
	poolLookup PoolLookup,
) (map[string]uint64, map[string]uint64, map[string]uint64, uint64, error) {
	poolsByIdent := map[string]types.Pool{}
	lockedLPByIdent := map[string]uint64{}
	valueByIdent := map[string]uint64{}
	totalValue := uint64(0)
	for _, position := range positions {
		// The values calculated by this method are only used for reporting purposes
		// and for the 1% pool filter; So, we interpret the spec to be
		// "only pools with 1% of the LP tokens locked at the snapshot are eligible"
		// (as opposed to integrated over the day)
		// This also avoids a subtle corner case where the pool can be deleted by the snapshot,
		// but still have a position earlier in the day with locked LP, which would cause a divide by zero below
		activeAtSnapshot := position.SpentTransaction == "" || (position.Slot < maxSlot && position.SpentSlot >= maxSlot)
		if !activeAtSnapshot {
			continue
		}

		for policy, policyMap := range position.Value {

			for assetName, amount := range policyMap {
				assetId := shared.FromSeparate(policy, assetName)

				if poolLookup.IsLPToken(assetId) {

					pool, err := poolLookup.PoolByLPToken(ctx, assetId)
					if err != nil {
						return nil, nil, nil, 0, fmt.Errorf("failed to lookup pool for LP token %v: %w", assetId, err)
					}

					poolsByIdent[pool.PoolIdent] = pool
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
	}
	totalLPByIdent := map[string]uint64{}
	for pool := range lockedLPByIdent {
		totalLPByIdent[pool] = poolsByIdent[pool].TotalLPTokens
	}
	return lockedLPByIdent, totalLPByIdent, valueByIdent, totalValue, nil
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
	if pool.TotalLPTokens == 0 {
		return false, "pool has 0 lp tokens"
	}
	if !atLeastIntegerPercent(locked, pool.TotalLPTokens, program.MinLPIntegerPercent) {
		return false, fmt.Sprintf("less than %v%% of LP tokens locked", program.MinLPIntegerPercent)
	}
	// We'll use a true/false striping to allow/disallow this pool
	// if it's covered by one of the "eligible" criteria, we'll flip it true
	// if it's covered by one of the "disqualified" criteria, we'll flip it back false
	// BUT, if the "eligible" criteria lists are all null, then every pool starts eligible, so this needs to start true
	qualified := program.EligibleVersions == nil &&
		program.EligiblePools == nil &&
		program.EligibleAssets == nil &&
		program.EligiblePairs == nil
	reason := ""
	if program.EligibleVersions != nil {
		found := false
		for _, version := range program.EligibleVersions {
			if version == pool.Version {
				qualified = true
				found = true
				break
			}
		}
		if !found {
			reason += fmt.Sprintf("Program lists eligible versions, but doesn't list this version (%v); ", pool.Version)
		}
	}
	if program.EligiblePools != nil {
		found := false
		for _, poolIdent := range program.EligiblePools {
			if poolIdent == pool.PoolIdent {
				qualified = true
				found = true
				break
			}
		}
		if !found {
			reason += "Program lists eligible pools, but doesn't list this pool; "
		}
	}
	if program.EligibleAssets != nil {
		found := false
		for _, assetID := range program.EligibleAssets {
			if assetID == pool.AssetA || assetID == pool.AssetB {
				qualified = true
				found = true
				break
			}
		}
		if !found {
			reason += "Program lists eligible assets, but doesn't list either asset from this pool; "
		}
	}
	if program.EligiblePairs != nil {
		found := false
		for _, pair := range program.EligiblePairs {
			if pair.AssetA == pool.AssetA && pair.AssetB == pool.AssetB {
				qualified = true
				found = true
				break
			}
		}
		if !found {
			reason += "Program lists eligible pairs, but doesn't list these two assets as an eligible pair; "
		}
	}
	if program.DisqualifiedVersions != nil {
		for _, version := range program.DisqualifiedVersions {
			if version == pool.Version {
				qualified = false
				reason += fmt.Sprintf("Version (%v) is explicitly disqualified; ", pool.Version)
				break
			}
		}
	}
	if program.DisqualifiedPools != nil {
		for _, poolIdent := range program.DisqualifiedPools {
			if poolIdent == pool.PoolIdent {
				qualified = false
				reason += "Pool is explicitly disqualified; "
				break
			}
		}
	}
	if program.DisqualifiedAssets != nil {
		for _, assetID := range program.DisqualifiedAssets {
			if assetID == pool.AssetA || assetID == pool.AssetB {
				qualified = false
				reason += "One of the assets in this pool is explicitly disqualified; "
				break
			}
		}
	}
	if program.DisqualifiedPairs != nil {
		for _, pair := range program.DisqualifiedPairs {
			if pair.AssetA == pool.AssetA && pair.AssetB == pool.AssetB {
				qualified = false
				reason += "Pair is explicitly disqualified; "
				break
			}
		}
	}
	return qualified, reason
}

// Check which pools are disqualified and why, and return just the qualified delegation amounts
func DisqualifyPools(ctx context.Context, program types.Program, lockedLPByPool map[string]uint64, delegationByPool map[string]uint64, poolLookup PoolLookup) (map[string]uint64, map[string]string, error) {
	qualifyingDelegationsPerPool := map[string]uint64{}
	poolDisqualificationReasons := map[string]string{}
	for poolIdent, locked := range lockedLPByPool {
		pool, err := poolLookup.PoolByIdent(ctx, poolIdent)
		if err != nil {
			return nil, nil, fmt.Errorf("failure to lookup pool with ident %v: %w", poolIdent, err)
		}
		qualified, reason := isPoolQualified(program, pool, locked)
		if !qualified {
			if reason != "" {
				poolDisqualificationReasons[poolIdent] = reason
			}
			qualifyingDelegationsPerPool[""] += delegationByPool[poolIdent]
		} else {
			qualifyingDelegationsPerPool[poolIdent] += delegationByPool[poolIdent]
		}
	}
	return qualifyingDelegationsPerPool, poolDisqualificationReasons, nil
}

// Sum up the qualifying delegations over the last several days, to give each pool some "sticking" power
func SumDelegationWindow(program types.Program, qualifyingDelegationsPerPool map[string]uint64, previousCalculations []CalculationOutputs) (map[string]uint64, error) {
	// A 3 day delegation window is today, plus two previous days
	// but, when a program is just starting, we don't have days to base it off of so we use a <
	if (program.ConsecutiveDelegationWindow == 0 && len(previousCalculations) != 0) ||
		program.ConsecutiveDelegationWindow-1 < len(previousCalculations) {
		return nil, fmt.Errorf("too many historical snapshots")
	}
	// Note: we assume the snapshots are from previous consecutive days, since checking this for correctness would be a little awkward

	windowedDelegation := map[string]uint64{}
	for _, snapshot := range previousCalculations {
		for poolIdent, amt := range snapshot.QualifyingDelegationByPool {
			windowedDelegation[poolIdent] += amt
		}
	}
	for poolIdent, amt := range qualifyingDelegationsPerPool {
		windowedDelegation[poolIdent] += amt
	}
	return windowedDelegation, nil
}

// Select the top pools according to the program criteria
func SelectEligiblePoolsForEmission(
	ctx context.Context,
	program types.Program,
	delegationsByPool map[string]uint64,
	poolLookup PoolLookup,
) (map[string]uint64, error) {
	// Convert the map into a list of candidates, so we can sort them
	type Candidate struct {
		PoolIdent string
		Total     uint64
	}
	var candidates []Candidate

	totalDelegation := uint64(0)
	for poolIdent, amt := range delegationsByPool {
		if poolIdent == "" {
			continue
		}
		totalDelegation += amt
		candidates = append(candidates, Candidate{PoolIdent: poolIdent, Total: amt})
	}

	var errs []error
	sort.Slice(candidates, func(i, j int) bool {
		// In the case of an exact tie (very unlikely), prefer the one with less liquidity
		// under the hypothesis that less liquidity needs to attract more liquidity providers
		// (technically wasn't part of the spec, and so we make a reasonable choice)
		if candidates[i].Total == candidates[j].Total {
			poolI, err := poolLookup.PoolByIdent(ctx, candidates[i].PoolIdent)
			if err != nil {
				errs = append(errs, err)
				return false
			}
			poolJ, err := poolLookup.PoolByIdent(ctx, candidates[j].PoolIdent)
			if err != nil {
				errs = append(errs, err)
				return false
			}
			iLP := poolI.TotalLPTokens
			jLP := poolJ.TotalLPTokens
			if iLP == jLP {
				return candidates[i].PoolIdent < candidates[j].PoolIdent
			}
			return iLP < jLP
		}
		return candidates[i].Total > candidates[j].Total
	})
	if len(errs) > 0 {
		return nil, fmt.Errorf("failed to sort candidates; %v errors; first error: %w", len(errs), errs[0])
	}

	poolsReceivingEmissionsByIdent := map[string]uint64{}
	totalQualifyingDelegation := uint64(0)

	// Ensure the nepotism pools (like ADA/SUNDAE) are always selected for emissions
	for _, pool := range program.NepotismPools {
		for _, delegation := range candidates {
			if delegation.PoolIdent == pool {
				poolsReceivingEmissionsByIdent[pool] = delegation.Total
				totalQualifyingDelegation += delegation.Total
			}
		}
	}

	// Then select either the top N pools, or the top covering percent,
	// whichever is fewer
	for _, delegation := range candidates {
		if delegation.PoolIdent == "" {
			continue
		}
		// Don't re-add any nepotistic pools
		if _, ok := poolsReceivingEmissionsByIdent[delegation.PoolIdent]; ok {
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

	return poolsReceivingEmissionsByIdent, nil
}

// Split the daily emissions of the program among a set of pools that have been chosen for emissions
func DistributeEmissionsToPools(program types.Program, poolsEligibleForEmissionsByIdent map[string]uint64) map[string]uint64 {
	// We'll need to loop over pools round-robin by largest value; ordering of maps is non-deterministic
	type Pairs struct {
		PoolIdent string
		Amount    uint64
	}
	poolWeights := []Pairs{}
	totalWeight := uint64(0)
	emissionsByPool := map[string]uint64{}

	allocatedEmissions := uint64(0)
	// First, add in any fixed-emissions pools
	for poolIdent, amount := range program.FixedEmissions {
		if program.DailyEmission < amount {
			panic("program is misconfigured, fixed emissions exceed daily emissions")
		}
		emissionsByPool[poolIdent] = amount
		allocatedEmissions += amount
	}

	// Now, sum up the various pool weights that are remaining
	for poolIdent, weight := range poolsEligibleForEmissionsByIdent {
		// Skip over pools that are receiving a fixed emission
		if _, ok := program.FixedEmissions[poolIdent]; ok {
			continue
		}

		totalWeight += weight
		poolWeights = append(poolWeights, Pairs{PoolIdent: poolIdent, Amount: weight})
	}

	// No pool has received weight
	// We then divide the daily emissions among these pools in proportion to their weight, rounding down
	if totalWeight == 0 {
		return emissionsByPool
	}

	// Then allocate the remainder according to the rules of the program
	dynamicEmissions := program.DailyEmission - allocatedEmissions
	for poolIdent, weight := range poolsEligibleForEmissionsByIdent {
		// Skip over pools that receive a fixed emission
		if _, ok := program.FixedEmissions[poolIdent]; ok {
			continue
		}
		frac := big.NewInt(0).SetUint64(dynamicEmissions)
		frac = frac.Mul(frac, big.NewInt(0).SetUint64(weight))
		frac = frac.Div(frac, big.NewInt(0).SetUint64(totalWeight))
		allocation := frac.Uint64()
		if allocatedEmissions+allocation > program.DailyEmission {
			panic("something went wrong: would allocate more than daily emissions")
		}
		emissionsByPool[poolIdent] += allocation
		allocatedEmissions += allocation
	}

	// and distributing [diminutive tokens] among them until the daily emission is accounted for.
	if allocatedEmissions > program.DailyEmission {
		panic("something went wrong with allocating emissions to pool; exceeded the daily emissions")
	} else if allocatedEmissions != program.DailyEmission {
		sort.Slice(poolWeights, func(i, j int) bool {
			if poolWeights[i].Amount == poolWeights[j].Amount {
				return poolWeights[i].PoolIdent < poolWeights[j].PoolIdent
			}
			return poolWeights[i].Amount > poolWeights[j].Amount
		})
		remainder := int(program.DailyEmission - allocatedEmissions)
		for i := 0; i < remainder; i++ {
			pool := poolWeights[i%len(poolWeights)]
			emissionsByPool[pool.PoolIdent] += 1
			allocatedEmissions += 1
		}
		if allocatedEmissions != program.DailyEmission {
			// There's a bug in the round-robin distribution code, panic so we fix the bug
			panic("round-robin distribution wasn't succesful")
		}
	}

	// Now check to make sure none of these pools (other than the fixed emissions) exceed the cap on daily emissions per pool

	return emissionsByPool
}

// Truncate the emissions to the maximum emission cap
func TruncateEmissions(program types.Program, emissionsByPool map[string]uint64) map[string]uint64 {
	if program.EmissionCap == 0 {
		return emissionsByPool
	}

	truncatedEmissions := map[string]uint64{}
	for pool, amount := range emissionsByPool {
		_, ok := program.FixedEmissions[pool]
		if !ok && amount > program.EmissionCap {
			truncatedEmissions[pool] = program.EmissionCap
		} else {
			truncatedEmissions[pool] = amount
		}
	}

	return truncatedEmissions
}

// Compute the total LP token days that each owner has; We multiply the LP tokens by seconds they were locked, and then divide by 86400.
// This effectively divides the LP tokens by the fraction of the day they are locked, to prevent someone locking in the last minute of the day to receive rewards
func TotalLPDaysByOwnerAndAsset(positions []types.Position, poolLookup PoolLookup, minSlot uint64, maxSlot uint64) (map[string]map[shared.AssetID]uint64, map[shared.AssetID]uint64) {
	lpDaysByOwner := map[string]map[shared.AssetID]uint64{}
	lpDaysByAsset := map[shared.AssetID]uint64{}
	for _, p := range positions {
		for policy, policyMap := range p.Value {
			for assetName, amount := range policyMap {
				assetId := shared.FromSeparate(policy, assetName)

				if poolLookup.IsLPToken(assetId) {
					// Compute the (truncated) start and end time,
					startTime := p.Slot
					if startTime < minSlot {
						startTime = minSlot
					}
					endTime := p.SpentSlot
					if p.SpentTransaction == "" || p.SpentSlot > maxSlot {
						endTime = maxSlot
					}
					if endTime == startTime {
						continue
					}
					// so we can compute what fraction of the day this position counts for
					secondsLocked := endTime - startTime

					weight := big.NewInt(0).SetUint64(secondsLocked)
					weight = weight.Mul(weight, amount.BigInt())
					weight = weight.Div(weight, big.NewInt(0).SetUint64(maxSlot-minSlot))

					existingLPDays, ok := lpDaysByOwner[p.OwnerID]
					if !ok {
						existingLPDays = map[shared.AssetID]uint64{}
					}
					newWeight := existingLPDays[assetId] + weight.Uint64()

					existingLPDays[assetId] = newWeight
					lpDaysByOwner[p.OwnerID] = existingLPDays

					lpDaysByAsset[assetId] += weight.Uint64()
				}
			}
		}
	}
	return lpDaysByOwner, lpDaysByAsset
}

// Switch the map key from pool Ident to LP token
func RegroupByAsset(ctx context.Context, byPool map[string]uint64, poolLookup PoolLookup) (map[shared.AssetID]uint64, error) {
	byLPAsset := map[shared.AssetID]uint64{}
	for poolIdent, amount := range byPool {
		if amount == 0 {
			continue
		}
		pool, err := poolLookup.PoolByIdent(ctx, poolIdent)
		if err != nil {
			return nil, fmt.Errorf("unable to lookup pool for ident %v: %w", poolIdent, err)
		}
		byLPAsset[pool.LPAsset] = amount
	}
	return byLPAsset, nil
}

// Switch the map key from LP token to pool Ident
func RegroupByPool(ctx context.Context, byAsset map[shared.AssetID]uint64, poolLookup PoolLookup) (map[string]uint64, error) {
	// Note: assumes bijection
	byIdent := map[string]uint64{}
	for asset, amount := range byAsset {
		if amount == 0 {
			continue
		}
		pool, err := poolLookup.PoolByLPToken(ctx, asset)
		if err != nil {
			return nil, fmt.Errorf("failed to lookup pool for LP token %v: %w", asset, err)
		}
		byIdent[pool.PoolIdent] = amount
	}
	return byIdent, nil
}

// Split the daily emissions of each pool among the owners of LP tokens, according to their total LP weight
func DistributeEmissionsToOwners(lpWeightByOwner map[string]map[shared.AssetID]uint64, emissionsByAsset map[shared.AssetID]uint64, lpTokensByAsset map[shared.AssetID]uint64) map[string]map[string]uint64 {
	// expand out the lpTokensByOwner, so we can sort them canonically for the round-robin
	type OwnerStake struct {
		OwnerID string
		Value   map[shared.AssetID]uint64
	}
	var ownerStakes []OwnerStake
	for ownerId, weights := range lpWeightByOwner {
		ownerStakes = append(ownerStakes, OwnerStake{OwnerID: ownerId, Value: weights})
	}
	// We sort by owner key here; in theory we could sort by "total value staked", but it's
	// very difficult to compare that here, so we just sort by owner;
	// This can result in at most one diminutive unit of a token, ex. 1 millionth of a SUNDAE
	// if we ever distribute a token that has a high diminutive value (like an XDIAMOND), we may need to revist this
	sort.Slice(ownerStakes, func(i, j int) bool {
		return ownerStakes[i].OwnerID < ownerStakes[j].OwnerID
	})

	// Now loop over each, and allocate a portion of the emissions
	emissionsByOwner := map[string]map[string]uint64{}
	allocatedByAsset := map[shared.AssetID]uint64{}
	for _, ownerStake := range ownerStakes {
		for assetId, amount := range ownerStake.Value {
			emission := emissionsByAsset[assetId]
			totalLP := lpTokensByAsset[assetId]
			if totalLP == 0 {
				continue
			}
			frac := big.NewInt(0).SetUint64(emission)
			frac = frac.Mul(frac, big.NewInt(0).SetUint64(amount))
			frac = frac.Div(frac, big.NewInt(0).SetUint64(totalLP))
			allocation := frac.Uint64()
			if allocation == 0 {
				continue
			}
			existing, ok := emissionsByOwner[ownerStake.OwnerID]
			if !ok {
				existing = map[string]uint64{}
			}
			existing[assetId.String()] += allocation
			emissionsByOwner[ownerStake.OwnerID] = existing
			allocatedByAsset[assetId] += allocation
		}
	}
	// The emissions for each owner will be rounded down, and millionths of a SUNDAE distributed round-robin until
	// the total user emissions match the pool emissions.
	for assetId, allocatedAmount := range allocatedByAsset {
		remainder := int(emissionsByAsset[assetId] - allocatedAmount)
		if remainder < 0 {
			panic(fmt.Sprintf("emitted %v more to users than the allocated emissions for a pool, somehow", remainder))
		} else if remainder > 0 {
			i := 0
			for remainder > 0 {
				owner := ownerStakes[i%len(ownerStakes)]
				m := emissionsByOwner[owner.OwnerID]
				i += 1
				// Pick the min LP token assset ID to add one token to
				// It doesn't actually matter which one we add it to, but this makes it determinsitic
				minLP := ""
				for asset := range m {
					if _, ok := emissionsByAsset[shared.AssetID(asset)]; ok && (minLP == "" || asset < minLP) {
						minLP = asset
					}
				}
				// If we didn't find anything, the user likely isn't qualified to receive emissions for *any* LP token, so skip over them
				if minLP == "" {
					continue
				}
				emissionsByOwner[owner.OwnerID][minLP] += 1
				allocatedAmount += 1
				remainder -= 1
			}
			if emissionsByAsset[assetId] != allocatedAmount {
				panic("round-robin distribution wasn't succesful")
			}
		}
	}
	return emissionsByOwner
}

// Convert a set of emissions records into actual earnings we can save in a database
func EmissionsByOwnerToEarnings(date types.Date, program types.Program, emissionsByOwner map[string]map[string]uint64, ownersByID map[string]types.MultisigScript) ([]types.Earning, map[string]uint64) {
	var ret []types.Earning
	total := map[string]uint64{}
	for ownerID, perLPToken := range emissionsByOwner {
		ownerValue := shared.ValueFromCoins(shared.Coin{AssetId: program.EmittedAsset, Amount: num.Uint64(0)})

		ownerValueByLP := map[string]compatibility.CompatibleValue{}
		for lpToken, amount := range perLPToken {
			ownerValue.AddAsset(shared.Coin{AssetId: program.EmittedAsset, Amount: num.Uint64(amount)})
			if amount > 0 {
				coinValue := shared.ValueFromCoins(shared.Coin{AssetId: program.EmittedAsset, Amount: num.Uint64(amount)})

				ownerValueByLP[lpToken] = compatibility.CompatibleValue(coinValue)
			}
			total[ownerID] += amount
		}
		if ownerValue.AssetAmount(program.EmittedAsset).Uint64() == 0 {
			continue
		}
		earning := types.Earning{
			OwnerID:        ownerID,
			Owner:          ownersByID[ownerID],
			Program:        program.ID,
			EarnedDate:     date,
			Value:          compatibility.CompatibleValue(ownerValue),
			ValueByLPToken: ownerValueByLP,
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
	return ret, total
}

type CalculationOutputs struct {
	Timestamp string

	TotalDelegations uint64
	DelegationByPool map[string]uint64

	QualifyingDelegationByPool  map[string]uint64
	PoolDisqualificationReasons map[string]string

	NumDelegationDays          int
	DelegationOverWindowByPool map[string]uint64

	PoolsEligibleForEmissions map[string]uint64

	LockedLPByPool map[string]uint64
	TotalLPByPool  map[string]uint64

	EstimatedLockedLovelace       uint64
	EstimatedLockedLovelaceByPool map[string]uint64

	TotalEmissions             uint64
	UntruncatedEmissionsByPool map[string]uint64
	EmissionsByPool            map[string]uint64

	EmissionsByOwner map[string]uint64

	EstimatedEmissionsLovelaceValue  uint64
	EstimatedEmissionsLovelaceByPool map[string]uint64

	Earnings []types.Earning
}

func CalculateEarnings(ctx context.Context, date types.Date, startSlot uint64, endSlot uint64, program types.Program, previousResults []CalculationOutputs, positions []types.Position, poolLookup PoolLookup) (CalculationOutputs, error) {
	// Check for start and end dates, inclusive
	if date < program.FirstDailyRewards {
		return CalculationOutputs{}, nil
	}
	if program.LastDailyRewards != "" && date > program.LastDailyRewards {
		return CalculationOutputs{}, nil
	}

	// To calculate the daily emissions, ... first take inventory of SUNDAE held at the Locking Contract
	// and factor in the users delegation
	delegationByPool, totalDelegation, err := CalculateTotalDelegations(ctx, program, positions, poolLookup)
	if err != nil {
		return CalculationOutputs{}, fmt.Errorf("failed to calculate total delegations: %w", err)
	}

	// Sum up the LP-seconds per pool, and estimate the value
	lockedLPByPool, totalLPByPool, estimatedValueByPool, totalEstimatedValue, err := CalculateTotalLPAtSnapshot(ctx, endSlot, positions, poolLookup)
	if err != nil {
		return CalculationOutputs{}, fmt.Errorf("failed to calculate total LP: %w", err)
	}

	// Disqualify any pools that need disqualification
	qualifyingDelegationsPerPool, poolDisqualificationReasons, err := DisqualifyPools(ctx, program, lockedLPByPool, delegationByPool, poolLookup)
	if err != nil {
		return CalculationOutputs{}, fmt.Errorf("failed to check for pool disqualification: %w", err)
	}

	// Sum up the delegation window for any previously used snapshots
	delegationOverWindowByPool, err := SumDelegationWindow(program, qualifyingDelegationsPerPool, previousResults)
	if err != nil {
		return CalculationOutputs{}, fmt.Errorf("failed to sum delegation window: %w", err)
	}

	// If no pools are qualified (extremely degenerate case, return no earnings, and reserve those tokens for the treasury)
	if _, ok := delegationOverWindowByPool[""]; len(delegationOverWindowByPool) == 0 || (ok && len(delegationOverWindowByPool) == 1) {
		return CalculationOutputs{
			Timestamp:                     time.Now().Format(time.RFC3339),
			TotalDelegations:              totalDelegation,
			DelegationByPool:              delegationByPool,
			NumDelegationDays:             program.ConsecutiveDelegationWindow,
			QualifyingDelegationByPool:    qualifyingDelegationsPerPool,
			DelegationOverWindowByPool:    delegationOverWindowByPool,
			PoolDisqualificationReasons:   poolDisqualificationReasons,
			LockedLPByPool:                lockedLPByPool,
			TotalLPByPool:                 totalLPByPool,
			EstimatedLockedLovelace:       totalEstimatedValue,
			EstimatedLockedLovelaceByPool: estimatedValueByPool,
		}, nil
	}

	// The top pools ... will be eligible for yield farming rewards that day.
	poolsEligibleForEmissions, err := SelectEligiblePoolsForEmission(ctx, program, delegationOverWindowByPool, poolLookup)
	if err != nil {
		return CalculationOutputs{}, fmt.Errorf("failed to select pools for emission: %w", err)
	}

	// We then divide the daily emissions among these pools ...
	rawEmissionsByPool := DistributeEmissionsToPools(program, poolsEligibleForEmissions)
	emissionsByPool := TruncateEmissions(program, rawEmissionsByPool)
	emissionsByAsset, err := RegroupByAsset(ctx, emissionsByPool, poolLookup)
	if err != nil {
		return CalculationOutputs{}, fmt.Errorf("failed to regroup emissions by asset: %w", err)
	}

	// For each pool, SundaeSwap labs will then calculate the allocation of rewards in proportion to the LP tokens held at the Locking Contract.
	lpDaysByOwner, lpTokensByAsset := TotalLPDaysByOwnerAndAsset(positions, poolLookup, startSlot, endSlot)

	emissionsByOwner := DistributeEmissionsToOwners(lpDaysByOwner, emissionsByAsset, lpTokensByAsset)

	ownersByID := map[string]types.MultisigScript{}
	for _, position := range positions {
		ownersByID[position.OwnerID] = position.Owner
	}

	// Find the pool that we should use for price reference, so we can estimate the ADA value of what was emitted
	var emittedLovelaceValue uint64
	emittedLovelaceValueByPool := map[string]uint64{}
	if program.ReferencePool != "" {
		referencePool, err := poolLookup.PoolByIdent(ctx, program.ReferencePool)
		if err != nil {
			return CalculationOutputs{}, fmt.Errorf("failure to fetch reference pool for pricing %v: %w", program.ReferencePool, err)
		}

		for ident, sundae := range emissionsByPool {
			// NOTE: assuming AssetA is ADA
			estimatedNumerator := big.NewInt(0).Mul(big.NewInt(int64(sundae)), big.NewInt(int64(referencePool.AssetAQuantity)))
			estimatedLovelaceValue := big.NewInt(0).Div(estimatedNumerator, big.NewInt(int64(referencePool.AssetBQuantity)))
			emittedLovelaceValue += estimatedLovelaceValue.Uint64()
			emittedLovelaceValueByPool[ident] += estimatedLovelaceValue.Uint64()
		}
	}

	// Users will be able to claim these emitted tokens
	// we return a set of "earnings" for the day
	earnings, perOwnerTotal := EmissionsByOwnerToEarnings(date, program, emissionsByOwner, ownersByID)

	totalEmissions := uint64(0)
	for _, byPool := range emissionsByOwner {
		for _, amount := range byPool {
			totalEmissions += amount
		}
	}

	return CalculationOutputs{
		Timestamp: time.Now().Format(time.RFC3339),

		TotalDelegations: totalDelegation,
		DelegationByPool: delegationByPool,

		QualifyingDelegationByPool:  qualifyingDelegationsPerPool,
		PoolDisqualificationReasons: poolDisqualificationReasons,

		NumDelegationDays:          program.ConsecutiveDelegationWindow,
		DelegationOverWindowByPool: delegationOverWindowByPool,

		PoolsEligibleForEmissions: poolsEligibleForEmissions,

		LockedLPByPool: lockedLPByPool,
		TotalLPByPool:  totalLPByPool,

		EstimatedLockedLovelace:       totalEstimatedValue,
		EstimatedLockedLovelaceByPool: estimatedValueByPool,

		TotalEmissions:             totalEmissions,
		UntruncatedEmissionsByPool: rawEmissionsByPool,
		EmissionsByPool:            emissionsByPool,

		EmissionsByOwner: perOwnerTotal,

		EstimatedEmissionsLovelaceValue:  emittedLovelaceValue,
		EstimatedEmissionsLovelaceByPool: emittedLovelaceValueByPool,

		Earnings: earnings,
	}, nil
}
