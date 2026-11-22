---
id: 145
title: "With adds a computed value without untracking a row"
area: tracking
status: active
tags: [With, computed, tracking, identity-map]
issues: [295]
related: [108]
changed_by: []
code: [query.go]
tests: [tests/features/loading/with_tracking_test.go]
---

# 145. With adds a computed value without untracking a row

**In short:** `With` adds computed values to a full stored row, so its results stay tracked; computed values themselves are neither compared nor written.

## Decision

`Column` and `Omit` return partial rows, and `Raw`, `Select` and `Project` return derived rows, so those results are not tracked. `With` is different: it reads every stored column and only adds expressions for fields tagged `computed`. A query that would otherwise be tracked therefore stays tracked when it uses `With`. `NoTracking` and the other tracking rules are unchanged.

Snapshots already leave computed fields out, as decision 108 describes. Editing one does not make an entity dirty, and `SaveChanges` never writes it. The stored fields still diff and save normally, including with an integer or UUID key.

The identity map can replace a freshly scanned row with the tracked instance it already holds. When that happens, only the computed values from the new query are copied onto the returned instance. Stored values and unsaved edits keep the existing identity-map behavior.
