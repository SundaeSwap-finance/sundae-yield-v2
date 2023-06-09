package types

import (
	"encoding/hex"

	"github.com/fxamacker/cbor/v2"
)

type StakeDatum struct {
	_           struct{} `cbor:",toarray"`
	Owner       MultisigScript
	Delegations []Delegation
}

func (s *StakeDatum) UnmarshalCBOR(bytes []byte) error {
	var rawTag cbor.RawTag
	if err := cbor.Unmarshal(bytes, &rawTag); err != nil {
		return err
	}
	var intermediate struct {
		_           struct{} `cbor:",toarray"`
		Owner       MultisigScript
		Delegations []cbor.RawTag
	}
	if err := cbor.Unmarshal(rawTag.Content, &intermediate); err != nil {
		return err
	}
	s.Owner = intermediate.Owner
	for _, rawDelegation := range intermediate.Delegations {
		var delegation struct {
			_         struct{} `cbor:",toarray"`
			PoolIdent []byte
			Weight    uint32
		}
		if err := cbor.Unmarshal(rawDelegation.Content, &delegation); err != nil {
			return err
		}
		s.Delegations = append(s.Delegations, Delegation{
			PoolIdent: hex.EncodeToString(delegation.PoolIdent),
			Weight:    delegation.Weight,
		})
	}
	return nil
}
