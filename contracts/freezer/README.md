# freezer

A very simple smart contract that allows tokens to be locked with some arbitrary data, and unlocked at any time.

## Datum

`owner` supports all the logic of Cardano Native Scripts, such as multisig and time-locking.
`data` allows arbitrary data to be attached to the datum, for use in off-chain contexts, or composing with future smart contracts.

## Logic

lib/freezer/native.ak provides a type and function which emulate the cardano native script capability. It allows you to compose criteria, to produce time-locking or multi-sig scenarios.  There's some talk about moving this to the aiken stdlib.

Given that, the script logic is incredibly simple: succeed if the `owner` criteria is satisfied, otherwise fail.