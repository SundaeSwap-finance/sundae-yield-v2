package types

import (
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/tj/assert"
)

func mustDecode(t *testing.T, s string) []byte {
	b, err := hex.DecodeString(s)
	assert.Nil(t, err)
	return b
}

func Test_MarshalUnmarshalNativeScript(t *testing.T) {
	type testCase struct {
		label  string
		hash   string
		cbor   []byte
		script NativeScript
	}
	testCases := []testCase{
		{
			label:  "Signature",
			hash:   "502c201094ff5b44c4d9c60077241448786c577005b48c9f49960027",
			cbor:   mustDecode(t, "d8799f581c6a5cf1e931c3bd034543b93ef9731cf16847e038b020033db359786dff"),
			script: NativeScript{Signature: &Signature{KeyHash: mustDecode(t, "6a5cf1e931c3bd034543b93ef9731cf16847e038b020033db359786d")}},
		},
		{
			label: "AllOf",
			hash:  "fafaceadccb45bea127890e3dc78701acc9c9b3a40cd57b9adef7389",
			cbor:  mustDecode(t, "d87a9f9fd8799f581c6a5cf1e931c3bd034543b93ef9731cf16847e038b020033db359786dffd8799f581c6a5cf1e931c3bd034543b93ef9731cf16847e038b020033db359786dffffff"),
			script: NativeScript{AllOf: &AllOf{
				Scripts: []NativeScript{
					{Signature: &Signature{KeyHash: mustDecode(t, "6a5cf1e931c3bd034543b93ef9731cf16847e038b020033db359786d")}},
					{Signature: &Signature{KeyHash: mustDecode(t, "6a5cf1e931c3bd034543b93ef9731cf16847e038b020033db359786d")}},
				}},
			},
		},
		{
			label: "AtLeast",
			hash:  "8558654b4edfb60dd19bcc568606b652ea4219a089d2d89fb3d526b2",
			cbor:  mustDecode(t, "d87c9f029fd8799f581c6a5cf1e931c3bd034543b93ef9731cf16847e038b020033db359786dffd8799f581c6a5cf1e931c3bd034543b93ef9731cf16847e038b020033db359786dffffff"),
			script: NativeScript{AtLeast: &AtLeast{
				Required: 2,
				Scripts: []NativeScript{
					{Signature: &Signature{KeyHash: mustDecode(t, "6a5cf1e931c3bd034543b93ef9731cf16847e038b020033db359786d")}},
					{Signature: &Signature{KeyHash: mustDecode(t, "6a5cf1e931c3bd034543b93ef9731cf16847e038b020033db359786d")}},
				}},
			},
		},
		{
			label: "AtLeastBefore",
			hash:  "0b9fd8a86eca6a8c88920ff74cdf60f22d30d4dbf22e5b5741ffb1a8",
			cbor:  mustDecode(t, "d87c9f029fd8799f581c6a5cf1e931c3bd034543b93ef9731cf16847e038b020033db359786dffd8799f581c6a5cf1e931c3bd034543b93ef9731cf16847e038b020033db359786dffd87d9f1a647d6573ffffff"),
			script: NativeScript{AtLeast: &AtLeast{
				Required: 2,
				Scripts: []NativeScript{
					{Signature: &Signature{KeyHash: mustDecode(t, "6a5cf1e931c3bd034543b93ef9731cf16847e038b020033db359786d")}},
					{Signature: &Signature{KeyHash: mustDecode(t, "6a5cf1e931c3bd034543b93ef9731cf16847e038b020033db359786d")}},
					{Before: &Before{Time: time.Unix(1685939571, 0)}},
				}},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%v Unmarshal", tc.label), func(t *testing.T) {
			var ns NativeScript
			assert.Nil(t, cbor.Unmarshal(tc.cbor, &ns))
			assert.EqualValues(t, tc.script, ns)
		})
		t.Run(fmt.Sprintf("%v Marshal", tc.label), func(t *testing.T) {
			bytes, err := cbor.Marshal(tc.script)
			assert.Nil(t, err)
			assert.EqualValues(t, tc.cbor, bytes)
		})
		t.Run(fmt.Sprintf("%v Hash", tc.label), func(t *testing.T) {
			hash, err := tc.script.Hash()
			assert.Nil(t, err)
			assert.EqualValues(t, tc.hash, hash)
		})
	}
}
