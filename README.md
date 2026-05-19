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

Mutation tests (create/delete real BAM entities) are gated behind a second
env var so CI and casual runs don't write to the shared instance:

```
BLUECAT_V2_ALLOW_MUTATIONS=1 go test ./... -run V2Live -v
```

The wire-contract snapshot `TestV2Live_RecordsWireContract` is the single
test that protects the cross-repo contract with
`server-api/lib/dns/proteus.rb`. If it fails, `server-api` will break in
production — investigate before merging.

## License

GNU Affero General Public License v3.0 (GNU AGPLv3)  
Copyright © 2023 Yale University
