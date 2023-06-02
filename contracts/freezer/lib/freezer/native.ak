use aiken/interval.{Finite, Interval, IntervalBound}
use aiken/list
use aiken/transaction.{ValidityRange}

pub type POSIXTime =
  Int

// TODO stdlib
pub fn count(self: List<a>, predicate: fn(a) -> Bool) -> Int {
  list.foldl(
    self,
    fn(item, total) {
      if predicate(item) {
        total + 1
      } else {
        total
      }
    },
    0,
  )
}

pub type NativeScript {
  Signature { keyHash: ByteArray }
  AllOf { scripts: List<NativeScript> }
  AnyOf { scripts: List<NativeScript> }
  AtLeast { required: Int, scripts: List<NativeScript> }
  Before { time: POSIXTime }
  After { time: POSIXTime }
}

pub fn satisfied(
  script: NativeScript,
  signatories: List<ByteArray>,
  validRange: ValidityRange,
) -> Bool {
  when script is {
    Signature { keyHash } -> list.has(signatories, keyHash)
    AllOf { scripts } ->
      list.all(scripts, fn(s) { satisfied(s, signatories, validRange) })
    AnyOf { scripts } ->
      list.any(scripts, fn(s) { satisfied(s, signatories, validRange) })
    AtLeast { required, scripts } ->
      required <= count(
        scripts,
        fn(s) { satisfied(s, signatories, validRange) },
      )
    Before { time } ->
      when validRange.upper_bound.bound_type is {
        Finite(hi) ->
          if validRange.upper_bound.is_inclusive {
            hi <= time
          } else {
            hi < time
          }
        _ -> False
      }
    After { time } ->
      when validRange.lower_bound.bound_type is {
        Finite(lo) ->
          if validRange.lower_bound.is_inclusive {
            time <= lo
          } else {
            time < lo
          }
        _ -> False
      }
  }
}

test satisfying() {
  let keyHash1 = "key1"
  let keyHash2 = "key2"
  let keyHash3 = "key3"
  let sig1 = Signature { keyHash: keyHash1 }
  let sig2 = Signature { keyHash: keyHash2 }
  let sig3 = Signature { keyHash: keyHash3 }
  let allOf = AllOf { scripts: [sig1, sig2] }
  let anyOf = AnyOf { scripts: [sig1, sig2] }
  let atLeast = AtLeast { required: 2, scripts: [sig1, sig2, sig3] }
  let before = Before { time: 10 }
  let after = After { time: 10 }
  let between = AllOf { scripts: [After { time: 10 }, Before { time: 15 }] }
  let vesting =
    AnyOf {
      scripts: [
        AllOf { scripts: [before, sig1] },
        // clawback
        AllOf { scripts: [after, sig2] },
      ],
    }
  // vested
  let validRange =
    fn(lo: Int, hi: Int) -> ValidityRange {
      Interval {
        lower_bound: IntervalBound {
          bound_type: Finite(lo),
          is_inclusive: True,
        },
        upper_bound: IntervalBound {
          bound_type: Finite(hi),
          is_inclusive: False,
        },
      }
    }
  // Helper method because ? binds more tightly than !
  let unsatisfied =
    fn(n: NativeScript, s: List<ByteArray>, v: ValidityRange) {
      !satisfied(n, s, v)
    }
  list.and(
    [
      satisfied(sig1, [keyHash1], validRange(0, 1))?,
      satisfied(sig2, [keyHash1, keyHash2], validRange(0, 1))?,
      satisfied(allOf, [keyHash1, keyHash2], validRange(0, 1))?,
      satisfied(anyOf, [keyHash2], validRange(0, 1))?,
      satisfied(atLeast, [keyHash2, keyHash3], validRange(0, 1))?,
      satisfied(before, [], validRange(0, 5))?,
      satisfied(after, [], validRange(15, 20))?,
      satisfied(after, [], validRange(10, 15))?,
      satisfied(between, [], validRange(12, 13))?,
      satisfied(vesting, [keyHash1], validRange(0, 5))?,
      satisfied(vesting, [keyHash2], validRange(15, 20))?,
      unsatisfied(sig1, [keyHash2], validRange(0, 1))?,
      unsatisfied(sig3, [keyHash1, keyHash2], validRange(0, 1))?,
      unsatisfied(allOf, [keyHash1, keyHash3], validRange(0, 1))?,
      unsatisfied(anyOf, [keyHash3], validRange(0, 1))?,
      unsatisfied(atLeast, [keyHash2], validRange(0, 1))?,
      unsatisfied(before, [], validRange(5, 15))?,
      unsatisfied(before, [], validRange(5, 10))?,
      unsatisfied(before, [], validRange(10, 10))?,
      unsatisfied(after, [], validRange(5, 15))?,
      unsatisfied(between, [], validRange(0, 5))?,
      unsatisfied(between, [], validRange(0, 13))?,
      unsatisfied(between, [], validRange(0, 20))?,
      unsatisfied(between, [], validRange(13, 20))?,
      unsatisfied(between, [], validRange(13, 15))?,
      unsatisfied(between, [], validRange(15, 20))?,
      unsatisfied(vesting, [keyHash2], validRange(0, 5))?,
      unsatisfied(vesting, [keyHash1], validRange(15, 20))?,
      unsatisfied(vesting, [keyHash3], validRange(10, 10))?,
      unsatisfied(vesting, [keyHash3], validRange(0, 5))?,
      unsatisfied(vesting, [keyHash3], validRange(15, 20))?,
    ],
  )
}
