# BlueCat v1 -> v2 API Migration Plan for dns-api-go

> **Status:** Phase 1 complete (commit `93b183e`, 2026-05-15). Phases 2–7 not yet started.

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

## Design Decisions

### 1. Keep `ServerInterface` and `MakeRequest` signature unchanged
The existing `MakeRequest(method, route, queryParam string, body io.Reader) ([]byte, error)` is sufficiently general. v2 routes like `/api/v2/zones/123` go in the `route` param, v2 filters like `filter=name:'foo'` go in `queryParam`. No new interface methods needed. `MockServer` stays unchanged, existing test structure preserved.

### 2. Update `MakeRequest` internals for v2
- **Auth header**: `Authorization: Basic {credentials}` instead of bare token
- **Accept header**: Add `Accept: application/hal+json`
- **Status codes**: Accept 2xx range (200, 201, 204) instead of only 200
- **Body retry bug**: Buffer request body as `[]byte` before first attempt so 401 retries work with bodies (pre-existing bug, fix during migration)

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

### 6. `BluecatAPIError` for distinguishing 404 from 500
v2 returns 404 for missing entities (v1 returned 200 with empty entity). Create typed error with status code so callers can detect not-found vs server errors without changing `MakeRequest` return signature.

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

### Phase 2: Authentication Migration
**Goal**: dns-api-go authenticates against BlueCat v2 and makes successful requests.

**Changes to `internal/api/helpers.go`**:
1. `generateAuthToken()`:
   - FROM: `GET {baseUrl}/login?username=...&password=...` → parse `"Session Token-> {TOKEN} <- for User : {user}"`
   - TO: `POST {baseUrl}/api/v2/sessions` with JSON body `{"type":"UserSession","username":"...","password":"..."}` and headers `Content-Type: application/json`, `Accept: application/hal+json` → expect **201 Created** → parse `basicAuthenticationCredentials` from JSON response. (Reference implementation: `v2_validation_test.go:105`.)
   - Add a logout path: `PATCH /api/v2/sessions/current` with `Content-Type: application/merge-patch+json` and body `{"state":"LOGGED_OUT"}`, invoked on graceful shutdown / token-rotation.
2. `MakeRequest()`:
   - Auth header: `Authorization: Basic {basicAuthenticationCredentials}` (the credentials field is the pre-encoded `base64(user:apiToken)`, do not re-encode)
   - Add `Accept: application/hal+json` header
   - Success check: `statusCode < 200 || statusCode >= 300` (was `!= 200`)
   - Return `[]byte{}` for 204 No Content
   - Fix body retry: buffer body as `[]byte`, use `bytes.NewReader` for each attempt
   - Return typed `BluecatAPIError` for non-2xx responses (includes status code)

**Changes to `internal/api/handlers.go`**:
3. `SystemInfoHandler()`:
   - FROM: `GET /getSystemInfo` → parse pipe-delimited string
   - TO: `GET /api/v2/settings?filter=type:'SystemSettings'` → parse JSON response

**Changes to `internal/api/server_errors.go`** (or new in helpers.go):
4. Add `BluecatAPIError{StatusCode int, Body string}` error type

### Phase 3: v2 Response Models
**Goal**: New model types for v2 JSON responses, conversion to existing `Entity`.

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

Key: `ToEntity()` must produce the **same `Properties` keys** that v1 pipe-delimited strings contained (e.g., `absoluteName`, `addresses`, `ttl`), since downstream handlers depend on these keys. This mapping is entity-type-specific and will be finalized after Phase 1 validation confirms v2 field names.

**Changes to `internal/models/entity.go`**:
- Remove `ToBluecatJSON()` (pipe-delimited serialization)
- Add `ToV2JSON() ([]byte, error)` for v2 PUT/POST request bodies (or just use `json.Marshal` directly)

### Phase 4: Core Service Helpers Migration
**Goal**: Shared building blocks (entity lookup, search, CRUD) work against v2.

**Changes to `internal/services/helpers.go`** — migrate each function:

| Function | v1 Call | v2 Call |
|----------|---------|---------|
| `GetConfigID` | `GetEntities(0,1,0,"Configuration",false)` → `/getEntities` | `GET /api/v2/configurations?limit=1` |
| `GetParentID` | `GET /getParent?entityId={id}` | Fetch entity by ID, parse `_links.up.href` |
| `GetEntityByID` | `GET /getEntityById?id={id}` | `GET /api/v2/{collection}/{id}` (use `expectedTypes` to pick collection) |
| `DeleteEntityByID` | Get entity, then `DELETE /delete?objectId={id}` | Get entity, then `DELETE /api/v2/{collection}/{id}` |
| `UpdateEntity` | `PUT /update` with pipe-delimited JSON body | `PUT /api/v2/{collection}/{id}` with structured JSON |
| `GetEntitiesByHintHelper` | Generic hint route dispatcher | Remove — callers migrated to direct v2 filter queries |
| `GetEntities` | `GET /getEntities?parentId=...&type=...` | `GET /api/v2/{parentCollection}/{parentId}/{childCollection}` |
| `GetEntityByName` | `GET /getEntityByName?name=...&type=...` | `GET /api/v2/{collection}?filter=name:'{name}'` |
| `searchObjectByTypes` | `GET /searchObjectByTypes?keyword=...&types=...` | `GET /api/v2?filter={predicate}` with type constraints |

