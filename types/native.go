package types

import (
	"fmt"
	"time"

	"github.com/fxamacker/cbor/v2"
)

type NativeScript struct {
	Signature *Signature
	AllOf     *AllOf
	AnyOf     *AnyOf
	AtLeast   *AtLeast
	Before    *Before
	After     *After
}

func (n NativeScript) Hash() string {
	// TODO: hash
	return ""
}

const tagBase = 120

func (n *NativeScript) UnmarshalCBOR(b []byte) error {
	var rawTag cbor.RawTag
	if err := cbor.Unmarshal(b, &rawTag); err != nil {
		return err
	}
	switch rawTag.Number - tagBase {
	case 1:
		return cbor.Unmarshal(rawTag.Content, &n.Signature)
	case 2:
		return cbor.Unmarshal(rawTag.Content, &n.AllOf)
	case 3:
		return cbor.Unmarshal(rawTag.Content, &n.AnyOf)
	case 4:
		return cbor.Unmarshal(rawTag.Content, &n.AtLeast)
	case 5:
		return cbor.Unmarshal(rawTag.Content, &n.Before)
	case 6:
		return cbor.Unmarshal(rawTag.Content, &n.After)
	default:
		return fmt.Errorf("unrecognized tag %v", rawTag.Number-tagBase)
	}
}

type Signature struct {
	_       struct{} `cbor:",toarray"`
	KeyHash []byte
}

type AllOf struct {
	_       struct{} `cbor:",toarray"`
	Scripts []NativeScript
}

type AnyOf struct {
	_       struct{} `cbor:",toarray"`
	Scripts []NativeScript
}

type AtLeast struct {
	_        struct{} `cbor:",toarray"`
	Required int
	Scripts  []NativeScript
}

type Before struct {
	_    struct{} `cbor:",toarray"`
	Time time.Time
}

type After struct {
	_    struct{} `cbor:",toarray"`
	Time time.Time
}
