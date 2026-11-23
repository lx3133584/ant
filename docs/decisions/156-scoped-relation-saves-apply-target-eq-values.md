---
id: 156
title: "Scoped relation saves apply target Eq values"
area: relations
status: active
tags: [Where, Eq, SaveChanges, scoped-relation]
issues: [311]
related: [32, 65, 93]
changed_by: []
code: [unit.go]
tests: [tests/features/relations/advanced_test.go]
---

# 156. Scoped relation saves apply target Eq values

**In short:** A row entering a direct scoped relation receives its target-side `Eq` values, while other scope conditions remain read filters.

## Decision

A relation `Where` filtered loads but did not affect saves, so a row appended with the zero value of a scoped column was stored outside the relation it had just joined (#311). `Morph` already established that a relation writes the values which make a new member visible. A direct relation now applies its target-side `Eq` values as well when it gains a row.

Only a direct comparison with a bound value implies a value to write. `Ne`, `In`, `Latest`, `Or` and the other conditions stay read-only, and many-to-many links change only their pivot rows. The normal writable and updatable field lists still apply. A row already in the loaded relation is not stamped again, so editing its scoped column can move it out of the relation. An existing untracked row writes only its foreign key and changed scope fields, not every zero in the struct passed to the relation. A pointer-valued condition is copied one value deep for each row, so editing a row cannot rewrite the relation's scope or a sibling.

Documenting scoped relations as read-only was rejected because it would preserve the silent disappearance and make `Where` inconsistent with `Morph`. Reapplying scopes to every loaded row was also rejected: it would overwrite an explicit edit that the current foreign-key rule deliberately preserves.
