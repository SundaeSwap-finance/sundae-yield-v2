package types

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/fxamacker/cbor/v2"
	"golang.org/x/crypto/blake2b"
)

type MultisigScript struct {
	Signature *Signature `json:"signature,omitempty" dynamodbav:"signature,omitempty"`
	AllOf     *AllOf     `json:"allOf,omitempty" dynamodbav:"allOf,omitempty"`
	AnyOf     *AnyOf     `json:"anyOf,omitempty" dynamodbav:"anyOf,omitempty"`
	AtLeast   *AtLeast   `json:"atLeast,omitempty" dynamodbav:"atLeast,omitempty"`
	Before    *Before    `json:"before,omitempty" dynamodbav:"before,omitempty"`
	After     *After     `json:"after,omitempty" dynamodbav:"after,omitempty"`
}

func (n MultisigScript) Hash() (string, error) {
	bytes, err := cbor.Marshal(&n)
	if err != nil {
		return "", err
	}
	b2, err := blake2b.New(224/8, nil)
	if err != nil {
		return "", err
	}
	_, err = b2.Write(bytes)
	if err != nil {
		return "", err
	}
	hash := b2.Sum(nil)
	return hex.EncodeToString(hash), nil
}

const tagBase = 120

func (n *MultisigScript) UnmarshalCBOR(b []byte) error {
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
func (n *MultisigScript) MarshalCBOR() ([]byte, error) {
	switch {
	case n.Signature != nil:
		return cbor.Marshal(&n.Signature)
	case n.AllOf != nil:
		return cbor.Marshal(&n.AllOf)
	case n.AnyOf != nil:
		return cbor.Marshal(&n.AnyOf)
	case n.AtLeast != nil:
		return cbor.Marshal(&n.AtLeast)
	case n.Before != nil:
		return cbor.Marshal(&n.Before)
	case n.After != nil:
		return cbor.Marshal(&n.After)
	default:
		return nil, fmt.Errorf("invalid native script")
	}
}

type Signature struct {
	_       struct{} `cbor:",toarray"`
	KeyHash []byte
}

func (n *Signature) MarshalCBOR() ([]byte, error) {
	var bytes []byte
	bytes = append(bytes, 0x9f) // indefinite length array
	key, err := cbor.Marshal(&n.KeyHash)
	if err != nil {
		return nil, err
	}
	bytes = append(bytes, key...)
	bytes = append(bytes, 0xff) // end indefinite length array
	return cbor.Marshal(cbor.RawTag{Number: 1 + tagBase, Content: bytes})
}

type AllOf struct {
	_       struct{} `cbor:",toarray"`
	Scripts []MultisigScript
}

func (n *AllOf) MarshalCBOR() ([]byte, error) {
	var bytes []byte
	bytes = append(bytes, 0x9f) // indefinite length array for the struct
	bytes = append(bytes, 0x9f) // indefinite length array for the scripts
	for _, script := range n.Scripts {
		s, err := cbor.Marshal(&script)
		if err != nil {
			return nil, err
		}
		bytes = append(bytes, s...)
	}
	bytes = append(bytes, 0xff) // end indefinite length array for the scripts
	bytes = append(bytes, 0xff) // end indefinite length array for the structs
	return cbor.Marshal(cbor.RawTag{Number: 2 + tagBase, Content: bytes})
}

type AnyOf struct {
	_       struct{} `cbor:",toarray"`
	Scripts []MultisigScript
}

func (n *AnyOf) MarshalCBOR() ([]byte, error) {
	var bytes []byte
	bytes = append(bytes, 0x9f) // indefinite length array for the struct
	bytes = append(bytes, 0x9f) // indefinite length array for the scripts
	for _, script := range n.Scripts {
		s, err := cbor.Marshal(&script)
		if err != nil {
			return nil, err
		}
		bytes = append(bytes, s...)
	}
	bytes = append(bytes, 0xff) // end indefinite length array for the scripts
	bytes = append(bytes, 0xff) // end indefinite length array for the structs
	return cbor.Marshal(cbor.RawTag{Number: 3 + tagBase, Content: bytes})
}

type AtLeast struct {
	_        struct{} `cbor:",toarray"`
	Required int
	Scripts  []MultisigScript
}

func (n *AtLeast) MarshalCBOR() ([]byte, error) {
	var bytes []byte
	bytes = append(bytes, 0x9f) // indefinite length array for the struct
	req, err := cbor.Marshal(&n.Required)
	if err != nil {
		return nil, err
	}
	bytes = append(bytes, req...) // The Required property
	bytes = append(bytes, 0x9f)   // indefinite length array for the scripts
	for _, script := range n.Scripts {
		s, err := cbor.Marshal(&script)
		if err != nil {
			return nil, err
		}
		bytes = append(bytes, s...)
	}
	bytes = append(bytes, 0xff) // end indefinite length array for the scripts
	bytes = append(bytes, 0xff) // end indefinite length array for the structs
	return cbor.Marshal(cbor.RawTag{Number: 4 + tagBase, Content: bytes})
}

type Before struct {
	_    struct{} `cbor:",toarray"`
	Time time.Time
}

func (n *Before) MarshalCBOR() ([]byte, error) {
	var bytes []byte
	bytes = append(bytes, 0x9f) // indefinite length array for the struct
	time, err := cbor.Marshal(&n.Time)
	if err != nil {
		return nil, err
	}
	bytes = append(bytes, time...) // The Time property
	bytes = append(bytes, 0xff)    // end indefinite length array for the structs
	return cbor.Marshal(cbor.RawTag{Number: 5 + tagBase, Content: bytes})
}

type After struct {
	_    struct{} `cbor:",toarray"`
	Time time.Time
}

func (n *After) MarshalCBOR() ([]byte, error) {
	var bytes []byte
	bytes = append(bytes, 0x9f) // indefinite length array for the struct
	time, err := cbor.Marshal(&n.Time)
	if err != nil {
		return nil, err
	}
	bytes = append(bytes, time...) // The Time property
	bytes = append(bytes, 0xff)    // end indefinite length array for the structs
	return cbor.Marshal(cbor.RawTag{Number: 6 + tagBase, Content: bytes})
}
