package types

import (
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/tj/assert"
)

func Test_UnmarshalDatum(t *testing.T) {
	bytes := mustDecode(t, "d8799fd8799f581c6a5cf1e931c3bd034543b93ef9731cf16847e038b020033db359786dff9fd8799f41080affd8799f41090fffffff")
	var datum StakeDatum
	assert.Nil(t, cbor.Unmarshal(bytes, &datum))
	assert.EqualValues(t, StakeDatum{
		Owner: NativeScript{Signature: &Signature{KeyHash: mustDecode(t, "6a5cf1e931c3bd034543b93ef9731cf16847e038b020033db359786d")}},
		Delegations: []Delegation{
			{PoolIdent: "08", Weight: 10},
			{PoolIdent: "09", Weight: 15},
		},
	}, datum)
}
