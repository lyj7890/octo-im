# In-flight tag versions: candidate fix for #84

This branch builds on PR #73 (`d3b7575c8f0223f262f56a0b3f34616e6bb25943`). It preserves that PR's immutable published snapshots and addresses channel membership version publication separately.

## Changes

- Publish the changed membership under a fresh key and update the channel mapping in one manager operation. Keep the old key's membership unchanged for delayed cross-node lookups.
- Route both the HTTP subscriber path and channel-tag RPC updates through the new operation; return update failures.
- Retire old versions after a fixed grace period using the existing `Tag.Expire` setting. Reads of newer versions cannot extend an old version's lifetime.
- Retain cached older versions on receiving nodes and distribute consecutive events by their stamped TagKey, preserving A/B/A order.

The lower-level AddUsers/RemoveUsers/RenameTag operations keep their prior semantics. They must not be composed to publish channel membership versions.

## Evidence

The deployed candidate was built from runtime commit `601612bbd3741eefaa3e67c2eaaf162af139d0ab` plus the production changes in this branch. The patch applies unchanged to PR #73; tests on this checkout are recorded separately from deployment evidence.

Local candidate: six focused regression tests repeated 25 times under `-race`, affected package tests, and `go build ./...` passed.

Three-node deployment, each with 32 CPU / 64 GiB and SSD; 80,000 WebSocket users, 500-member groups, five child channels per group. Each channel sends 10 messages per minute in total. Heartbeat 60s, SENDACK timeout 30s. Baseline plus two independent SUB5 rounds (five complete unsubscribe/re-subscribe flows per second, five-minute measurement per round):

| Metric | Round 1 | Round 2 |
| --- | ---: | ---: |
| Minimum online | 80,000 | 80,000 |
| Formal SEND / on-time SENDACK | 48,000 / 48,000 | 48,000 / 48,000 |
| Completed subscription flows | 1,500 | 1,500 |
| Stable-member delivery gaps, including warmup and probes | 0 | 0 |
| Duplicate delivery / Tag lookup errors | 0 / 0 | 0 / 0 |
| Message P99 | ~6.03s | ~6.03s |
| Complete subscription-flow P99 | ~23.07s | ~22.02s |
| Presence RPC timeout log rows | 826 | 975 |

The two mixed rounds audited 73,962,319 expected stable-member deliveries, all received within the audit windows. The original image recorded 16,942 / 22,472 gaps under the same scenario. Neither image had SENDACK timeouts. P99 values are histogram estimates. These were sequential runs, not randomized performance trials; they do not isolate the cause of the higher mixed-load latency.

## Limits

- Bounded tag retention does not provide durable delivery recovery. A delayed task beyond the grace period, process restart or topology change can still fail to resolve its stamped version; failed dequeued distribution has no durable retry here.
- Retired versions consume memory proportional to update rate and retention time. Longer soak, reclamation and restart/failover tests remain necessary.
- Membership routing is calculated while holding the manager lock. The observed latency and RPC timeouts require profiling before treating this as a performance improvement.
- Lazy initial tag construction concurrent with membership updates needs further coverage.

Related: https://github.com/Mininglamp-OSS/octo-im/issues/84

## Checks on the PR #73 checkout

Affected package tests with `-race` passed. The six focused tests repeated 25 times under `-race` also passed.

A full `go test ./...` was attempted. It reported failures outside the modified files: `internal/server` could not bind TCP port 5172 (address already in use), and `internal/track/TestMessageString` expected path bits `1001000010000000` but observed `1001000001000000`. This branch does not modify those packages. A green full-suite result is not claimed.
