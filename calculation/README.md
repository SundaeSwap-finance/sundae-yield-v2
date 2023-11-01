# Yield Farming v2 Calculation Spec

Source: https://governance.sundaeswap.finance/#/proposal#a05c237661ca815c13b0526932ed40dd643fd7071cd6cd973df02532d7993a4a

Note: A few small details were left unspecified in the spec, and are noted below with **ADDENDUM**; These differences can only ever impact the outcome by one millionth of a sundae per pool or per person, so is considered not critical enough to seek another governance vote. 

This also includes changes described in the following proposals:
 - https://governance.sundaeswap.finance/#/proposal#346ed07d68025757fadc420fcca56c20a5cb8a8bc44e21bb3156372172f43923
 - https://governance.sundaeswap.finance/#/proposal#b329aec41e3e612d9b088d0af580627b14e2b40ea238be72f5aa78c2ec6bdbe7
 - https://governance.sundaeswap.finance/#/proposal#fc3294e71a2141f2147b32a72299c0b0bb061d44409d498bc8063141d7b0c0e9
 - https://governance.sundaeswap.finance/#/proposal#3073216b94547c3737246d5a6c2190e475cd71a00fafa8f7b753704898a942ec
 - https://governance.sundaeswap.finance/#/proposal#aad6e69a965debc3fa90d3bbe72c5d2aeceabcd14534b6f53265eaf4669b8cc7

Algorithm:
- Any unclaimed Yield Farming v1 rewards will be assigned an expiration date 90 days after the launch of the new yield farming program.
- After the development of the features summarized above, the yield farming program will be launched with a separate protocol vote to decide between the following three initial daily emission rates:
  - 118,430 SUNDAE per day (equivalent to 20% of the treasury in the next 4 years);
  - 296,077 SUNDAE per day (equivalent to 50% of the treasury in the next 4 years);
  - 444,115 SUNDAE per day (equivalent to 75% of the treasury in the next 4 years).
- Every 90 days, a governance proposal will automatically be created with the following options:
  - Keep daily emissions the same;
  - Raise daily emissions by 5%;
  - Lower daily emissions by 5%;
  - Lower daily emissions by 10%.
- The DAO may, at any time, pass a proposal to update the daily emission to an arbitrary value if that proposal has a quorum of (e.g., has votes by) at least 20% of the circulating supply of SUNDAE tokens.
  - Circulating supply will be defined as the total supply, minus the Sundae treasury holdings, minus the Sundae team multisig wallet.
- A new, very simple and open source contract (henceforth the Locking Contract) will be written that allows users to lock arbitrary assets and reclaim them at any time.
  - This will serve the dual role of allowing users to lock Liquidity Tokens, to earn yield farming rewards, and locking SUNDAE, to indicate their preference for which pools should earn rewards.
  - Note that the term “lock” is a misnomer here, and these assets can be reclaimed by their owner at any time.
  - The owner will be encoded in the datum; unlike the Yield Farming v1 script, this will use the same the “native script” format, to support yield farming from multisig schemes.
  - The datum will also support arbitrary extra data in the datum, for future flexibility; for example, we will use this to encode a list of pools and weights for each pool when staking SUNDAE.
- From the web app, users will be able to lock either LP tokens or SUNDAE (or both) into the Locking Contract.
  - If a user has an existing position, the web app will construct an on-chain transaction which adds to that position, though users may have multiple positions if building transactions manually or as a result of eventual consistency.
  - When locking SUNDAE, the user will additionally specify a list of preferences for pools they wish to receive SUNDAE token emissions; this will be in the form of a map from the unique identifier for a pool to a weight value, with the empty string signifying a portion of the SUNDAE held in abstention.
  - To avoid forcing users to choose between yield farming and governance, SUNDAE tokens and SUNDAE/ADA LP tokens held in this contract will be counted towards any governance votes.
