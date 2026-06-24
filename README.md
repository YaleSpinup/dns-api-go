# dns-api-go

This API provides simple restful API access to a service.

## Endpoints

```
GET /v1/test/ping
GET /v1/test/version
GET /v1/test/metrics
```

## Authentication

Authentication is accomplished via an encrypted pre-shared key passed via the `X-Auth-Token` header.

## Testing

Unit tests run offline against `MockServer` fixtures:

```
go test ./...
```

A second tier of integration tests (`V2Live*`) hits a real BlueCat v2 BAM
instance to validate the auth chain, filter syntax, and wire contract with
`server-api`. They skip when credentials are unreachable, so the offline
suite stays green either way.

Credential sources, in priority order:

1. `BLUECAT_V2_URL` + `BLUECAT_V2_USER` + `BLUECAT_V2_PASS` env vars
2. `BLUECAT_V2_CONFIG` env var pointing at a `config.json`
3. `docker/config.json` (the local-dev config)

Run all live tests:

```
go test ./... -run V2Live -v
```

The committed `V2Live*` tests are read-only — they query BAM but never
create or delete records. The Spinup Testing `10.5.0.0/26` CIDR exists
in both BAM-test and BAM-production with no way for a test to tell them
apart at the wire level, so an accidental run against production would
allocate real IPs and create real DNS records. If you need to validate
mutation paths (assign/delete IP, create/delete record), run those
manually against a known-safe sandbox rather than checking write tests
into the suite.

The wire-contract snapshot `TestV2Live_RecordsWireContract` is the single
test that protects the cross-repo contract with
`server-api/lib/dns/proteus.rb`. If it fails, `server-api` will break in
production — investigate before merging.

## License

GNU Affero General Public License v3.0 (GNU AGPLv3)  
Copyright © 2023 Yale University
