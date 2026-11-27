---
id: 161
title: "A month of a partitioned table is read from its bounds, not its name"
area: migrations
status: active
tags: [partitions, MonthlyPartitions, DropBefore, Ensure, List, postgres, bounds]
issues: [196]
related: [72, 130]
changed_by: []
code: [dialect/postgres/partitions.go]
tests: [tests/features/migrations/modeling_test.go]
---

# 161. A month of a partitioned table is read from its bounds, not its name

**In short:** `postgres.MonthlyPartitions` finds a month through the partition's bounds, so one made under another name — a DBA's `payment_p2022_01` — is listed, kept by `Ensure`, and dropped by `DropBefore`; a partition over another span is left alone, and one whose name collides with a month it is not is reported rather than skipped.

## Decision

`List` matched `<table>_YYYY_MM` by name, so a table whose months were made before ant owned it, as Pagila's `payment_p2022_01`, listed nothing. `DropBefore` never dropped those months and retention quietly stopped, and `Ensure` failed on the overlap it could not see (#196).

A partition is now a month when its bounds are one: midnight UTC on the first of a month up to midnight UTC on the first of the next. What makes it a month is the span, which is what the table partitions on, not the name, which anyone may have chosen. `List` reads `pg_get_expr(relpartbound)` and takes the range it prints; `Ensure` skips a month the list already holds; `DropBefore` drops by month, as before. A partition over another span, or one whose bound is `MINVALUE` or `MAXVALUE`, is not a month and is left alone.

The bounds are text, and how PostgreSQL writes a timestamp in it follows the key's type and `DateStyle`. `List` pins `DateStyle` to ISO in a read-only transaction of its own, so the parse does not depend on the server or the caller. A `timestamptz` column prints a zone, as `2026-04-01 00:00:00+00`; a `timestamp` column prints none, as `2026-04-01 00:00:00`; a `date` column prints a plain date, as `2026-04-01`. All three are read, the zone-less forms as UTC, since ant partitions on UTC. Reading only the zoned form would drop a table on the other two types from `List`, recreating the retention failure this fixes for those tables.

`Ensure` names a month `<table>_YYYY_MM`. If a partition already has that name but spans something else, it cannot be replaced: `CREATE TABLE IF NOT EXISTS` skips it and reports success. `Ensure` returns an error naming the partition rather than a month it did not make, so a schedule does not report a month covered that is not.

Rejected:

- A configurable name format would still miss a table whose months were named ad hoc, and the bounds say what every month is without being told.
- Matching the name as a second signal would keep a `table_2026_04` that spans something other than April, which `Ensure` and `DropBefore` would then treat as a month it is not.
