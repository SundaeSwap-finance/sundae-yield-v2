package incentive

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

type CalculationOutputs struct {
	Timestamp                 string
	StartDate                 types.Date
	EndDate                   types.Date
	TotalEmissions            uint64
	EmittedAsset              string
	EmittedAssetLovelaceValue uint64
	StakedAssetLovelaceValue  uint64

	TotalDelegators  uint64
	DelegatorWeights map[string]uint64
	EmissionsByOwner map[string]uint64

	Earnings []types.Earning
}

func PositionsToOwners(positions []types.Position) map[string]types.MultisigScript {
	ownersById := map[string]types.MultisigScript{}
	for _, position := range positions {
		ownersById[position.OwnerID] = position.Owner
	}
	return ownersById
}

func CalculateDelegationWeights(
	ctx context.Context,
	program types.IncentiveProgram,
	positions []types.Position,
	startSlot, endSlot uint64,
	poolLookup types.PoolLookup,
) (map[string]uint64, num.Int, error) {
	delegationWeightByOwner := map[string]uint64{}
	windowLength := num.Uint64(endSlot - startSlot)
	total := num.Uint64(0)
	for _, position := range positions {
		if len(position.Delegation) == 0 {
			continue
		}
		truncatedStart := position.Slot
		if truncatedStart < startSlot {
			truncatedStart = startSlot
		}
		truncatedEnd := position.SpentSlot
		if position.SpentTransaction == "" || truncatedEnd > endSlot {
			truncatedEnd = endSlot
		}
		programValue := shared.Value(position.Value)
		stakedAsset := programValue.AssetAmount(program.StakedAsset)
		for policyId, assets := range programValue {
			for assetName, amount := range assets {
				assetId := shared.FromSeparate(policyId, assetName)
				if poolLookup.IsLPToken(assetId) {

					pool, err := poolLookup.PoolByLPToken(ctx, assetId)
					if err != nil {
						return nil, num.Int64(0), fmt.Errorf("failed to lookup pool for LP token %v: %w", assetId, err)
					}

					if pool.AssetA == program.StakedAsset || pool.AssetB == program.StakedAsset {
						frac := big.NewInt(amount.Int64())
						if pool.AssetA == program.StakedAsset {
							frac = frac.Mul(frac, big.NewInt(int64(pool.AssetAQuantity)))
						} else if pool.AssetB == program.StakedAsset {
							frac = frac.Mul(frac, big.NewInt(int64(pool.AssetBQuantity)))
						}
						frac = frac.Div(frac, big.NewInt(int64(pool.TotalLPTokens)))
						stakedAsset = stakedAsset.Add(num.Int(*frac))
					}
				}
			}
		}

		positionLength := num.Uint64(truncatedEnd - truncatedStart)
		numerator := stakedAsset.Mul(positionLength)
		weight := numerator.Div(windowLength)
		if weight.Uint64() == 0 {
			continue
		}
		total = total.Add(weight)
		delegationWeightByOwner[position.OwnerID] += weight.Uint64()
	}
	return delegationWeightByOwner, total, nil
}

func SplitEmissionPerOwner(
	emission uint64,
	weightByOwner map[string]uint64,
	total num.Int,
) map[string]uint64 {
	emissionByOwner := map[string]uint64{}
	owners := []string{}
	remainder := emission
	for owner, weight := range weightByOwner {
		owners = append(owners, owner)
		amt := num.Uint64(weight)
		amt = amt.Mul(num.Uint64(emission))
		amt = amt.Div(total)
		remainder -= amt.Uint64()
		emissionByOwner[owner] = amt.Uint64()
	}
	sort.Slice(owners, func(i, j int) bool {
		return emissionByOwner[owners[i]] < emissionByOwner[owners[j]]
	})
	if remainder > 0 {
		for _, owner := range owners {
			emissionByOwner[owner] += 1
			remainder -= 1
			if remainder <= 0 {
				break
			}
		}
	}
	return emissionByOwner
}

func EmissionsToEarnings(
	program types.IncentiveProgram,
	date types.Date,
	emissionsByOwner map[string]uint64,
	ownersById map[string]types.MultisigScript,
) []types.Earning {
	earnings := []types.Earning{}
	for owner, emission := range emissionsByOwner {
		ownerValue := shared.ValueFromCoins(shared.Coin{
			AssetId: program.EmittedAsset,
			Amount:  num.Uint64(emission),
		})
		earning := types.Earning{
			OwnerID:        owner,
			Owner:          ownersById[owner],
			Program:        program.ID,
			EarnedDate:     date,
			Value:          compatibility.CompatibleValue(ownerValue),
			ValueByLPToken: nil,
		}
		earnings = append(earnings, earning)
	}
	return earnings
}

func EstimateLovelaceValue(
	ctx context.Context,
	amount uint64,
	asset shared.AssetID,
	poolIdent string,
	poolLookup types.PoolLookup,
) (uint64, error) {
	pool, err := poolLookup.PoolByIdent(ctx, poolIdent)
	if err != nil {
		return 0, fmt.Errorf("Failed to lookup pool %v: %w", poolIdent, err)
	}
	if pool.AssetA != "" && pool.AssetA != shared.AdaAssetID {
		return 0, fmt.Errorf("Reference pool must be an ADA pool")
	}
	if asset == shared.AdaAssetID {
		return amount, nil
	} else if asset == pool.AssetB {
		return num.Uint64(amount).
			Mul(num.Uint64(pool.AssetAQuantity)).
			Div(num.Uint64(pool.AssetBQuantity)).
			Uint64(), nil
	} else {
		return 0, fmt.Errorf("Pool %v for the wrong asset: %v", poolIdent, pool.AssetB)
	}
}

func CalculateEarnings(
	ctx context.Context,
	startDate, endDate types.Date,
	startSlot, endSlot uint64,
	emission uint64,
	program types.IncentiveProgram,
	positions []types.Position,
	poolLookup types.PoolLookup,
) (CalculationOutputs, error) {
	weightByOwner, total, err := CalculateDelegationWeights(ctx, program, positions, startSlot, endSlot, poolLookup)
	if err != nil {
		return CalculationOutputs{}, fmt.Errorf("Failed to calculate delegation by weights")
	}
	emissionsByOwner := SplitEmissionPerOwner(emission, weightByOwner, total)
	ownersById := PositionsToOwners(positions)
	earnings := EmissionsToEarnings(program, endDate, emissionsByOwner, ownersById)

	emittedLovelaceValue, err := EstimateLovelaceValue(ctx, emission, program.EmittedAsset, program.EmittedReferencePool, poolLookup)
	if err != nil {
		return CalculationOutputs{}, fmt.Errorf("Failed to estimate lovelace value: %w", err)
	}
	stakedLovelaceValue, err := EstimateLovelaceValue(ctx, total.Uint64(), program.StakedAsset, program.StakedReferencePool, poolLookup)
	if err != nil {
		return CalculationOutputs{}, fmt.Errorf("Failed to estimate lovelace value: %w", err)
	}
	return CalculationOutputs{
		Timestamp:                 time.Now().Format(time.RFC3339),
		StartDate:                 startDate,
		EndDate:                   endDate,
		TotalEmissions:            emission,
		EmittedAssetLovelaceValue: emittedLovelaceValue,
		StakedAssetLovelaceValue:  stakedLovelaceValue,
		EmittedAsset:              string(program.EmittedAsset),
		TotalDelegators:           uint64(len(weightByOwner)),
		DelegatorWeights:          weightByOwner,
		EmissionsByOwner:          emissionsByOwner,
		Earnings:                  earnings,
	}, nil
}
