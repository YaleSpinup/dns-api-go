# BlueCat v1 -> v2 API Migration Plan for dns-api-go

> **Status:** Phases 1–7 complete on `tl694-rest-v2-migration`, validated end-to-end against Yale BAM-test (Spinup Testing block, 10.5.0.0/26). Read-only V2Live tests all green; wire-contract snapshot pins the cross-repo shape with `server-api/lib/dns/proteus.rb`. Mutation tests were exercised during development but removed before merge — see §Risks #4. Ready for review/merge.

## Context

BlueCat is deprecating the Address Manager v1 REST API (`/Services/REST/v1/*`). dns-api-go currently calls 21 v1 endpoints. All must migrate to v2 RESTful endpoints (`/api/v2/*`) before the deprecation window closes.

The v2 API is fundamentally different: session-based Basic auth instead of token-string auth, RESTful resource paths instead of RPC verbs, structured JSON instead of pipe-delimited strings, and HAL+JSON `_links` instead of `getParent`.

## Scope

This is a **tactical** v1→v2 migration of a thin consumer, not a full BlueCat v2 client rewrite. The Spinup architecture is converging toward a single Laravel monolith (`ui/`); the long-term destination for DNS functionality is direct PHP→BlueCat-v2 calls from `ui/`, with dns-api-go retired. This migration buys deprecation runway without locking in a heavyweight Go client.

### Consumer audit (May 2026)

Two production consumers across the Spinup + SpinupManaged ecosystems. The full exercised surface (`spinup/{ui,server-api,server-api-go,ec2-api,ec2-api-go,*-api}`, `spinupmanaged/{server-api,go_ui}`):

| dns-api-go route | Caller(s) | Response fields consumed |
|---|---|---|
| `POST /v2/dns/{acct}/records` | `server-api` (`create_host_record`, `create_alias_record`, `create_external_record`); SpinupManaged DNS-create page | passthrough; SpinupManaged reads response `id` (record ID) |
| `GET /v2/dns/{acct}/records?type=…&hint=…` | `server-api` (`host_record_search`, `host_id`, `host_ip`); SpinupManaged DNS-search page | `[i].id`, `[i].properties.addresses` (comma-split into IPs) |
| `GET /v2/dns/{acct}/records/{id}` | SpinupManaged DNS-record detail page | whole entity passed to Blade view |
| `DELETE /v2/dns/{acct}/records/{id}` | `server-api` (`delete_host_record(release_ips=false)`) | passthrough |
| `POST /v2/dns/{acct}/ips` | `server-api` (`assign_ip`); SpinupManaged IP-create page | `server-api` reads `resp.ip`; SpinupManaged reads `resp.id` |
| `DELETE /v2/dns/{acct}/ips/{ip}` | `server-api` (`delete_ip`) | passthrough |
| `GET /v2/dns/{acct}/ips/cidrs` | SpinupManaged subnet-dropdown widget | JSON `{cidr: label}` map (served from local file, not BlueCat) |

Spinup `ui/` has `DNS_API_URL` / `DNS_API_TOKEN` in `docker/.env.deco` but no application code consumes them — dormant placeholders. `spinupmanaged/server-api` mirrors `spinup/server-api`'s 5-route surface; no additional routes.

### What's in scope

Migrate the BlueCat-backed routes to v2 transport. Preserve the wire contract so `server-api` and SpinupManaged keep working without changes. `GET /ips/cidrs` is in scope as a surviving route but **needs no v2 work** — its handler reads a local JSON file, never calls BlueCat.

### What's out of scope (delete, don't migrate)

Zero-caller dns-api-go handlers and their backing services:

