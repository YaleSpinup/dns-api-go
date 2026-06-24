package models

// Entity is the JSON wire shape dns-api-go returns to consumers
// (server-api, SpinupManaged). V2HostRecord.ToEntity and
// V2Address.ToEntity are the conversion boundaries from BlueCat's v2
// representation to this shape; the wire-contract test in
// internal/api/v2_contract_test.go pins the JSON layout against
// server-api/lib/dns/proteus.rb.
type Entity struct {
	ID         int               `json:"id"`
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	Properties map[string]string `json:"properties"`
}
