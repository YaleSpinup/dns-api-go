# BlueCat v1 -> v2 API Migration Plan for dns-api-go

> **Status:** Phases 1–2 complete. Phase 2 landed on the v2-migration branch (pending squash/commit hash), validated end-to-end against Yale BAM-test. Phases 3–7 not yet started.

## Context

BlueCat is deprecating the Address Manager v1 REST API (`/Services/REST/v1/*`). dns-api-go currently calls 21 v1 endpoints for DNS record, zone, network, IP, and MAC operations. All must migrate to v2 RESTful endpoints (`/api/v2/*`) before the deprecation window closes. The external API contract (`/v2/dns/...` routes consumed by Spinup UI) must remain unchanged.

The v2 API is fundamentally different: session-based Basic auth instead of token-string auth, RESTful resource paths instead of RPC verbs, structured JSON instead of pipe-delimited strings, and HAL+JSON `_links` instead of `getParent`.

## Findings from Phase 1 (validated against Yale BAM-test, Spinup Testing block)

These resolve or refine several assumptions in the original plan:

- **Login payload requires a `type` field**: `{"type":"UserSession","username":"...","password":"..."}` — not just username/password. Login returns **201 Created**, not 200. Response includes `id`, `apiToken`, `basicAuthenticationCredentials`, `state`. Use `basicAuthenticationCredentials` for the `Authorization: Basic …` header.
- **Logout**: `PATCH /api/v2/sessions/current` with body `{"state":"LOGGED_OUT"}` and `Content-Type: application/merge-patch+json`.
- **Filter predicate syntax (confirmed)**: function-call style — `name:eq('foo')`, `name:contains('foo')`, `address:eq('10.5.0.1')`, `absoluteName:contains('spinuptest')`. Always URL-encode. *(Replaces the original plan's `name:'foo'` shorthand.)*
- **Networks hang off `Block`s, not the configuration root**: Spinup Testing is a `Block`. Path: `/api/v2/blocks/{id}/networks`. Global filter `/api/v2/networks?filter=range:…` should still work for hint search but the hierarchy matters when scoping.
- **`resourceRecords` is filterable globally**: `/api/v2/resourceRecords?filter=absoluteName:contains('…')` works without a zone ID. This **simplifies Phase 5C** — zone-ID resolution from FQDN is needed only for **creation** (`POST /api/v2/zones/{zoneId}/resourceRecords`), not for hint lookup.
- **HostRecord `addresses` shape (contract)**: array of objects `{id, type, address}` — not pipe-delimited strings, not bare IP list. The Phase 1 harness asserts each `HostRecord.addresses[].address` is non-empty.
- **Zone name collisions exist within a view**: a view can contain both a `Zone` and an `ExternalHostsZone` with the same name. Always filter by `type:'Zone'` (or the specific kind) when drilling down.
- **Config now caches both `ConfigurationId` and `ViewId`** (not just configurationId as originally planned). This avoids a `/configurations` and `/views` round-trip on every request. Wired through `internal/common/config.go` and the deco template.
- **OpenAPI spec is pinned in repo**: `docs/bluecat-v2-openapi.json` — authoritative source for field names, status codes, and filter predicates in subsequent phases.
- **Not yet validated**: `MACPool` collection path (`/api/v2/macPools/{id}/macAddresses` from Phase 5E) — Phase 1 only exercised `/configurations/{id}/macAddresses`. Validate during Phase 5E.

## Findings from Phase 2 (validated against Yale BAM-test, live integration tests in `internal/api/v2_live_test.go`)

These confirm assumptions and add detail collected while building the v2 transport layer:

- **`basicAuthenticationCredentials` is used verbatim** as the `Authorization: Basic …` header value — do **not** re-encode it. Live `TestV2Live_SessionCurrent` proves this; the credentials field is already `base64(user:apiToken)`.
- **v2 404 error envelope (confirmed)**: `{"status":404,"reason":"Not Found","code":"ResourceNotFound","message":"The requested resource was not found","detail":"…"}`. Callers in Phases 4–5 can decode this for richer errors, but `api.IsNotFound(err)` is sufficient for the common case.
- **`/api/v2/settings` requires a filter** to isolate a specific settings type — the collection's `data[]` is a `oneOf` of 12 settings subtypes (`SystemSettings`, `GlobalSettings`, `DataCheckerSettings`, etc.). Pattern: `filter=type:eq('SystemSettings')`, URL-encoded via `url.QueryEscape`.
- **`/v2/dns/systeminfo` has zero callers in Spinup** (grepped `ui/`, `*.go`, `*.php`, `*.blade.php`, `*.js`). Migrated to v2 anyway as the cheapest end-to-end smoke test of the new transport, but is a cleanup candidate post-migration.
- **Body retry bug was real**: pre-v2 `MakeRequest` passed the original `io.Reader` to a 401-driven retry, silently failing any retry that needed to replay a body. Fixed by buffering the body to `[]byte` once at function entry and building a fresh `bytes.NewReader` per attempt. Live `TestV2Live_401TriggersRotation` exercises this against real BAM.
- **Live integration tests are valuable beyond mocks** — they validated the `basicAuthenticationCredentials` assumption, surfaced the real 404 envelope shape, and confirmed PATCH-based logout actually invalidates the server-side session. Establishing them as a standing pattern: every subsequent phase adds skip-guarded `TestV2Live_*` tests alongside unit tests.

## Design Decisions

### 1. Keep `ServerInterface` and `MakeRequest` signature unchanged
The existing `MakeRequest(method, route, queryParam string, body io.Reader) ([]byte, error)` is sufficiently general. v2 routes like `/api/v2/zones/123` go in the `route` param, v2 filters like `filter=name:'foo'` go in `queryParam`. No new interface methods needed. `MockServer` stays unchanged, existing test structure preserved.

### 2. Update `MakeRequest` internals for v2 ✅ (Phase 2)
- **Auth header**: `Authorization: Basic {credentials}` instead of bare token
- **Accept header**: Add `Accept: application/hal+json`
- **Status codes**: Accept 2xx range; return `[]byte{}` on 204
- **Body retry bug**: Buffer request body as `[]byte` before first attempt; fresh `bytes.NewReader` per attempt
- **Bounded retry**: 401 rotation uses an explicit `for attempt := 1; attempt <= maxAttempts` loop (max 2) — no recursion, persistent 401 surfaces as `*BluecatAPIError{StatusCode:401}` instead of looping
- **Latent bug fixed**: `getToken()`'s error was being dropped via a shadowed `:=` declaration; now propagated

### 3. New `BluecatV2Entity` model alongside existing, then retire v1
- New model deserializes structured v2 JSON (no pipe-delimited parsing)
- `ToEntity()` maps v2 fields to existing `Entity{ID, Name, Type, Properties map[string]string}`
- `Entity` model is the internal representation — **does not change**
- After migration, remove `BluecatEntity`, `ToBluecatJSON`, and pipe-delimited helpers

### 4. `getParent` replaced by `_links` parsing
v2 has no `/getParent`. Entity responses include `_links.up.href` or `_links.collection.href`. New helper `ExtractIDFromHref(href)` parses parent ID from link URL. `GetParentID` refactored to fetch entity then extract parent from links.

### 5. Type-to-collection mapping for RESTful paths
v2 uses `/api/v2/{collection}/{id}` — need a mapping function:
```
Configuration -> configurations
Zone -> zones
HostRecord/AliasRecord/ExternalHostRecord -> resourceRecords
IP4Network -> networks
IP4Address -> addresses
MACAddress -> macAddresses
MACPool -> macPools
```

### 6. `BluecatAPIError` for distinguishing 404 from 500 ✅ (Phase 2)
v2 returns 404 for missing entities (v1 returned 200 with empty entity). Implemented in `internal/api/server_errors.go` as `BluecatAPIError{StatusCode, Body}` with a top-level `IsNotFound(err error) bool` helper that handles wrapped errors via `errors.As`. Phase 4/5 callers should use `api.IsNotFound(err)` instead of type-asserting at every site.

### 7. Logout is best-effort, deferred from graceful shutdown
`logout()` (PATCH `/api/v2/sessions/current`, `merge-patch+json`, `{"state":"LOGGED_OUT"}`) lives on `*server`. It clears local state (`token`, `sessionID`) unconditionally and swallows server errors. **Not yet wired into shutdown** — `NewServer` blocks on `ListenAndServe()` with no SIGTERM handling. Adding signal handling + `srv.Shutdown()` + a `defer s.logout()` is real scope (changes the server's blocking contract) and is tracked as a follow-up before production rollout.

---

## Implementation Phases

### Phase 1: v2 Endpoint Validation Script ✅ **Complete** (commit `93b183e`)
**Goal**: Validate dns-api-go's planned v2 endpoint shapes against a live BAM instance.
**Delivered**:
- `internal/services/v2_validation_test.go` — skip-guarded harness with two tests:
  - `TestV2_Auth` — login (`POST /api/v2/sessions`) + `GET /api/v2/sessions/current` round-trip
  - `TestV2_DiscoverFixtures` — single discovery walk from `configurations` → `views` → `blocks` (Spinup Testing) → `networks` → `zones` → `zones/{id}/resourceRecords`, plus global `resourceRecords` filter, `networks/{id}/addresses`, `addresses` filter by value, `configurations/{id}/macAddresses`, and pagination shape.
  - Asserts the `HostRecord.addresses[].address` contract.
- `docs/bluecat-v2-openapi.json` — pinned v2 OpenAPI spec.
- `internal/common/config.go` (+test) — `Bluecat` struct extended with `ConfigurationId` **and** `ViewId`.
- `docker/config.deco.json` — adds `configurationId` / `viewId` to the deco template.
- `docker/Dockerfile` — DECO_VERSION bump (1.4.4 → 1.4.8); unrelated maintenance.

Creds source order (built into the harness): `BLUECAT_V2_URL` env → `BLUECAT_V2_CONFIG` env (path to JSON) → `../../docker/config.json` fallback. Scoped read-only to the **Spinup Testing** block (10.5.0.0/16). No mutating calls.

> **Deviation from original plan**: the original called for "one test function per endpoint mapping (21 total)." The delivered shape is one auth test + one discovery walk that exercises the endpoints the migration relies on. The Phase 1 goal — documenting v2 response shapes to drive model design — is met without the 21-test fan-out.

### Phase 2: Authentication Migration ✅ **Complete**
**Goal**: dns-api-go authenticates against BlueCat v2 and makes successful requests.

**Delivered**:

1. **`internal/api/server_errors.go`** — `BluecatAPIError{StatusCode, Body}` typed error with `Error()` method, plus a package-level `IsNotFound(err error) bool` helper that uses `errors.As` to handle wrapped errors. Unit-tested.
2. **`internal/api/server.go`** — added `sessionID int` to the `bluecat` struct so `logout()` can target the active session under `tokenLock`.
3. **`internal/api/helpers.go`**:
   - `generateAuthToken()` rewritten: `POST /api/v2/sessions` with `{"type":"UserSession",…}`, expects **201 Created**, parses `basicAuthenticationCredentials` + `id` from the JSON response, stashes the session ID on `s.bluecat.sessionID`.
   - New `logout()` method: PATCH `/api/v2/sessions/current` with `Content-Type: application/merge-patch+json` and body `{"state":"LOGGED_OUT"}`. Best-effort; clears local state on any outcome. **Not yet wired into shutdown** (see Design Decision #7).
   - New `bluecatHTTPClient()` helper shared by login/logout/MakeRequest.
   - `MakeRequest()` rewritten: bounded retry loop (max 2), buffered body for retry replay, `Authorization: Basic …`, `Accept: application/hal+json`, 2xx success range, `[]byte{}` on 204, `*BluecatAPIError` on non-2xx. Fixed a latent bug where `getToken()` errors were dropped via shadowed `:=`.
4. **`internal/api/handlers.go`** — `SystemInfoHandler` migrated: `GET /api/v2/settings?filter=type:eq('SystemSettings')` (URL-encoded via `url.QueryEscape`), HAL+JSON envelope decoded, `data[0]` flattened to `map[string]string` with `_links` dropped and non-string fields stringified via `fmt.Sprintf("%v", …)`. This handler has zero callers in Spinup but served as the first real consumer of the new transport.

**Tests delivered**:

- `internal/api/server_errors_test.go` — `BluecatAPIError.Error()`, `IsNotFound` across 7 cases including wrapped errors.
- `internal/api/helpers_test.go` — 15 unit tests via `httptest` covering login happy path / 401 / missing credentials field, logout flows, and `MakeRequest` (200/201/204/404/500, header correctness, 401-then-200 body replay, persistent 401 bounding, nil body, query params).
- `internal/api/handlers_test.go` — 3 unit tests for `SystemInfoHandler` (decode + stringification + `_links` removal, empty data → 500, upstream non-2xx → 500).
- `internal/api/v2_live_test.go` — 6 skip-guarded live integration tests against Yale BAM-test, driving the real `server.MakeRequest`/`generateAuthToken`/`logout` paths (separate from the Phase 1 harness's private `v2Client`). Cred sources: env vars → `BLUECAT_V2_CONFIG` → `../../docker/config.json`. Coverage: session-current, configurations list, 404 path, 401 rotation, logout+reauth, `SystemInfoHandler` end-to-end.

**Verified against BAM-test**: full auth → transport → handler chain works end-to-end. 401 rotation tested with deliberately poisoned credentials. Logout confirmed to invalidate the session server-side.

**Deferred to follow-up** (called out so they don't get lost):

- Graceful shutdown + signal handling + `defer s.logout()` wiring. No existing shutdown infrastructure to attach to. Low risk if deferred until after Phases 3–5 land, since sessions are best-effort and BAM cleans them up server-side eventually.
- `/v2/dns/systeminfo` route is unused; flag for removal in Phase 7 cleanup.

### Phase 3: v2 Response Models
**Goal**: New model types for v2 JSON responses, conversion to existing `Entity`.

**Pre-work (do this BEFORE writing models)** — the dominant risk for this phase is `Properties` mapping fidelity. Before touching models:

1. **Enumerate downstream `Properties` consumers**: `grep -rn '\.Properties\[' internal/api/ internal/services/` to list every key v1 callers read from `Entity.Properties`. This is the contract Phase 3 must preserve.
2. **For each entity type touched in Phase 5** (`HostRecord`, `AliasRecord`, `ExternalHostRecord`, `Zone`, `ExternalHostsZone`, `IP4Network`, `IP4Address`, `MACAddress`, `MACPool`, `Configuration`), capture the v2 response shape from BAM-test using the live-test pattern. Compare each consumed `Properties` key against the v2 field names. Document the mapping (likely a small table per type in this section).
3. Watch for non-trivial mappings already known from Phase 1: `HostRecord.addresses` is `[{id, type, address}]` (v2) vs pipe-delimited `addresses=10.5.0.6` (v1); `userDefinedFields` is a nested object, not a flat key/value blob.

**New file `internal/models/bluecat_v2.go`**:
```go
type HALLink struct { Href string }
type HALLinks struct { Self, Collection, Up *HALLink }

type BluecatV2Entity struct {
    ID    int
    Name  string
    Type  string
    Links HALLinks `json:"_links"`
    // Additional fields populated via custom UnmarshalJSON
    // to handle varying entity types
    Properties map[string]string `json:"-"` // built during unmarshal
}

type BluecatV2Collection struct {
    Count int
    Data  []BluecatV2Entity
    Links HALLinks `json:"_links"`
}

func (v2 *BluecatV2Entity) ToEntity() Entity
func ParseV2Entity(data []byte) (*Entity, error)
func ParseV2Entities(data []byte) ([]Entity, error)
func ExtractIDFromHref(href string) (int, error)
```

**`ToEntity()` must produce the same `Properties` keys** that v1 pipe-delimited strings contained — `absoluteName`, `addresses`, `ttl`, etc. — since downstream handlers depend on these. Build the mapping per entity type from the pre-work above; do not infer it.

**Open design question** — generic vs. type-specific parsing:
- The plan as written has one generic `ParseV2Entity` that uses a custom `UnmarshalJSON` to populate `Properties` for any type.
- Given the variety of v2 entity shapes (`oneOf` of 12 settings subtypes, multiple resource record subtypes with different shapes, nested objects like `userDefinedFields`), a per-type parser dispatched by `Type` field may be cleaner than one universal `UnmarshalJSON`.
- **Recommendation**: prototype the generic version first with `HostRecord` (Phase 1's reference shape). If `UnmarshalJSON` complexity balloons when adding `IP4Address` / `MACAddress` / `Network`, split into per-type parsers behind the same `ParseV2Entity` dispatcher.

**Changes to `internal/models/entity.go`**:
- Remove `ToBluecatJSON()` (pipe-delimited serialization)
- For v2 PUT/POST bodies prefer direct `json.Marshal` on per-call request structs over a shared `ToV2JSON()` — request shapes vary too much between create vs. update vs. assign-IP to share a single method.

**Tests for Phase 3** (per the standing pattern):
- Unit tests in `internal/models/bluecat_v2_test.go` driving `ParseV2Entity` / `ParseV2Entities` against captured response fixtures from BAM-test for each entity type touched in Phase 5.
- Live tests can wait until Phase 5 — Phase 3 is pure data transformation, validatable from fixtures.

### Phase 4: Core Service Helpers Migration
**Goal**: Shared building blocks (entity lookup, search, CRUD) work against v2.

**Use the Phase 2 error machinery throughout this phase.** Callers should:
- Treat `*api.BluecatAPIError` as the canonical error from `MakeRequest`.
- Use `api.IsNotFound(err)` (handles wrapped errors) to distinguish missing entities from server errors, instead of inspecting `err.Error()` strings.
- For not-found responses where v1 returned an empty entity (e.g., `GetEntityByID` returning an `Entity{ID:0}` for missing IDs), preserve the v1 contract by translating `IsNotFound(err)` into the empty-entity return — don't propagate the error to handlers that aren't ready for it.

**Changes to `internal/services/helpers.go`** — migrate each function:

| Function | v1 Call | v2 Call | Notes |
|----------|---------|---------|-------|
| `GetConfigID` | `GetEntities(0,1,0,"Configuration",false)` → `/getEntities` | Use cached `s.bluecat.configurationId` (Phase 1) | No network call needed if cached value is set; fall back to `GET /api/v2/configurations?limit=1` only if unset |
| `GetParentID` | `GET /getParent?entityId={id}` | Fetch entity by ID, parse `_links.up.href` via `ExtractIDFromHref` | Verify `_links.up` is present across all entity types we touch — not yet validated for `MACAddress` / `IP4Address` |
| `GetEntityByID` | `GET /getEntityById?id={id}` | `GET /api/v2/{collection}/{id}` | On 404, return v1's empty-entity sentinel rather than `*BluecatAPIError` if any caller depends on the v1 contract |
| `DeleteEntityByID` | Get entity, then `DELETE /delete?objectId={id}` | `DELETE /api/v2/{collection}/{id}` directly (skip the preflight get if v2 returns useful errors on bad delete) | Expect 204 — `MakeRequest` returns `[]byte{}` for that, callers should not parse the body |
| `UpdateEntity` | `PUT /update` with pipe-delimited JSON body | `PUT /api/v2/{collection}/{id}` with structured JSON | Per-entity request shapes; do not share a single update method |
| `GetEntitiesByHintHelper` | Generic hint route dispatcher | Remove — callers migrated to direct v2 filter queries |
| `GetEntities` | `GET /getEntities?parentId=...&type=...` | `GET /api/v2/{parentCollection}/{parentId}/{childCollection}` | Use `BluecatV2Collection` to decode |
| `GetEntityByName` | `GET /getEntityByName?name=...&type=...` | `GET /api/v2/{collection}?filter=name:eq('{name}')` | URL-encode the filter value; remember single-quoted literal inside the predicate |
| `searchObjectByTypes` | `GET /searchObjectByTypes?keyword=...&types=...` | `GET /api/v2/{collection}?filter={predicate}` with type constraints | One collection per call — there is no global cross-type search endpoint |

**New helper**: `typeToCollection(entityType string) string` mapping entity types to v2 collection paths.

**New helper**: `buildFilter(predicates ...string) string` that joins predicates with ` and ` and `url.QueryEscape`s the whole value, returning the `filter=…` query param. Keeps filter construction out of every caller.

**Tests** (per the standing pattern):
- Unit tests for each migrated helper using the existing mock-server infrastructure in `internal/services/`, with v2 JSON fixtures.
- `internal/services/v2_helpers_live_test.go` — skip-guarded live test that exercises `GetConfigID`, `GetParentID`, `GetEntityByID`, `GetEntities`, `GetEntityByName` against BAM-test for safe read-only cases (Spinup Testing block).

### Phase 5: Service Layer Migration
**Goal**: All domain operations (zones, records, IPs, MACs) use v2 endpoints.

**Standing pattern for each sub-phase 5A–5E**: add `internal/services/{service}_v2_live_test.go` exercising read-only operations against BAM-test (Spinup Testing block, no mutating calls in CI). Mutating tests (CreateRecord, DeleteIpAddress, AddMacAddress) should be gated behind an additional opt-in env var (e.g., `BLUECAT_V2_ALLOW_MUTATIONS=1`) so they don't run by default and create churn against the shared test instance.

#### 5A: ZoneService (`internal/services/zone_service.go`)
- `GetEntitiesByHint` → `GET /api/v2/zones?filter=name:contains('{hint}') and type:eq('Zone')&offset={s}&limit={c}` — include `type:eq('Zone')` to exclude `ExternalHostsZone` collisions
- `GetEntity` → `GET /api/v2/zones/{id}`

#### 5B: NetworkService (`internal/services/network_service.go`)
- `GetEntitiesByHint` → `GET /api/v2/networks?filter=range:startsWith('{prefix}')&offset={s}&limit={c}`
- `GetEntity` → `GET /api/v2/networks/{id}`
- **Hierarchy note** (Phase 1 finding): networks hang off `Block`s, not the configuration root. Scoped path is `/api/v2/blocks/{blockId}/networks`. Global filter on `/api/v2/networks` still works for hint search, but parent-scoped reads need the block ID.

#### 5C: RecordService (`internal/services/record_service.go`) — **most complex**
- `GetEntity` → `GET /api/v2/resourceRecords/{id}` (or filter-based if collection-scoped)
- `getHostOrAliasRecordsByHint` → `GET /api/v2/resourceRecords?filter=absoluteName:contains('{hint}') and type:eq('{type}')` — **global filter works; no zone-ID resolution required for lookups** (Phase 1 finding). Use `type:eq('HostRecord')`, `type:eq('AliasRecord')`, etc.
- `getExternalRecord` → queries on ExternalHostsZone. Discovery: `GET /api/v2/views/{viewId}/zones?filter=name:eq('{name}') and type:eq('ExternalHostsZone')` — **always include `type:eq(...)`** to avoid the name-collision case (a view can hold a `Zone` and an `ExternalHostsZone` with the same name; Phase 1 hit this on the `internal` view).
- `CreateRecord` (all types) → `POST /api/v2/zones/{zoneId}/resourceRecords` with JSON body differentiated by `type` field — replaces three separate prep functions. **Creation is the one path that still needs zone-ID resolution from FQDN.**
- **New helper needed**: zone resolution from FQDN (parse `host.sub.example.com`, look up zone `sub.example.com` or `example.com`). Only required for create/update, not lookup.
- v2 response returns full entity (not just ID), so follow-up `GetEntity` call may be unnecessary.

#### 5D: IpAddressService (`internal/services/ip_address_service.go`)
- `GetIpAddress` → `GET /api/v2/addresses?filter=address:eq('{addr}')`
- `DeleteIpAddress` → get ID, then `DELETE /api/v2/addresses/{id}`
- `AssignIpAddress` → `POST /api/v2/networks/{parentId}/addresses` with JSON body (action, macAddress, hostInfo, properties)

#### 5E: MacAddressService (`internal/services/mac_address_service.go`)
- `GetMacAddress` → `GET /api/v2/macAddresses?filter=address:eq('{mac}')` (or scoped under configuration: `/api/v2/configurations/{configId}/macAddresses?filter=address:eq('{mac}')`)
- `AddMacAddress` → `POST /api/v2/configurations/{configId}/macAddresses` with JSON body
- `AssociateMacAddress` → `POST /api/v2/macPools/{poolId}/macAddresses` with MAC entity ref — **`macPools` collection still not validated** as of Phase 2; Phase 1 only exercised `/configurations/{id}/macAddresses`. Before implementing 5E:
  1. Extend the Phase 1 harness (or add a small probe) with a read-only `GET /api/v2/macPools` against BAM-test to confirm the collection exists and capture its shape.
  2. Verify `POST /api/v2/macPools/{poolId}/macAddresses` matches the OpenAPI spec at `docs/bluecat-v2-openapi.json`.
- `UpdateMacAddress` → `PUT /api/v2/macAddresses/{id}` with JSON body

### Phase 6: Test Updates
**Goal**: All tests reflect v2 request/response shapes; live coverage matches unit coverage.

Most test work happens *within* Phases 2–5 (the standing pattern: each phase adds unit + skip-guarded live tests). This phase covers what's left:

- Audit `MockServer.MakeRequestFunc` test callbacks — any v1 pipe-delimited responses still in fixtures must be replaced with v2 JSON.
- Replace `bluecat_entity_test.go` with `bluecat_v2_test.go` covering `ToEntity()` mappings per entity type (using captured BAM fixtures).
- End-to-end contract check: hit each `/v2/dns/...` route through the full handler stack and assert the response JSON matches the pre-migration v1 shape (snapshot test). Especially important for `Properties`-derived fields exposed to Spinup UI.
- Confirm all skip-guarded live tests across Phases 2–5 still pass against BAM-test in a single run: `BLUECAT_V2_CONFIG=… go test ./... -run V2Live`.

### Phase 7: Cleanup
- Delete `internal/models/bluecat_entity.go` (and test file)
- Remove `Entity.ToBluecatJSON()` from `entity.go`
- Remove dead v1 helpers from `services/helpers.go` (`GetEntitiesByHintHelper` if no longer used)
- Verify `common.ConvertToSeparatedString` / `ConvertToMap` still needed (used by API handlers for client input parsing — likely stays)
- **Wire `logout()` into graceful shutdown** (deferred from Phase 2): add SIGTERM/SIGINT handling in `main.go`, switch `NewServer` to a non-blocking start + `srv.Shutdown()` path, `defer s.logout()` on the way out.
- **Decide on `/v2/dns/systeminfo`**: no callers exist in Spinup as of Phase 2. Either remove the route entirely or document it as an admin/probe endpoint.

---

## Files Modified (by phase)

| Phase | Files | Type |
|-------|-------|------|
| 1 | `internal/services/v2_validation_test.go` | New |
| 1 | `docs/bluecat-v2-openapi.json` | New |
| 1 | `internal/common/config.go`, `docker/config.deco.json`, `docker/Dockerfile` | Edit |
| 2 | `internal/api/server.go` (`sessionID` field) | Edit |
| 2 | `internal/api/server_errors.go` (`BluecatAPIError`, `IsNotFound`) | Edit |
| 2 | `internal/api/helpers.go` (`generateAuthToken`, `logout`, `MakeRequest`, `bluecatHTTPClient`) | Edit (major) |
| 2 | `internal/api/handlers.go` (`SystemInfoHandler`) | Edit |
| 2 | `internal/api/server_errors_test.go`, `internal/api/helpers_test.go`, `internal/api/handlers_test.go` | New / Edit |
| 2 | `internal/api/v2_live_test.go` | New (live integration tests) |
| 3 | `internal/models/bluecat_v2.go` | New |
| 3 | `internal/models/entity.go` | Edit |
| 4 | `internal/services/helpers.go` | Edit (major) |
| 4 | `internal/services/v2_helpers_live_test.go` | New |
| 5A | `internal/services/zone_service.go` (+ `_v2_live_test.go`) | Edit / New |
| 5B | `internal/services/network_service.go` (+ `_v2_live_test.go`) | Edit / New |
| 5C | `internal/services/record_service.go` (+ `_v2_live_test.go`) | Edit (major) / New |
| 5D | `internal/services/ip_address_service.go` (+ `_v2_live_test.go`) | Edit / New |
| 5E | `internal/services/mac_address_service.go` (+ `_v2_live_test.go`) | Edit / New |
| 6 | `*_test.go` files, `internal/mocks/mock_server.go` | Edit |
| 7 | `internal/models/bluecat_entity.go` | Delete |
| 7 | `main.go`, `internal/api/server.go` (graceful shutdown + `defer logout`) | Edit |
| 7 | `internal/api/routes.go` (drop `/systeminfo` if no caller emerges) | Edit |

## Key Risks

1. **Properties mapping fidelity** *(top risk; now blocks Phase 3)*: v2 `ToEntity()` must produce identical `Properties` keys to v1 pipe-delimited output. Phase 1 captured the `HostRecord` shape. Still need to enumerate every key actually read by downstream callers (`grep -rn '\.Properties\[' internal/`) and confirm the v2 source field per entity type. Plan handles this via Phase 3 pre-work.
2. **`_links` reliability**: `GetParentID` replaces `/getParent` by parsing `_links.up.href`. Not yet verified on `MACAddress`, `IP4Address`, `IP4Network`, or `Configuration` — confirm during Phase 4.
3. **MAC address / macPool paths**: `/configurations/{id}/macAddresses` validated; `/macPools/{id}/macAddresses` (Phase 5E) still unvalidated. Add a probe before Phase 5E.
4. **Zone name collisions**: A view can contain both a `Zone` and `ExternalHostsZone` with the same name. All zone-by-name filters must include `type:eq(...)`.
5. **Zone ID requirement for record _creation_** (narrowed by Phase 1): only creation needs zone-ID resolution from FQDN — lookups can use the global `resourceRecords` filter.
6. **Mutating tests against shared BAM-test**: gating mutations behind `BLUECAT_V2_ALLOW_MUTATIONS=1` keeps CI safe but means mutation paths only run by deliberate developer action. Worth documenting in `README` so this doesn't lead to silent regressions.
7. ~~**Filter predicate syntax**~~: Resolved — `field:eq('value')`, `field:contains('value')`, `field:startsWith('value')`. Combine with ` and ` / ` or `. Always `url.QueryEscape` the predicate value.
8. ~~**Body retry bug**~~: Resolved in Phase 2 — body buffered to `[]byte`, fresh `bytes.NewReader` per attempt, bounded retry loop. Tested live.
9. ~~**`basicAuthenticationCredentials` encoding**~~: Resolved in Phase 2 — the value is already `base64(user:apiToken)` and is used verbatim as the Basic auth header value. Confirmed live.

## Verification

Each phase ships with both unit tests (via `httptest` / mocks) **and** skip-guarded live tests against BAM-test. Live tests are gated on creds being reachable (env vars → `BLUECAT_V2_CONFIG` → `../../docker/config.json`) so the offline suite stays green.

1. **Phase 1**: ✅ Run validation harness against BAM-test; `internal/services/v2_validation_test.go` exercises the endpoints the migration relies on.
2. **Phase 2**: ✅ `go test ./internal/api/...` passes offline; `BLUECAT_V2_CONFIG=… go test ./internal/api/ -run V2Live` passes against BAM-test. Validates auth → MakeRequest → `SystemInfoHandler` end-to-end including 401 rotation and logout.
3. **Phase 3**: `go test ./internal/models/...` against captured BAM fixtures; `ToEntity()` mappings match v1 `Properties` keys for every entity type touched in Phase 5.
4. **Phase 4**: `go test ./internal/services/...` and `BLUECAT_V2_CONFIG=… go test ./internal/services/ -run V2Live` pass; helpers correctly translate `IsNotFound` to v1-compatible empty-entity returns where required.
5. **Phase 5**: Per-service live tests pass; mutating tests pass under `BLUECAT_V2_ALLOW_MUTATIONS=1`.
6. **Phase 6**: `go test ./...` passes; snapshot tests confirm `/v2/dns/...` response JSON matches pre-migration v1 output for every consumed key.
7. **End-to-end**: Deploy to test environment, verify Spinup UI operations (zone browse, record create/delete, IP assign, MAC create) work identically.

## Open Questions

- ~~Is v2 BlueCat test instance accessible from dev machine for Phase 1 validation?~~ **Resolved** — Yale BAM-test is reachable; Phase 1 harness and Phase 2 live tests both run against it.
- ~~What BlueCat Address Manager version / v2 API version are we targeting?~~ **Resolved** — see `docs/bluecat-v2-openapi.json`. Live BAM-test runs `25.1.1-1157.GA.bcn` (confirmed via `SystemInfoHandler`).
- ~~Do you have v2 API documentation or a Swagger/OpenAPI spec we can reference for exact field names?~~ **Resolved** — OpenAPI spec pinned at `docs/bluecat-v2-openapi.json`.
- ~~Where does this plan live going forward?~~ **Resolved** — `dns-api-go/docs/v2-migration-plan.md`.
- ~~Should we maintain v1/v2 coexistence during rollout (feature flag)?~~ **Leaning hard cutover.** Phase 2 chose direct replacement (no v1/v2 dual-call path). Splitting at the `MakeRequest` layer would have added significant complexity for minimal benefit; the deployment unit (a single Go binary) flips atomically. Final decision should be confirmed before Phase 7 closes.
- **NEW** — Is there a `MACPool` actually configured on Yale's BAM-test? If not, Phase 5E's `AssociateMacAddress` migration may need a manual setup step or a stub validation against a different MAC pool to confirm the contract.
- **NEW** — Does Spinup UI have any indirect dependency on v1 error-message formats? Phase 2's switch to `*BluecatAPIError` returns a different error string shape via `Error()`; if any handler propagates `err.Error()` to the wire (we confirmed several do via `http.Error(w, err.Error(), …)`), UI code matching on old strings would silently degrade. Worth a grep across `ui/` before Phase 5 ships.
