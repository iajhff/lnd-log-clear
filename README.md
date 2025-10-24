# Clear ALL forwarding logs
go run clear_db.go ~/.lnd/data/graph/mainnet/channel.db circuit-fwd-log

# Clear forwarding logs older than 1 week
go run clear_db.go ~/.lnd/data/graph/mainnet/channel.db --older-than=1w circuit-fwd-log

# Clear logs older than 2 weeks
go run clear_db.go ~/.lnd/data/graph/mainnet/channel.db --older-than=2w circuit-fwd-log

# Clear logs older than 1 month
go run clear_db.go ~/.lnd/data/graph/mainnet/channel.db --older-than=1m circuit-fwd-log

# Clear logs older than 3 months
go run clear_db.go ~/.lnd/data/graph/mainnet/channel.db --older-than=3m circuit-fwd-log

# Clear multiple buckets (all data)
go run clear_db.go ~/.lnd/data/graph/mainnet/channel.db \
    circuit-fwd-log \
    closed-chan-bucket \
    historical-chan-bucket

```bash
# Cron job (daily)
0 3 * * * go run clear_db.go ~/.lnd/data/graph/mainnet/channel.db --older-than=1d circuit-fwd-log closed-chan-bucket historical-chan-bucket
```

# What's In Your LND Database If Compromised

Simple breakdown of privacy risks.

---

## The Database File

**Location:** `~/.lnd/data/graph/mainnet/channel.db`

**Format:** BoltDB (key-value database)

**If someone gets this file, here's what they can extract:**

---

## 1. Forwarding Log (`circuit-fwd-log` bucket)

### What's Stored

Each forwarding event contains:
- **Timestamp** - Exact time you routed the payment
- **Incoming channel ID** - Where payment came from
- **Outgoing channel ID** - Where you sent it
- **Amount in** - How much received (millisatoshis)
- **Amount out** - How much sent (millisatoshis)
- **Fee** - Amount in - Amount out
- **HTLC IDs** - Specific payment identifiers

### Example Entry

```
Timestamp: 2025-10-24 12:34:56.789
Incoming Chan: 123456789012345
Outgoing Chan: 234567890123456  
Amount In: 1,000,000 msat
Amount Out: 999,000 msat
Fee: 1,000 msat
```

### What Attacker Learns

**From ONE entry:**
- You routed a payment at that exact time
- Channels 123456 and 234567 are linked
- Payment was ~1000 sats
- You earned 1 sat in fees

**From ALL entries:**
- Complete routing history timeline
- Which channels route together (reveals routing topology)
- Total fee income
- Busiest times/days
- Traffic patterns

### Reconstruction Example

```
Your database shows:
  [12:34:56] Chan A → Chan B (1M msat)
  [12:34:56] Chan C → Chan D (1M msat)
  
Inference: These are likely the same payment (same time, same amount)
Possible path: Chan A → You → Chan B → Someone → Chan C → Someone → Chan D
```

---

## 2. Closed Channels (`closed-chan-bucket`)

### What's Stored

For each closed channel:
- **Channel point** - Bitcoin TX that opened it
- **Remote peer pubkey** - Who you had the channel with
- **Capacity** - Total channel size
- **Settled balance** - Your final balance when closed
- **Close height** - What block it closed
- **Close type** - Cooperative, force close, or breach
- **Closing TX ID** - Bitcoin transaction that closed it

### Example Entry

```
Channel Point: abc123def456...:0
Remote Peer: 02aabbccdd1122334455...
Capacity: 5,000,000 sats
Your Balance: 3,200,000 sats
Peer Balance: 1,800,000 sats
Closed At: Block 800,000
Close Type: Cooperative
```

### What Attacker Learns

- You had a channel with peer 02aabbccdd...
- You ended up with 3.2M sats (started with 2M if you funded it)
- You made 1.2M sats profit on this channel
- Channel was open for ~5000 blocks (~35 days)
- No disputes (cooperative close)

