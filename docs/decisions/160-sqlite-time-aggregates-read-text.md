---
id: 160
title: "SQLite time aggregates read the text their destination can hold"
area: values
status: active
tags: [sqlite, time, MaxOf, MinOf, MaxRelated, MinRelated, scan]
issues: [317]
related: [25, 84, 134, 145, 147]
changed_by: []
code: [internal/meta/read.go, internal/scan/scan.go]
tests: [internal/meta/read_test.go, tests/any_column_test.go, tests/features/lifecycle/utc_test.go]
---

# 160. SQLite time aggregates read the text their destination can hold

**In short:** `MaxOf`, `MinOf`, `Max`, `Min`, `MaxRelated` and `MinRelated` read a SQLite time aggregate into `time.Time`, `*time.Time`, `sql.NullTime`, `sql.Null[time.Time]` and `ant.DeletedAt`, including the nullable wrappers a related aggregate uses.

## Decision

SQLite's driver decodes text as a time only when the result column has a declared `TIMESTAMP`, `DATETIME` or `DATE` type. An aggregate has no declared type, so it arrives as a string and `database/sql` refuses to put it in a time field (#317).

`meta.ReaderFor` now gives the time types it knows a scanner for this case. The scanner accepts `time.Time.String`, the driver's default when it binds a time, and the layouts the driver accepts for a declared time column, including `_time_format=sqlite`. Text with a monotonic-clock suffix loses that suffix, which is not part of the stored instant, and the result is held in UTC. A single value read through one of these scanners still gets the same UTC conversion as a value read directly.

A `time.Time` from another driver, NULL, and text that matches none of those layouts still go through `database/sql`'s original conversion. Other databases' behavior and its errors for values that are not times are unchanged. No SQL is rewritten and no DSN option is required.

Rejected:

- Casting aggregates in SQL would change every dialect's query generation for a SQLite scan detail, and the destination's Go type is already known while scanning.
- A driver-wide option that guesses whether every untyped text column is a time would also affect text that ant intends to keep as text. The conversion belongs at the typed destination.
