use aiken/transaction.{ScriptContext}
use sundae/multisig.{MultisigScript}

type StakeDatum {
  owner: MultisigScript,
  data: Data,
}

// NOTE: Unused, but this is the type we intend to store in Data above
type Delegation {
  pool_ident: ByteArray,
  weight: Int,
}

validator {
  fn stake(datum: StakeDatum, _r: Void, ctx: ScriptContext) -> Bool {
    multisig.satisfied(
      datum.owner,
      ctx.transaction.extra_signatories,
      ctx.transaction.validity_range,
    )
  }
}
// TODO(pi): Write some tests with a realistic ScriptContext