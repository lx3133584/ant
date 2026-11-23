---
id: 157
title: "Insert validation leaves zero default fields to the database"
area: hooks
status: active
tags: [Validate, default, Insert, Upsert, Required, zero-value]
issues: [309]
related: [47, 119]
changed_by: []
code: [hooks.go]
tests: [tests/features/lifecycle/validate_test.go]
---

# 157. Insert validation leaves zero default fields to the database

**In short:** Field rules skip a zero `default=` field on an actual insert, because the database supplies the value; updates and upserts still validate that zero.

## Decision

A model default makes a zero field mean `DEFAULT` on insert, but field rules ran before the insert and rejected that zero. `Required` and `OneOf` could therefore refuse a row whose database default was valid (#309). `Insert` and the insert paths of `SaveChanges`, including batched inserts, now skip a rule when its field has `Default` and the entity holds its zero value. A nonzero value is still checked, and `WithoutHooks` keeps its existing behavior.

An update writes the zero instead of taking a default, so it must keep validating it. `Upsert` also keeps checking: it runs insert hooks whichever branch the database takes, and decision 119 makes its update branch write the zero the entity holds. Skipping validation there could let an invalid zero through on conflict. Applying the skip to every `beforeInsert` call was rejected for that reason; the unit's `insert` flag, which a standalone `Upsert` does not set, distinguishes the actual insert paths without changing the hook order or API.

Copying the default into Go in a `BeforeInsert` hook remains possible but is no longer required. A model-level `Validate(ctx)` hook is not a field rule and still sees the entity as supplied; it can apply whatever default-aware policy the model chooses.
