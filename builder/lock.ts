import { Lucid, Blockfrost, SpendingValidator, toHex, fromHex, Data, Constr, TxHash, Script, Assets } from "https://deno.land/x/lucid@0.10.6/mod.ts";
import * as cbor from "https://deno.land/x/cbor@v1.4.1/index.js";
import { parse } from "https://deno.land/std@0.184.0/flags/mod.ts";

const flags = parse(Deno.args, {
  boolean: ["help", "dry", "list-utxos"],
  string: ["skeyFile", "mnemonic", "mnemonicFile", "blockfrost", "env", "blueprint"],
  collect: ["lock"]
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

const utxos = await lucid.wallet.getUtxos();
if(flags["list-utxos"]) {
  for(const utxo of utxos) {
    console.log(`${utxo.txHash}#${utxo.outputIndex}:`)
    for(const [asset, amount] of Object.entries(utxo.assets)) {
      console.log(`  ${asset}: ${amount}`)
    }
  }
  Deno.exit(0);
}

async function readValidator(): Promise<SpendingValidator> {
  const validator = JSON.parse(await Deno.readTextFile(flags.blueprint ?? "../contracts/freezer/plutus.json")).validators[0];
  return {
    type: "PlutusV2",
    script: toHex(cbor.encode(fromHex(validator.compiledCode))),
  };
}

const validator = await readValidator();
const datum = buildDatum(await lucid.wallet.address())


const assets: Assets = {}
for(const lock_ of flags.lock) {
  const lock = lock_ as string;
  const parts = lock.split(':')
  assets[parts[0]] = BigInt(parts[1])
}

const hash = await lock(assets, { into: validator, datum: Data.to(datum) })
console.log(`Transaction Hash: ${hash}`)
if(!flags.dry) {
  await lucid.awaitTx(hash);
  console.log("Tx Seen.")
}

function buildDatum<T>(address: string, arbitrary?: Data): Data {
  const addressDetails = lucid.utils.getAddressDetails(address);

  // Can't get Lucid's types to work, so just Constr manually
  const owner = new Constr(0, [
    addressDetails.paymentCredential!.hash
  ])
  return new Constr(0, [
    owner,
    arbitrary ?? new Constr(0, []),
  ])
}

async function lock(assets: Assets, { into, datum }: { into: Script, datum: string }): Promise<TxHash> {
  const contractAddress = lucid.utils.validatorToAddress(into);
 
  const tx = await lucid
    .newTx()
    .payToContract(contractAddress, { inline: datum }, assets)
    .complete();
 
  const signedTx = await tx.sign().complete();
 
  if (flags.dry) {
    console.log("DRY: transaction not submitted")
    return signedTx.toHash();
  } else {
    return signedTx.submit();
  }
}