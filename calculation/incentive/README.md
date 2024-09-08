# ADA Incentive Yield Farming Spec

Sundae Labs was directed to distributed 15% of monthly yield farming rewards to SUNDAE token holders.

This was passed in the following governance proposal:

- https://governance.sundaeswap.finance/#/proposal#ec282b8ee99483baefd925fa52d3429eaa9c6305c1ad86c3397bca70a0de8823

This system is much simpler than the yield farming program:

- Once per month, we fetch all positions that existed for the duration of the month
- For each position, we calculate its weight:
  - The quantity of SUNDAE
  - Plus the quantity of SUNDAE represented by LP tokens (using the value of the pool at the end of the month)
  - Times the number of seconds the position existed within the month
  - Divided by the number of seconds in the month
- We sum this weight by owner ID
- We then multiply the emitted asset amount by the weight of each owner
- And divide by the total weight
- We distributed any remaining lovelace round-robin among owners to avoid under-emission
  - The round robin distribution is sorted by smallest emissions first
- We save these as earnings in the database for the user to claim
