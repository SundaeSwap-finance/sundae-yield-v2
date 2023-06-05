use aiken/interval.{Finite, Interval, IntervalBound}
use aiken/list
use aiken/time.{PosixTime}
use aiken/transaction.{ValidityRange}

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
  Signature { key_hash: ByteArray }
  AllOf { scripts: List<NativeScript> }
  AnyOf { scripts: List<NativeScript> }
  AtLeast { required: Int, scripts: List<NativeScript> }
  Before { time: PosixTime }
  After { time: PosixTime }
}

pub fn satisfied(
  script: NativeScript,
  signatories: List<ByteArray>,
  validRange: ValidityRange,
) -> Bool {
  when script is {
    Signature { key_hash } -> list.has(signatories, key_hash)
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
  let key_hash1 = "key1"
  let key_hash2 = "key2"
  let key_hash3 = "key3"
  let sig1 = Signature { key_hash: key_hash1 }
  let sig2 = Signature { key_hash: key_hash2 }
  let sig3 = Signature { key_hash: key_hash3 }
  let all_of = AllOf { scripts: [sig1, sig2] }
  let any_of = AnyOf { scripts: [sig1, sig2] }
  let at_least = AtLeast { required: 2, scripts: [sig1, sig2, sig3] }
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
      satisfied(sig1, [key_hash1], validRange(0, 1))?,
      satisfied(sig2, [key_hash1, key_hash2], validRange(0, 1))?,
      satisfied(all_of, [key_hash1, key_hash2], validRange(0, 1))?,
      satisfied(any_of, [key_hash2], validRange(0, 1))?,
      satisfied(at_least, [key_hash2, key_hash3], validRange(0, 1))?,
      satisfied(before, [], validRange(0, 5))?,
      satisfied(after, [], validRange(15, 20))?,
      satisfied(after, [], validRange(10, 15))?,
      satisfied(between, [], validRange(12, 13))?,
      satisfied(vesting, [key_hash1], validRange(0, 5))?,
      satisfied(vesting, [key_hash2], validRange(15, 20))?,
      unsatisfied(sig1, [key_hash2], validRange(0, 1))?,
      unsatisfied(sig3, [key_hash1, key_hash2], validRange(0, 1))?,
      unsatisfied(all_of, [key_hash1, key_hash3], validRange(0, 1))?,
      unsatisfied(any_of, [key_hash3], validRange(0, 1))?,
      unsatisfied(at_least, [key_hash2], validRange(0, 1))?,
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
      unsatisfied(vesting, [key_hash2], validRange(0, 5))?,
      unsatisfied(vesting, [key_hash1], validRange(15, 20))?,
      unsatisfied(vesting, [key_hash3], validRange(10, 10))?,
      unsatisfied(vesting, [key_hash3], validRange(0, 5))?,
      unsatisfied(vesting, [key_hash3], validRange(15, 20))?,
    ],
  )
}
