package types

import (
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/tj/assert"
)

func Test_UnmarshalDatum(t *testing.T) {
	bytes := mustDecode(t, "d8799fd8799f581cc279a3fb3b4e62bbc78e288783b58045d4ae82a18867d8352d02775aff9fd8799f46524245525259410105ffd8799f46524245525259410d02ffd8799f46534245525259410101ffffff")
	var datum StakeDatum
	assert.Nil(t, cbor.Unmarshal(bytes, &datum))
	assert.EqualValues(t, StakeDatum{
		Owner: MultisigScript{Signature: &Signature{KeyHash: mustDecode(t, "c279a3fb3b4e62bbc78e288783b58045d4ae82a18867d8352d02775a")}},
		Delegations: []Delegation{
			{Program: "RBERRY", PoolIdent: "01", Weight: 5},
			{Program: "RBERRY", PoolIdent: "0d", Weight: 2},
			{Program: "SBERRY", PoolIdent: "01", Weight: 1},
		},
	}, datum)
}
func Test_UnmarshalNil(t *testing.T) {
	bytes := mustDecode(t, "d8799fd8799f581c121fd22e0b57ac206fefc763f8bfa0771919f5218b40691eea4514d0ff80ff")
	var datum StakeDatum
	assert.Nil(t, cbor.Unmarshal(bytes, &datum))
	assert.EqualValues(t, StakeDatum{
		Owner:       MultisigScript{Signature: &Signature{KeyHash: mustDecode(t, "121fd22e0b57ac206fefc763f8bfa0771919f5218b40691eea4514d0")}},
		Delegations: nil,
	}, datum)
}
