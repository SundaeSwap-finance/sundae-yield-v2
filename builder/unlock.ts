import { Lucid, Blockfrost, SpendingValidator, toHex, fromHex, Data, TxHash, UTxO, Script } from "https://deno.land/x/lucid@0.10.6/mod.ts";
import * as cbor from "https://deno.land/x/cbor@v1.4.1/index.js";
import { parse } from "https://deno.land/std@0.184.0/flags/mod.ts";


const flags = parse(Deno.args, {
  boolean: ["help", "dry", "list-utxos", "all"],
  string: ["skeyFile", "mnemonic", "mnemonicFile", "blockfrost", "env", "blueprint"],
  collect: ["unlock"]
});

const blockfrost = new Blockfrost(
    `https://cardano-${flags.env ?? "preview"}.blockfrost.io/api/v0`,
    flags.blockfrost ?? Deno.env.get("BLOCKFROST_API_KEY"),
  );
const lucid = await Lucid.new(blockfrost, flags.env == "mainnet" ? "Mainnet" : "Preview");

if (flags.skeyFile) {
  lucid.selectWalletFromPrivateKey(await Deno.readTextFile(flags.skeyFile))
} else if (flags.mnemonicFile) {
  lucid.selectWalletFromSeed(await Deno.readTextFile(flags.mnemonicFile));
} else if (flags.mnemonic) {
  lucid.selectWalletFromSeed(flags.mnemonic)
} else {
  console.log("must specify a wallet source: skeyFile, mnemonic, or mnemonicFile")
  Deno.exit(1);
}

const validator = await readValidator();
const scriptAddress = lucid.utils.validatorToAddress(validator);

const availableUtxos = await lucid.utxosAt(scriptAddress)
const utxos: UTxO[] = [];
for(const utxo of availableUtxos) {
  if(flags["list-utxos"]) {
    console.log(`${utxo.txHash}#${utxo.outputIndex}:`)
    for(const [asset, amount] of Object.entries(utxo.assets)) {
      console.log(`  ${asset}: ${amount}`)
    }
  }
  if (flags.unlock || flags.all) {
    for(const unlock of flags.unlock) {
      if (flags.all || unlock === `${utxo.txHash}#${utxo.outputIndex}`) {
        utxos.push(utxo);
      }
    }
  }
}
if(flags["list-utxos"]) {
  Deno.exit(0);
}
if(utxos.length == 0) {
  console.log("Nothing to unlock")
  Deno.exit(1)
}

const redeemer = Data.void();
const txUnlock = await unlock(utxos, { from: validator, redeemer });
if(!flags.dry) {
  console.log(`Waiting for tx ${txUnlock}...`);
  await lucid.awaitTx(txUnlock);
}
async function readValidator(): Promise<SpendingValidator> {
  const validator = JSON.parse(await Deno.readTextFile("../contracts/freezer/plutus.json")).validators[0];
  return {
    type: "PlutusV2",
    script: toHex(cbor.encode(fromHex(validator.compiledCode))),
  };
}
async function unlock(utxos: UTxO[], { from, redeemer }: { from: Script, redeemer: string}): Promise<TxHash> {
  const tx = await lucid
    .newTx()
    .collectFrom(utxos, redeemer)
    .addSigner(await lucid.wallet.address()) // this should be beneficiary address
    .attachSpendingValidator(from)
    .complete();
 
  const signedTx = await tx
    .sign()
    .complete();

  if (flags.dry) {
    console.log("DRY: not submitting tx")
    console.log(`Tx: ${signedTx.toString()}`);
    return signedTx.toHash();
  } else {
    return signedTx.submit();
  }
}