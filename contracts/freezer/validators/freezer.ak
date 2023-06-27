use aiken/dict
use aiken/hash
use aiken/interval
use aiken/transaction.{Transaction}
use aiken/transaction/credential
use aiken/transaction/value.{to_minted_value}
use sundae/multisig.{MultisigScript}

type StakeDatum {
  owner: MultisigScript,
  data: Data,
}

// NOTE: Unused, but this is the type we intend to store in Data above
type Delegation {
  program: String,
  pool_ident: ByteArray,
  weight: Int,
}

validator {
  fn can_unlock(
    datum: StakeDatum,
    _r: Void,
    ctx: transaction.ScriptContext,
  ) -> Bool {
    multisig.satisfied(
      datum.owner,
      ctx.transaction.extra_signatories,
      ctx.transaction.validity_range,
    )
  }
}

test test_can_unlock() {
  // TODO: compiler bug https://github.com/aiken-lang/aiken/issues/591
  // let tx =
  //   Transaction {
  //     inputs: [
  //       transaction.Input(
  //         transaction.OutputReference(transaction.TransactionId("B"), 1),
  //         transaction.Output(
  //           credential.Address(credential.ScriptCredential("C"), None),
  //           value.zero(),
  //           transaction.NoDatum,
  //           None,
  //         ),
  //       ),
  //     ],
  //     reference_inputs: [],
  //     outputs: [],
  //     fee: value.zero(),
  //     mint: to_minted_value(value.zero()),
  //     certificates: [],
  //     withdrawals: dict.new(),
  //     validity_range: interval.everything(),
  //     extra_signatories: ["X"],
  //     redeemers: dict.new(),
  //     datums: dict.new(),
  //     id: transaction.TransactionId("A"),
  //   }

  // let datum = StakeDatum { owner: multisig.Signature("X"), data: Void }

  // let out_ref = transaction.OutputReference(transaction.TransactionId("B"), 1)
  // let sc = transaction.ScriptContext(tx, transaction.Spend(out_ref))
  // can_unlock(datum, Void, sc)
  True
}
