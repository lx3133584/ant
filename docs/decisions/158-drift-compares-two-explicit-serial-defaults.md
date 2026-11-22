---
id: 158
title: "Drift compares two explicit serial defaults"
area: migrations
status: active
tags: [drift, identity, serial, nextval, sequence, postgres]
issues: [194]
related: [57, 114]
changed_by: []
code: [dialect/postgres/inspect.go, migrate/operate.go]
tests: [tests/features/migrations/sequence_drift_test.go]
---

# 158. Drift compares two explicit serial defaults

**In short:** Drift reports two explicit serial defaults on different sequences without changing the ordinary migration plan; identity/serial adoption and a missing default remain compatible.

## Decision

Inspection treated every `nextval` default as identity and discarded its expression, so drift could not see that a live column and the models used different sequences (#194). A true PostgreSQL identity still drops its default expression during inspection. A serial, where `attidentity` is empty, now keeps the expression while also being marked as database-generated.

After the usual expression reconciliation, the drift engine compares explicit serial defaults on matching columns and reports different sequences as differences. This is a drift-only diagnostic, not a change to `schema.Diff`: decision 114 still suppresses identity default changes in ordinary migration plans, and `drift --sql` does not emit `SET DEFAULT` for this diagnostic. The same sequence produces no drift, including when the model omits PostgreSQL's `::regclass` cast, and a default missing on either side keeps the existing adoption behavior. Sequence diagnostics count toward the total in `driftNote`.

Changing shared diff to compare every identity default was rejected because it would also generate migration DDL, exceeding this bug's scope and risking `SET DEFAULT` on a true identity. Discarding every sequence name was rejected because it hides a real schema difference.
