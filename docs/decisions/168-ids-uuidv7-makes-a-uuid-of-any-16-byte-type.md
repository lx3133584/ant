---
id: 168
title: "ids.UUIDv7 makes a UUID of any 16-byte type"
area: models
status: active
tags: [ids.UUIDv7, ids.UUID, GenerateKey, uuid, keys, text]
issues: [344]
related: [150]
changed_by: []
code: [ids/ids.go]
tests: [ids/ids_test.go, tests/features/keys_and_identity/uuidv7_key_test.go]
---

# 168. ids.UUIDv7 makes a UUID of any 16-byte type

**In short:** `ids.UUIDv7[T]()` returns a version 7 UUID as any type whose underlying type is `[16]byte`, so `ant.GenerateKey(b, &Users.ID, ids.UUIDv7[uuid.UUID])` fills a key that PostgreSQL stores in a `uuid` column. `ids.UUID` stays, and is the same value as text for a `string` key, which is a `text` column.

## Decision

`ids.UUID` returns a string, so the key it fills is a `string` field, and a `string` field is a `text` column. The key and every foreign key to it then take 37 bytes where a `uuid` takes 16, in the table and in each index; any string is accepted as a key, since `GenerateKey` keeps a key the application supplies; and changing the type later rewrites every table that holds it (#344). A `uuid` column needed `.Type("uuid")` on every model, or a `uuid.UUID` key with a wrapper around `uuid.NewV7`, which returns an error as well and so is not a `func() V`.

`ids.UUIDv7[T ~[16]byte]() T` fills the layout `ids.UUID` already built, a 48-bit millisecond timestamp, then random bits, with the version and variant set, and returns it as `T`. PostgreSQL's column type goes by the shape of the field, sixteen bytes, and not by its name, so the key is a `uuid` column for `uuid.UUID` of `github.com/google/uuid`, for another library's type, and for a plain `[16]byte`. `ids.UUID` formats what `UUIDv7` returns, so there is one layout.

A `string` key stays possible, with `.Type("uuid")` for a `uuid` column: a program that already has `string` keys keeps its fields, and pays no rewrite.

Rejected:

- Returning `uuid.UUID`: `ids` would import a UUID library, and choose it for every program that uses another.
- Changing what `ids.UUID` returns: every `string` key filled by it would stop compiling.
- Allowing only named types, or only those with a `String` method: the column type does not depend on either, and a plain `[16]byte` key is written and read like any other.
- Making a `string` key that `ids.UUID` fills a `uuid` column: `GenerateKey` is given a function, and nothing says the strings it returns are UUIDs. `.Type("uuid")` says it.
