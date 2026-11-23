---
id: 159
title: "A SQLite DSN names its lock behavior"
area: dialects
status: active
tags: [sqlite, dsn, CheckDSN, busy_timeout, _txlock, SQLITE_BUSY]
issues: [306]
related: [74, 118]
changed_by: []
code: [dialect/sqlite/sqlite.go]
tests: [dialect/sqlite/sqlite_test.go]
---

# 159. A SQLite DSN names its lock behavior

**In short:** `Open` requires an explicit `busy_timeout` and `_txlock` choice as well as `foreign_keys`; zero, deferred, and exclusive remain valid opt-outs and alternatives.

## Decision

A DSN with only `foreign_keys` passed `Open`, but concurrent writes could wait no time for a lock, and a read-then-write transaction could not upgrade its lock without failing with `SQLITE_BUSY` (#306). `CheckDSN` now also requires a valued `busy_timeout`, written as `_pragma=busy_timeout(...)` or with the driver's `_busy_timeout` / `_timeout` shorthand, and a non-empty `_txlock`. The error recommends `busy_timeout(5000)` and `_txlock=immediate`, the values already used by the quick start.

As with `foreign_keys(0)`, the check requires an explicit choice rather than one blessed value. `busy_timeout(0)` keeps the driver's no-wait default on purpose, `_txlock=deferred` keeps deferred transactions, and `_txlock=exclusive` is another valid lock mode. The pragma parser accepts the same parenthesized, equals, whitespace, and case variants as the existing foreign-key check, but a query-only `busy_timeout` and an empty `_txlock` make no choice. The `_timeout` alias takes precedence over `_busy_timeout`, as it does in the driver, so an empty alias cannot be bypassed by also naming the primary key. `Wrap` still checks nothing because the application owns that pool.

A warning without a refusal was rejected: `CheckDSN` has no logging channel, and a server that misses the warning still fails only under load. Reading the current connection settings was rejected for the same reason as in decision 118: it would remove the explicit opt-out and would not cover the other connections a pool opens later.

Refusing DSNs that opened before is an intentional breaking change. The matching ant-docs page is `reference/databases.mdx`, whose SQLite DSN examples need the same three settings when this ships.