**New helper**: `typeToCollection(entityType string) string` mapping entity types to v2 collection paths.

### Phase 5: Service Layer Migration
**Goal**: All domain operations (zones, records, IPs, MACs) use v2 endpoints.

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
- `AssociateMacAddress` → `POST /api/v2/macPools/{poolId}/macAddresses` with MAC entity ref — **`macPools` collection not yet validated in Phase 1**; confirm exact path/shape against the OpenAPI spec or add a targeted validation test before implementing.
- `UpdateMacAddress` → `PUT /api/v2/macAddresses/{id}` with JSON body

### Phase 6: Test Updates
**Goal**: All tests reflect v2 request/response shapes.

- Update `MockServer.MakeRequestFunc` test callbacks to return v2 JSON (not pipe-delimited)
- Update `bluecat_entity_test.go` → new `bluecat_v2_test.go` testing `ToEntity()` mappings
- Update `entity_service_test.go` with v2 response shapes
- Add service-level tests for each migrated service file
- Validate external API contract produces identical JSON output

### Phase 7: Cleanup
- Delete `internal/models/bluecat_entity.go` (and test file)
- Remove `Entity.ToBluecatJSON()` from `entity.go`
- Remove dead v1 helpers from `services/helpers.go` (`GetEntitiesByHintHelper` if no longer used)
- Verify `common.ConvertToSeparatedString` / `ConvertToMap` still needed (used by API handlers for client input parsing — likely stays)

---

## Files Modified (by phase)

| Phase | Files | Type |
|-------|-------|------|
| 1 | `internal/services/v2_validation_test.go` | New |
| 2 | `internal/api/helpers.go`, `internal/api/handlers.go`, `internal/api/server_errors.go` | Edit |
| 3 | `internal/models/bluecat_v2.go` | New |
| 3 | `internal/models/entity.go` | Edit |
| 4 | `internal/services/helpers.go` | Edit (major) |
| 5A | `internal/services/zone_service.go` | Edit |
| 5B | `internal/services/network_service.go` | Edit |
| 5C | `internal/services/record_service.go` | Edit (major) |
| 5D | `internal/services/ip_address_service.go` | Edit |
| 5E | `internal/services/mac_address_service.go` | Edit |
| 6 | `*_test.go` files, `internal/mocks/mock_server.go` | Edit |
| 7 | `internal/models/bluecat_entity.go` | Delete |

## Key Risks

1. **Properties mapping fidelity**: v2 `ToEntity()` must produce identical `Properties` keys to v1 pipe-delimited output. Phase 1 captured the HostRecord shape (`addresses` is array of `{id, type, address}` objects); still need to enumerate the rest by entity type before/during Phase 3.
2. **Zone ID requirement for record _creation_** (narrowed by Phase 1): only creation needs zone-ID resolution from FQDN — lookups can use the global `resourceRecords` filter.
3. **getParent removal**: Must verify `_links` structure is reliable across all entity types. Not yet exercised in Phase 1.
4. **MAC address / macPool paths**: `/configurations/{id}/macAddresses` validated; `/macPools/{id}/macAddresses` (Phase 5E) not yet validated.
5. ~~**Filter predicate syntax**~~: Resolved — `field:eq('value')`, `field:contains('value')`, `field:startsWith('value')`. Combine with ` and ` / ` or `.
6. **Body retry bug**: Existing `MakeRequest` 401 retry fails when body is consumed. Fix during auth migration.
7. **Zone name collisions**: A view can contain both a `Zone` and `ExternalHostsZone` with the same name. All zone-by-name filters must include `type:eq(...)`.

## Verification

1. **Phase 1**: Run validation script against BlueCat test instance — all 21 endpoints return expected shapes
2. **Phase 2**: `go test ./internal/api/...` passes; service can authenticate to v2
3. **Phase 3-5**: `go test ./...` passes after each phase; no v1 calls remain
4. **End-to-end**: Deploy to test environment, verify Spinup UI operations (zone browse, record create/delete, IP assign, MAC create) work identically

## Open Questions

- ~~Is v2 BlueCat test instance accessible from dev machine for Phase 1 validation?~~ **Resolved** — Yale BAM-test is reachable; Phase 1 harness runs against it.
- ~~What BlueCat Address Manager version / v2 API version are we targeting?~~ **Resolved** — see `docs/bluecat-v2-openapi.json` (BlueCat Address Manager RESTful v2 API, HAL+JSON).
- ~~Do you have v2 API documentation or a Swagger/OpenAPI spec we can reference for exact field names?~~ **Resolved** — OpenAPI spec pinned at `docs/bluecat-v2-openapi.json`.
- Should we maintain v1/v2 coexistence during rollout (feature flag), or is this a hard cutover once tested? **Still open.**
- Where does this plan live going forward? Suggested: `dns-api-go/docs/v2-migration-plan.md` (alongside the OpenAPI spec).
