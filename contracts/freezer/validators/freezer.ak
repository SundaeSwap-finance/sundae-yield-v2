use aiken/transaction.{ScriptContext}
use freezer/native

type StakeDatum {
  owner: native.NativeScript,
  data: Data,
}

// NOTE: Unused, but this is the type we intend to store in Data above
type Delegation {
  pool_ident: ByteArray,
  weight: Int,
}

validator {
  fn stake(datum: StakeDatum, _r: Void, ctx: ScriptContext) -> Bool {
    native.satisfied(
      datum.owner,
      ctx.transaction.extra_signatories,
      ctx.transaction.validity_range,
    )
  }
}
// TODO(pi): Write some tests with a realistic ScriptContext