---

## 3. Historical Channels (`historical-chan-bucket`)

Same as closed channels but with more detailed state information. Redundant for most analysis.

---

## 4. Open Channels (`open-chan-bucket`)

**Contains:** Current balances, pending HTLCs, commitment state

**Privacy risk:** Shows your CURRENT money, but you need this for operation.

**Note:** Can't delete without losing access to your funds.

---

## Correlation Attack Example

### Scenario: Attacker gets YOUR database

**Your forwarding log shows:**
```
[Oct 24, 12:34:56]
  Channel 111111 → Channel 222222 (1,000,000 msat, fee 1,000)

[Oct 24, 12:34:57] (1 second later)
  Channel 333333 → Channel 444444 (999,000 msat, fee 1,000)
```

**Analysis:**
- Two forwards, 1 second apart
- Amounts match (1M → 999k after fees)
- These are likely THE SAME payment going through you twice (multi-hop)

**Conclusion:**
- Payment path includes: Channel 111111 → You → 222222 → Someone → 333333 → You → 444444
- You're routing for the same payment multiple times

---

### Scenario: Attacker gets MULTIPLE node databases

**Your database:**
```
[12:34:56] Alice_chan → Bob_chan (1M msat → 999k msat)
```

**Bob's database:**
```
[12:34:56] You_chan → Carol_chan (999k msat → 998k msat)
```

**Carol's database:**
```
[12:34:56] Bob_chan → Dave_chan (998k msat → 997k msat)
```

**Reconstruction:**
```
Complete path: Alice → You → Bob → Carol → Dave

Original amount: 1,000,000 msat
Final amount: 997,000 msat
Total fees: 3,000 msat (you earned 1k, Bob 1k, Carol 1k)

Sender: Alice
Receiver: Dave
```

**This reveals WHO sent and WHO received, even though they're multiple hops apart.**

---

## Real Attack Output

```bash
$ go run forensic_analysis.go ~/.lnd/data/graph/mainnet/channel.db

⚠️  PRIVACY LEAK DETECTED!
Found 598 forwarding events in database

Timeline:
  First: 2025-09-24 06:44:01
  Last: 2025-10-24 02:28:10
  Period: 30 days

Channel Pairs:
  123456 → 789012: 157 times (7.7M msat fees)
  234567 → 789012: 155 times (7.2M msat fees)
  123456 → 345678: 145 times (7.5M msat fees)

Traffic:
  Busiest hour: 19:00 (45 forwards)
  Busiest day: Oct 21 (40 forwards)

Financials:
  Total volume: 29,241,421 sats routed
  Total fees: 29,241 sats earned
  Fee rate: 0.1%
```

---

## What This Means

From your database alone, an attacker knows:

1. **Every payment you routed** - Complete history with timestamps
2. **Channel topology** - Which channels connect in routing paths
3. **Activity schedule** - When you're busiest
4. **Income** - Exactly how much you earned
5. **Routing patterns** - Which routes are most popular

Combined with public Lightning graph data:
- Can identify your peers (channel IDs → pubkeys)
- Can determine likely senders and receivers
- Can reconstruct multi-hop payment paths

---

## The Fix

(https://github.com/iajhff/lnd/commit/cc39030134a5589721e4381beae5512d89fc5a30)
```bash
lnd --no-forwarding-history 
```

**After enabling flag:**
```bash
$ go run forensic_analysis.go channel.db

✓ No forwarding history found in database!
```

**Result:** Database contains ZERO routing history. Attacker learns nothing about your forwarding activity.

---

## Summary

**Database contains:** Complete forwarding history forever

**Attacker can extract:** 
- Routing timeline
- Channel correlations  
- Fee income
- Traffic patterns
- Payment path hints

**Solution:** `--no-forwarding-history` flag prevents all of this

That's the risk. That's the fix.

