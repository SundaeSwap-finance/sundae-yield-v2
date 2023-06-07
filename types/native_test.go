package types

import (
	"encoding/hex"
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
		cbor   []byte
		script NativeScript
	}
	testCases := []testCase{
		{
			label:  "Signature",
			cbor:   mustDecode(t, "d8799f581c6a5cf1e931c3bd034543b93ef9731cf16847e038b020033db359786dff"),
			script: NativeScript{Signature: &Signature{KeyHash: mustDecode(t, "6a5cf1e931c3bd034543b93ef9731cf16847e038b020033db359786d")}},
		},
		{
			label: "AllOf",
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
		t.Run(tc.label, func(t *testing.T) {
			var ns NativeScript
			assert.Nil(t, cbor.Unmarshal(tc.cbor, &ns))
			assert.EqualValues(t, tc.script, ns)
		})
		/*
			t.Run(tc.label, func(t *testing.T) {
				bytes, err := cbor.Marshal(tc.script)
				assert.Nil(t, err)
				fmt.Printf("%v\n", hex.EncodeToString(bytes))
				assert.EqualValues(t, tc.cbor, bytes)
			})
		*/
	}
}