- Each day, 2 hours after Midnight UTC, SundaeSwap Labs will, by means of an automated process, compute the daily emissions to each pool and earned rewards, using the ledger-state snapshot “as of” (up to, but not exceeding) midnight UTC.
  - This 2-hour delay is to ensure roll-backs don’t change the result.
  - To calculate the daily emissions, SundaeSwap Labs will first take inventory of SUNDAE held at the Locking Contract.
  - Each UTXO of locked SUNDAE may encode a weighting for a set of pools, as described above; the absence of such a list will exclude all SUNDAE at that UTXO from consideration.
  - SundaeSwap Labs will then divide the SUNDAE at the UTXO among the selected options in accordance to the weight, rounding down and distributing millionths of a SUNDAE among the options in order until the total SUNDAE allocated equals the SUNDAE held at the UTXO.
  - Any pool that has less than 1% of the pools LP tokens held at the Locking Contract will be considered an abstention and will not be eligible for rewards.
  - Any pool, asset, or pair that is explicitly disqualified will also be disqualified, such as any ADA/SUNDAE pool.
  - We will sum up the allocated SUNDAE across all UTXOs held at the Locking Contract.
  - We will add this SUNDAE to the raw SUNDAE delegations from the previous 2 days, to deter wild swings in delegation.
  - Any pools with fixed emissions will be assigned those emissions; for example, the ADA/SUNDAE pool (08) has a fixed emission of 133234.5 SUNDAE per day.
  - Among the remaining qualified pools, the top N pools (currently 10), or the top pools that collectively receive P percent (currently 80%) of the total weight (whichever is fewer) will be eligible for yield farming rewards that day.
    - **ADDENDUM**: Perfect ties will be broken by those who have issued the fewest LP tokens, and then in favor of the lesser poolIdentifer
  - Note that this criteria can be updated by a governance vote.
  - We then divide the remaining daily emissions among these pools in proportion to their weight, rounding down and distributing millionths of a SUNDAE among them until the daily emission is accounted for.
    - **ADDENDUM**: Perfect ties are resolved in favor of the lesser poolIdentifier
  - Any pool which exceeds the Emissions cap (currently 62176.1 SUNDAE) will be set to the emissions cap, and the remaining emissions returned to the treasury.
- For each pool, SundaeSwap labs will then calculate the allocation of rewards in proportion to the LP tokens held at the Locking Contract at any point during the 24 hours between each snapshot.
  - We will sum up the LP tokens held by each distinct owner encoded in the Locking Contract; thus, if someone has multiple UTXOs at the Locking Contract, they will receive a single allocation in proportion to the sum of their LP tokens.
  - We will multiply the quantity of LP tokens by the lifespan of the UTXO, in seconds, intersected with the 24 hour window.
    - For example, if someone has 100 LP tokens locked for the full day, their weight will be 8640000; if someone has 100 LP tokens locked for half the day, their weight will be 4320000.
    - This is to prevent gamesmanship by locking tokens in the last minutes before the snapshot.
  - The emissions for each owner will be rounded down, and millionths of a SUNDAE distributed round-robin until the total user emissions match the pool emissions.
    - **ADDENDUM**: The sort order for this round-robin will be by hash of the owner in the datum; this will only ever make one sprinkle worth of difference.
- Users will be able to claim these emitted tokens at any time, with no 30-day locking requirement.
  - A service administered by SundaeSwap Labs will allow users to claim these Sundae rewards by providing a signature (or set of signatures, in the case of a multisig owner).
  - They can choose to pay the accumulated SUNDAE rewards directly to their wallet, or directly into the Locking UTXO, effectively “re-staking” that SUNDAE immediately.
- Rewards must be claimed within 6 months of being earned.
- A governance vote may be held to explicitly exclude any token or pool from consideration.
  - During such a vote, rewards will accrue, but be unclaimable until the vote has concluded.
  - If the vote fails, the rewards will be claimable as normal.
  - If the vote succeeds, the emitted SUNDAE tokens will be returned to the treasury for future emissions.
- Additionally, any project may choose to emit their own project token, split similarly across one or multiple pools.
  - SundaeSwap Labs will administer this service, and enter into an agreement with each project.
  - The project is responsible for furnishing the tokens to be distributed.
  - SundaeSwap Labs will allow LP tokens for these pools to be locked in a similar way, and a daily emission of tokens to be distributed among those liquidity providers in a similar way.
  - A user may claim both SUNDAE and native token rewards in the same transaction, to save on network fees.
  - SundaeSwap Labs will charge a small transaction fee to each claim involving a token other than SUNDAE, to cover administrative costs.
  - Explicitly, SundaeSwap Labs will not charge a fee for claims that only distribute SUNDAE tokens.