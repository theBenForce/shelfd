# Multi-Library Plugin Config via Secret Array-Item Annotations

* Status: accepted
* Deciders: Lead Systems Architect, @core, @security, @librarian
* Date: 2026-11-22

## Context and problem statement

ADR-0022 encrypted per-user plugin config by walking `config_schema` for properties flagged `"x-shelfd-secret": true` or `"format": "password"`. That walk only covered **top-level** object properties. The first multi-account connector needs **one cookie header per configured site** (each site is its own host with its own session jar), i.e. a config shape like:

```json
{ "libraries": [ { "id": "site-a", "cookie": "<Cookie header>" }, … ] }
```

A single top-level `cookie` secret cannot express N libraries, and Flutter's JSON-Schema-driven `PluginConfigDialog` had no card type for repeatable object arrays. The crypto layer also matched stored array elements to incoming ones **by position**, so reordering or deleting a row would silently bind one user's ciphertext to another library's record.

How should Shelfd support any number of named, individually-secret sub-accounts inside one plugin config, with correct redaction, cURL-paste, and per-record key binding?

## Decision drivers

* Store arbitrary repeatable sub-account lists for every future multi-account connector without per-plugin Go code.
* Encrypt each item's secret fields independently at rest so redaction and rotation stay per-record.
* Reorder/delete/add rows must never bind ciphertext to the wrong record.
* One generic Flutter form widget covers every future repeatable-array plugin config.
* Keep the existing AES-256-GCM / HKDF key derivation from `jwt_secret` + plugin name (no new key material).

## Considered options

* **Schema-level walker: declare secrets on nested object properties (`items.properties`) and match array items by stable non-secret identity (Chosen)**
* A single top-level `"cookie": "lib1=…; lib2=…"` blob parsed inside the plugin
* Per-library child plugins (one install per subdomain)
* Dedicated multi-account table in the database

## Decision outcome

Chosen option: **Schema-level walker — secrets declared on nested object properties, array items matched by stable non-secret identity before encryption.**

1. **Manifest annotations (`apps/server/internal/plugins`):**
   * `config_schema` may use a JSON-Schema `array` of `object` items whose `items.properties` carry their own `"x-shelfd-secret": true` / `"format": "password"` annotations (e.g. `libraries[].cookie`).
   * Manifest validation preserves these item-level annotations end-to-end (`TestManifest_ConfigSchemaPreservesItemSecretAnnotations`), so the same schema drives server-side crypto **and** Flutter rendering.
2. **Walker-based storage path (`crypto.go`):**
   * `PrepareConfigForStorage` walks nested objects and arrays-of-objects; every annotated property is collected via `secretMetaForProp` into a flat set of secret paths and encrypted separately (AES-256-GCM, nonce‖ciphertext base64) while non-secret values pass through unchanged.
   * Smart cURL-paste extraction applies to each annotated cookie field, not just top-level ones.
   * `GET …/plugins/{name}/config` masks every set secret with `"__SHELFD_REDACTED__"`, including array-item secrets.
3. **Identity-matched array binding (`prepareObjectArray` + `matchExistingIndex`):**
   * Before encrypting an incoming array, each element is matched against stored elements by a stable **non-secret** identity field when one exists on both sides (e.g. `id`); otherwise positional matching is kept as the fallback.
   * Matched elements keep their existing ciphertext/decrypted prior value semantics per record; unmatched new elements get fresh encryption. Surviving records therefore always decrypt to their own cookie — reordering, deleting, or re-adding a row cannot cross-bind secrets (covered by `TestCrypto_ArrayItemSecrets_RoundTripRedactAndCurlPaste` and `TestCrypto_ArrayItemSecrets_ReorderAndDeleteMatchByIdentity`).
4. **Flutter repeatable cards (`plugin_config_dialog`):**
   * The schema-driven dialog renders arrays-of-objects as repeatable cards with add/remove rows; each row's secret field participates in standard redact-and-patch PUT semantics.
5. **Consumers:** multi-account plugins simply declare schemas like `libraries: [{ id, cookie }]`; each plugin resolves its own hosts and reports per-record session expiry through standard plugin error paths.

## Consequences

### Positive consequences

* Any future multi-account connector ships config, not Go: declare `items.properties` secrets and both the server crypto and the Flutter form pick them up automatically.
* Per-record encryption with identity matching eliminates a whole class of "wrong-library cookie after reordering" incidents without client involvement.
* Existing single-object plugin configs are unaffected: the walker degrades to the previous top-level behavior when no array items are present.

### Negative consequences

* Config schemas become slightly more expressive than some JSON-Schema validators the team uses for linting; item-level annotations must be preserved through every manifest round-trip (pinned by a dedicated test).
* Identity matching depends on the presence of a stable non-secret field; schemas whose arrays lack one fall back to positional binding and carry the old reorder hazard — plugin authors are expected to include an identity field (documented in the SDK notes).
