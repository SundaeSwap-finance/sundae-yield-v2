import { Lucid, Blockfrost, SpendingValidator, toHex, fromHex, Data, Constr, TxHash, Script, Assets } from "https://deno.land/x/lucid@0.10.6/mod.ts";
import * as cbor from "https://deno.land/x/cbor@v1.4.1/index.js";
import { parse } from "https://deno.land/std@0.184.0/flags/mod.ts";
import { Buffer } from "https://deno.land/std@0.184.0/io/buffer.ts"

const flags = parse(Deno.args, {
  boolean: ["help", "dry"],
  string: ["skeyFile", "mnemonic", "mnemonicFile", "blockfrost", "env", "blueprint"],
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

async function readValidator(): Promise<SpendingValidator> {
  const validator = JSON.parse(await Deno.readTextFile(flags.blueprint ?? "../contracts/freezer/plutus.json")).validators[0];
  return {
    type: "PlutusV2",
    script: toHex(cbor.encode(fromHex(validator.compiledCode))),
  };
}

const validator = await readValidator();

const hash = await publish(validator)
console.log(`Transaction Hash: ${hash}`)
if(!flags.dry) {
  console.log(`Waiting for tx ${hash}.`)
  await lucid.awaitTx(hash);
  console.log("Tx Seen.")
}

async function publish(script: Script): Promise<TxHash> {
  const tx = await lucid
    .newTx()
    .payToAddressWithData(await lucid.wallet.address(), {scriptRef: script}, { "lovelace": 2000000n })
    .complete();
 
  const signedTx = await tx.sign().complete();
 
  if (flags.dry) {
    console.log("DRY: transaction not submitted")
    console.log(`Tx: ${signedTx.toString()}`)
    return signedTx.toHash();
  } else {
    return signedTx.submit();
  }
}