- `/zones`, `/zones/{id}`, `GetZonesHandler`, `GetZoneHandler`, **all of `ZoneService` as a public surface** (an internal FQDN→zone-id resolver stays as a private helper for record creation)
- `/networks`, `/networks/{id}`, `GetNetworksHandler`, `GetNetworkHandler`, **all of `NetworkService` as a public surface**
- `/macs`, `/macs/{mac}`, all MAC handlers, `MacAddressService` entirely (this also retires Phase 5E and the unvalidated `macPools` collection)
- `/id/{id}` GET/DELETE, `GetEntityHandler`, `DeleteEntityHandler`
- `/search`, `CustomSearchHandler`
- `/ips/{ip}` GET, `GetIpAddressHandler` (the `GetIpAddress` *service method* stays as a private helper for `DeleteIpAddress`; the *public route* goes)
- `/systeminfo`, `SystemInfoHandler` (Phase 2's transport smoke test; not consumed externally)

### Extensibility

`docs/bluecat-v2-openapi.json` is pinned and authoritative. If a future consumer in `ui/` needs broader coverage (zones, networks, MACs, etc.), the OpenAPI spec is the starting point — but **building it speculatively is not in this plan**. Add coverage when a real caller appears, ideally as PHP-side code in `ui/` rather than expanding dns-api-go.

## Findings from Phase 1 (validated against Yale BAM-test, Spinup Testing block)

- **Login payload requires a `type` field**: `{"type":"UserSession","username":"...","password":"..."}`. Returns **201 Created**. Use `basicAuthenticationCredentials` (already `base64(user:apiToken)`) verbatim as the `Authorization: Basic …` value.
- **Logout**: `PATCH /api/v2/sessions/current` with body `{"state":"LOGGED_OUT"}` and `Content-Type: application/merge-patch+json`.
- **Filter predicate syntax**: function-call style — `name:eq('foo')`, `name:contains('foo')`, `range:startsWith('10.5.')`. Always URL-encode.
- **`resourceRecords` is filterable globally**: `/api/v2/resourceRecords?filter=absoluteName:eq('…') and type:eq('HostRecord')` works without a zone ID. **Zone-ID resolution from FQDN is needed only for creation** (`POST /api/v2/zones/{zoneId}/resourceRecords`).
- **`HostRecord.addresses` shape**: array of objects `{id, type, address}`. Phase 1 harness asserts each entry has non-empty `address`.
- **Zone name collisions exist within a view**: always filter by `type:eq('Zone')` (or the specific kind) — a view can contain both a `Zone` and an `ExternalHostsZone` with the same name.
- **Config caches `ConfigurationId` and `ViewId`** (wired through `internal/common/config.go` and the deco template).
- **OpenAPI spec pinned**: `docs/bluecat-v2-openapi.json` — authoritative for field names, status codes, filter predicates.

## Findings from Phase 2 (validated against Yale BAM-test, live tests in `internal/api/v2_live_test.go`)

- **`basicAuthenticationCredentials` is used verbatim** as the `Authorization: Basic …` header value — do not re-encode.
- **v2 404 envelope**: `{"status":404,"reason":"Not Found","code":"ResourceNotFound","message":"…","detail":"…"}`. `api.IsNotFound(err)` covers the common case.
- **`/api/v2/settings` requires a filter** (`data[]` is a `oneOf` of 12 settings subtypes).
- **Body retry bug was real**: pre-v2 `MakeRequest` passed the original `io.Reader` to a 401 retry. Fixed by buffering body to `[]byte` once and building a fresh `bytes.NewReader` per attempt.
- **Live integration tests are valuable beyond mocks** — surfaced the real 404 envelope, validated `basicAuthenticationCredentials` semantics, confirmed PATCH-based logout invalidates the server-side session. Standing pattern: every subsequent phase adds skip-guarded `TestV2Live_*` tests alongside unit tests.

## Design Decisions

### 1. `ServerInterface` and `MakeRequest` signature unchanged
`MakeRequest(method, route, queryParam string, body io.Reader) ([]byte, error)` is sufficiently general. v2 routes go in `route`, v2 filters in `queryParam`. No new interface methods. `MockServer` unchanged.

### 2. `MakeRequest` internals updated for v2 ✅ (Phase 2)
- `Authorization: Basic {credentials}` + `Accept: application/hal+json`
- Accept 2xx range; return `[]byte{}` on 204
- Body buffered as `[]byte` before first attempt; fresh `bytes.NewReader` per attempt
- Bounded retry loop (max 2 attempts); persistent 401 surfaces as `*BluecatAPIError{StatusCode:401}`
- `getToken()`'s error no longer dropped via shadowed `:=`

### 3. `BluecatAPIError` for distinguishing 404 from 500 ✅ (Phase 2)
`*BluecatAPIError{StatusCode, Body}` with package-level `IsNotFound(err)` via `errors.As`. Use `api.IsNotFound(err)` instead of type-asserting at every site.

### 4. Thin v2 models — no generic dispatcher
Two response structs (`V2HostRecord`, `V2Address`) with explicit `json` tags. No `BluecatV2Entity`, no `HALLinks`, no `ExtractIDFromHref`, no `typeToCollection` mapping. The `Entity{ID, Name, Type, Properties}` shape stays internal for now to minimize churn through `server-api`'s wire contract — `properties.addresses` (comma-joined string) is the only output key any consumer reads.

### 5. Per-call request structs for v2 POST/PUT bodies
No shared `ToV2JSON()`. Request shapes vary between create-record / assign-ip / etc. Two or three small structs, one per call site.

### 6. Logout is best-effort, deferred from graceful shutdown
`logout()` (PATCH `/api/v2/sessions/current`) lives on `*server`, clears local state unconditionally, swallows server errors. Wiring into `srv.Shutdown()` is deferred to Phase 7 (real scope: changes the server's blocking contract).

---

## Implementation Phases

### Phase 1: v2 Endpoint Validation Script ✅ **Complete** (commit `93b183e`)
Skip-guarded harness in `internal/services/v2_validation_test.go`:
- `TestV2_Auth` — login + `/sessions/current` round-trip
- `TestV2_DiscoverFixtures` — discovery walk: configurations → views → blocks → networks → zones → resourceRecords, plus global `resourceRecords` filter, `networks/{id}/addresses`, address filter by value, `configurations/{id}/macAddresses`, pagination shape

`docs/bluecat-v2-openapi.json` pinned. `internal/common/config.go` extended with `ConfigurationId` + `ViewId`. Creds source order: `BLUECAT_V2_URL` env → `BLUECAT_V2_CONFIG` env → `../../docker/config.json`. Read-only, scoped to Spinup Testing block.

### Phase 2: Authentication Migration ✅ **Complete**

- **`internal/api/server_errors.go`** — `BluecatAPIError{StatusCode, Body}` typed error; package-level `IsNotFound(err)` via `errors.As`.
- **`internal/api/server.go`** — added `sessionID int` for `logout()`.
- **`internal/api/helpers.go`** — `generateAuthToken()` rewritten for v2; new `logout()` (PATCH, `merge-patch+json`, `{"state":"LOGGED_OUT"}`); new `bluecatHTTPClient()` shared by login/logout/`MakeRequest`. `MakeRequest()` rewritten: bounded retry, buffered body, `Basic` auth, HAL+JSON Accept, 2xx range, `[]byte{}` on 204, `*BluecatAPIError` on non-2xx.
- **`internal/api/handlers.go`** — `SystemInfoHandler` migrated (will be deleted in Phase 7; served as transport smoke test).

Tests: `server_errors_test.go`, `helpers_test.go` (15 cases via `httptest`), `handlers_test.go`, `v2_live_test.go` (6 skip-guarded live tests against BAM-test). 401 rotation, logout, and `SystemInfoHandler` end-to-end all verified live.

**Deferred to Phase 7**: graceful shutdown + signal handling + `defer s.logout()`.

### Phase 3: Scope Contraction

**Goal**: Get the surface narrow before writing any v2 logic. Delete the zero-caller handlers, services, and routes identified in the [consumer audit](#consumer-audit-may-2026).

Deletes the v1-shaped surface area we're not migrating:

- `internal/api/routes.go` — drop `/search`, `/id/{id}`, `/zones`, `/zones/{id}`, `/networks`, `/networks/{id}`, `/macs`, `/macs/{mac}`, `/ips/{ip}` GET, `/systeminfo`. Final route table: `/ping`, `/version`, `/metrics`, `/`, `POST /records`, `GET /records`, `GET /records/{id}`, `DELETE /records/{id}`, `POST /ips`, `DELETE /ips/{ip}`, `GET /ips/cidrs`.
- `internal/api/entity_handlers.go`, `zone_handlers.go`, `network_handlers.go`, `mac_address_handlers.go` (and tests) — delete. (No `search_handlers.go` exists; `CustomSearchHandler` lives in `entity_handlers.go`.)
- `internal/api/handlers.go` — keep `Ping`, `Version`, `Home`; remove `SystemInfoHandler` (and its v1→v2-migrated body) plus tests.
- `internal/services/zone_service.go`, `network_service.go`, `mac_address_service.go` — delete as public services. Their *private helpers* (FQDN→zone-id resolution for record creation, CIDR→parent-network-id for IP assignment) move into the service that needs them, scoped down.
- `internal/api/ip_address_handlers.go` — remove `GetIpAddressHandler`; keep `AssignIpAddressHandler`, `DeleteIpAddressHandler`, `GetCIDRHandler` (local-file read for SpinupManaged), and `parentIdFromCidr` (the last one becomes a candidate for v2 simplification in Phase 5B).
- Cross-package: any imports/types this exposes (e.g., `types.MACADDRESS`, mac-related helpers in `services/helpers.go`) get cleaned up alongside.

**Verification**: `go build ./...` clean. `go test ./...` passes (existing tests for the deleted code go with it). Skip-guarded live tests still pass — they only exercise auth, sessions, and `SystemInfoHandler`, the last of which is replaced in Phase 6 with a record-search snapshot.

**Single commit, no v2 logic mixed in.** Reviewers see the cleanup separately from the migration.

### Phase 4: v2 Models + Thin Helpers ✅ **Complete**

**Goal**: minimum-viable v2 response decoding + shared HTTP plumbing for the two services that survive.

**Deviation from original plan**: the v1 helpers in `internal/services/helpers.go` (`GetEntityByID`, `DeleteEntityByID`, `GetEntities`, `GetEntityByName`, `searchObjectByTypes`, `GetParentID`, `UpdateEntity`, `GetEntitiesByHintHelper`) are still wired into `record_service.go` and `ip_address_service.go`. Deleting them here would break the build before Phase 5 has a chance to migrate the services. Phase 4 is therefore **purely additive**; the v1 helpers retire as part of Phase 5 alongside their last callers.

**`internal/models/bluecat_v2.go`** (new):
```go
type V2HostRecord struct {
    ID           int    `json:"id"`
    Type         string `json:"type"`
    Name         string `json:"name"`
    AbsoluteName string `json:"absoluteName"`
    Addresses    []struct {
        Address string `json:"address"`
    } `json:"addresses"`
}

type V2Address struct {
    ID      int    `json:"id"`
    Type    string `json:"type"`
    Address string `json:"address"`
}

type V2Collection[T any] struct {
    Count int `json:"count"`
    Data  []T `json:"data"`
}
```

`V2HostRecord.ToEntity()` returns an `Entity{ID, Name, Type, Properties{"absoluteName": …, "addresses": "10.5.0.1,10.5.0.2"}}` — only the keys `server-api` actually reads. Other v2 fields are decoded but not surfaced to the wire. Empty address strings inside `addresses[]` are skipped so the joined value never has trailing/embedded commas.

Decision deferred to implementation: whether we keep funnelling through `Entity` or let record handlers serialize a purpose-built `recordResponse` struct directly. The wire shape `server-api` parses is what matters; the internal representation is a refactor we can make once the migration lands.

**`internal/services/helpers.go`** — additive edits:
- `GetConfigID(server) (int, error)` rewritten to v2: returns cached `s.bluecat.configurationId` via the new `ServerInterface.ConfigurationID() (int, bool)` method; falls back to `GET /api/v2/configurations?limit=1` only if unset. No network call in the steady state.
- New `buildFilter(predicates ...string) string` — joins with ` and ` and `url.QueryEscape`s the whole value. Pure string helper; ~10 lines.
- The v1 helpers (`GetParentID`, `GetEntityByID`, `GetEntityByName`, `GetEntities`, `GetEntitiesByHintHelper`, `searchObjectByTypes`, `UpdateEntity`, `DeleteEntityByID`) stay in place for now — `record_service.go` and `ip_address_service.go` still call them. They're deleted in Phase 5 as each service flips to v2.

**Wiring**: `internal/common/config.go`'s `Bluecat.ConfigurationId` (string) is parsed to int during `NewServer` and stashed on the bluecat struct; non-integer values are logged and ignored (falls back to the v2 lookup). `MockServer` gains a `ConfigurationIDFunc` hook with a `0, false` default.

**Tests**: `internal/models/bluecat_v2_test.go` with one v2 fixture per struct (representative shapes in `internal/models/testdata/`, not BAM captures — the live discovery walk in `internal/services/v2_validation_test.go` already pins the shape against BAM-test). Tests cover unmarshal, `ToEntity`, the empty-address skip, and `V2Collection[T]` for both records and addresses.

### Phase 5: Service Migration

**Standing pattern**: each sub-phase adds skip-guarded `internal/services/{service}_v2_live_test.go` exercising **read-only** operations against BAM-test (Spinup Testing block). Mutation paths (create record, assign+delete IP) are exercised manually during development but not checked into the committed suite — the Spinup Testing `/26` exists in both BAM-test and BAM-production, and a misconfigured `docker/config.json` could silently target production. See §Risks #4.

**Cleanup carried into Phase 5**: each sub-phase deletes the v1 helpers that lose their last caller as the service migrates. By the end of 5B, `internal/services/helpers.go` is left with just `GetConfigID` and `buildFilter` as originally targeted.

#### 5A: RecordService (`internal/services/record_service.go`)

- `GetRecord(id)` → `GET /api/v2/resourceRecords/{id}`. Decode into `V2HostRecord` → `ToEntity()`. On 404, return `*ErrEntityNotFound`.
- `getHostOrAliasRecordsByHint(hint)` → `GET /api/v2/resourceRecords?filter=absoluteName:contains('{hint}') and type:eq('{recordType}')`. Decode `V2Collection[V2HostRecord]`. **No zone-ID resolution required for lookups** (Phase 1 finding).
- `CreateRecord` — three flavors (`HostRecord`, `AliasRecord`, `ExternalHostRecord`) → `POST /api/v2/zones/{zoneId}/resourceRecords` with JSON body discriminated by `type` field. **Creation is the one path that still needs zone-ID resolution from FQDN.**
- New private helper `resolveZoneIDFromFQDN(fqdn string) (int, error)` — walks FQDN labels (`host.sub.example.com` → try `sub.example.com`, then `example.com`) calling `GET /api/v2/views/{viewId}/zones?filter=name:eq('…') and type:eq('Zone')`. Always include `type:eq('Zone')` to avoid the `ExternalHostsZone` collision.
- `DeleteRecord(id)` → `DELETE /api/v2/resourceRecords/{id}`. Expect 204.

Record-create response decodes the full entity (v2 returns it inline) so no follow-up `GetEntity` round-trip.

#### 5B: IpAddressService (`internal/services/ip_address_service.go`)

- `GetIpAddress(addr)` *(now private; the public route is gone)* → `GET /api/v2/addresses?filter=address:eq('{addr}')`. Decode `V2Collection[V2Address]`. Used internally by `DeleteIpAddress` to resolve IP→ID.
- `DeleteIpAddress(addr)` → resolve via `GetIpAddress`, then `DELETE /api/v2/addresses/{id}`. Expect 204.
- `AssignIpAddress` → `POST /api/v2/networks/{parentId}/addresses` with JSON body (`action`, `macAddress`, `hostInfo`, properties).
- `parentIdFromCidr` — replace the "probe 10 IPs and walk `_links.up`" v1 workaround with `GET /api/v2/networks?filter=range:eq('{cidr}')`. Decode the first hit's ID. Falls back to the probe-walk only if v2 filter on `range` proves unreliable in BAM-test (validate during 5B development).

### Phase 6: Contract Snapshot Test

The bulk of test work landed inside Phases 2/4/5 (unit + skip-guarded live tests per phase). What remains:

- **Wire-contract snapshot**: one test that hits `GET /v2/dns/{acct}/records?type=HostRecord&hint=^…$` through the full handler stack against BAM-test, then asserts the JSON response shape matches what `server-api/lib/dns/proteus.rb` parses — specifically `[i].id` (int) and `[i].properties.addresses` (comma-separated IP string). This is the only test that protects the cross-repo contract.
- **Smoke check**: `BLUECAT_V2_CONFIG=… go test ./... -run V2Live` runs all skip-guarded live tests across Phases 2/4/5 in one pass. Add a short README section pointing developers at this command and the no-mutation-tests policy.
- Audit `MockServer.MakeRequestFunc` callbacks — any v1 pipe-delimited fixtures still in tests get replaced with v2 JSON. (Most will have been deleted in Phase 3 alongside the handlers they tested.)

### Phase 7: Cleanup

- Delete `internal/models/bluecat_entity.go` (and test file) — no surviving caller.
- Delete `Entity.ToBluecatJSON()` from `internal/models/entity.go`.
- Decide whether `Entity{ID, Name, Type, Properties}` still pays its keep, or whether record/ip handlers should serialize directly from `V2HostRecord`/`V2Address`. If the latter is cleaner, delete `Entity` too. (Defer this decision until Phase 5 lands — the right call may be obvious once the two surviving services are migrated.)
- `common.ConvertToSeparatedString` / `ConvertToMap` — verify still needed (handlers parse pipe-delimited input from `server-api` for `properties` on `POST /ips`; likely stays until `server-api` is retired).
- **Wire `logout()` into graceful shutdown**: add SIGTERM/SIGINT handling in `main.go`, switch `NewServer` to non-blocking start + `srv.Shutdown()`, `defer s.logout()` on the way out.

---

## Files Modified (by phase)

| Phase | Files | Type |
|-------|-------|------|
| 1 | `internal/services/v2_validation_test.go`, `docs/bluecat-v2-openapi.json` | New |
| 1 | `internal/common/config.go`, `docker/config.deco.json`, `docker/Dockerfile` | Edit |
| 2 | `internal/api/server.go`, `internal/api/helpers.go`, `internal/api/handlers.go` | Edit |
| 2 | `internal/api/server_errors.go`, `internal/api/v2_live_test.go` | New |
| 2 | `internal/api/server_errors_test.go`, `internal/api/helpers_test.go`, `internal/api/handlers_test.go` | New / Edit |
| 3 | `internal/api/routes.go`, `internal/api/handlers.go` | Edit (delete routes/handlers) |
| 3 | `internal/api/entity_handlers.go`, `zone_handlers.go`, `network_handlers.go`, `mac_address_handlers.go`, `search_handlers.go` (+ their `_test.go`) | Delete |
| 3 | `internal/services/zone_service.go`, `network_service.go`, `mac_address_service.go` (+ their `_test.go`) | Delete |
| 3 | `internal/api/ip_address_handlers.go` | Edit (drop GetIpAddress/GetCIDR routes) |
| 3 | `internal/services/helpers.go` | Edit (drop unused helpers, alongside cross-package callers) |
| 4 | `internal/models/bluecat_v2.go`, `internal/models/bluecat_v2_test.go`, `internal/models/testdata/v2_*.json` | New |
| 4 | `internal/services/helpers.go` | Edit (v2 `GetConfigID` + `buildFilter`; v1 helpers stay until Phase 5) |
| 4 | `internal/interfaces/server_interface.go`, `internal/api/server.go`, `internal/mocks/mock_server.go` | Edit (add `ConfigurationID() (int, bool)`) |
| 5A | `internal/services/record_service.go` (+ `_v2_live_test.go`) | Edit (major) / New |
| 5B | `internal/services/ip_address_service.go` (+ `_v2_live_test.go`) | Edit / New |
| 6 | `internal/api/v2_contract_test.go` (new), various `_test.go` mock fixtures | New / Edit |
| 7 | `internal/models/bluecat_entity.go` (+ test) | Delete |
| 7 | `internal/models/entity.go` (drop `ToBluecatJSON`; possibly delete entirely) | Edit / Delete |
| 7 | `main.go`, `internal/api/server.go` (graceful shutdown + `defer logout`) | Edit |

## Key Risks

1. **Wire contract to `server-api`**: the GET `/records` response must preserve `[i].id` and `[i].properties.addresses` as a comma-separated string. Phase 6's contract snapshot is the single test that protects this. Highest blast radius if it drifts — server-api silently degrades.
2. **`range:eq('{cidr}')` reliability for `parentIdFromCidr`**: this is the one v2 simplification in Phase 5B that hasn't been live-validated yet. If the filter doesn't behave as expected, fall back to the v1 probe-walk pattern (translated to v2 calls) rather than blocking the phase.
3. **`server-api` propagates `err.Error()` to clients in places**. Phase 2's switch to `*BluecatAPIError` changes the error string shape. Worth a focused grep across `server-api/lib/dns/` (smaller surface than `ui/`) before Phase 5 ships, to flag any string-matching on v1 error formats.
4. **Mutating tests against shared BAM-test**: the Spinup Testing `10.5.0.0/26` exists in both BAM-test (10.16.8.40) and BAM-production with the same CIDR and zone names. A live mutation test has no way to tell them apart at the wire level — a swapped `docker/config.json` or stale env var would silently allocate real IPs and create real DNS records in production. Mutation tests were exercised manually during 5A/5B development and then removed from the committed suite before merge. If mutation coverage is needed in the future, run it against a known-safe sandbox out-of-band.
5. ~~**Filter predicate syntax**~~, ~~**body retry bug**~~, ~~**`basicAuthenticationCredentials` encoding**~~, ~~**MACPool collection unvalidated**~~ — Phase 5E retired (no consumer), MAC paths deleted in Phase 3, so the macPool risk goes with it.

## Verification

Each phase ships with both unit tests (`httptest` / mocks) **and** skip-guarded live tests against BAM-test. Live tests gated on creds being reachable so the offline suite stays green.

1. **Phase 1**: ✅ Validation harness against BAM-test (`v2_validation_test.go`).
2. **Phase 2**: ✅ `go test ./internal/api/...` offline + `BLUECAT_V2_CONFIG=… go test ./internal/api/ -run V2Live`.
3. **Phase 3**: `go build ./...` and `go test ./...` both green after the deletions. No new tests required; the deleted code's tests go with it.
4. **Phase 4**: ✅ `go build ./...` and `go test ./...` green. `go test ./internal/models/ -run V2 -v` runs 7 cases (unmarshal + ToEntity + collection) against captured v2 fixtures.
5. **Phase 5**: Per-service live tests pass read-only against BAM-test.
6. **Phase 6**: Wire contract snapshot passes against BAM-test; `go test ./... -run V2Live` is a single command that runs every live test in the repo.
7. **End-to-end**: Deploy to test environment, exercise `server-api` zone-add flows (host record create + delete, IP assign + release) against the new dns-api-go build.

## Open Questions

- ~~Is v2 BlueCat test instance accessible?~~ **Resolved** — Yale BAM-test reachable.
- ~~What BAM version are we targeting?~~ **Resolved** — `25.1.1-1157.GA.bcn` (confirmed via Phase 2 `SystemInfoHandler`).
- ~~Where does this plan live?~~ `dns-api-go/docs/v2-migration-plan.md`.
- ~~Maintain v1/v2 coexistence during rollout?~~ **No** — hard cutover, single deployment unit flips atomically. Decision locked.
- ~~Generic vs. per-type parsing?~~ **Resolved** — per-type. Only two response structs (`V2HostRecord`, `V2Address`) survive the scope cut, so the generic-vs-specific debate is moot.
- **Open** — Does `server-api` parse any v1-specific error string formats? Grep `server-api/lib/dns/` before Phase 5 ships.
- **Open** — After Phase 5 lands, should the internal `Entity` type be retired in favor of v2 structs serializing straight to wire? Defer the call until the code is in front of us.
