package models

import "strings"

// V2HostRecord is the BlueCat v2 representation of a HostRecord resource.
// Only the fields the dns-api-go consumers read are decoded; v2 returns more.
type V2HostRecord struct {
	ID           int    `json:"id"`
	Type         string `json:"type"`
	Name         string `json:"name"`
	AbsoluteName string `json:"absoluteName"`
	Addresses    []struct {
		Address string `json:"address"`
	} `json:"addresses"`
}

// V2Address is the BlueCat v2 representation of an IP4Address resource.
// v2 returns Type as "IPv4Address" (vs v1's "IP4Address"); we pass that
// through to consumers verbatim — the wire-contract audit shows neither
// server-api nor SpinupManaged parse the type field for ip endpoints.
type V2Address struct {
	ID      int    `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Address string `json:"address"`
	State   string `json:"state"`
}

// ToEntity converts a V2Address into the legacy Entity shape AssignIpAddress
// and GetIpAddress return to handlers. Properties["address"] is the IP
// string — that's the only properties key the AssignIpAddressHandler reads
// (server-api consumes `resp.ip` derived from it).
func (a V2Address) ToEntity() Entity {
	return Entity{
		ID:   a.ID,
		Name: a.Name,
		Type: a.Type,
		Properties: map[string]string{
			"address": a.Address,
		},
	}
}

// V2Collection is the HAL+JSON envelope for v2 list responses.
type V2Collection[T any] struct {
	Count int `json:"count"`
	Data  []T `json:"data"`
}

// ToEntity converts a V2HostRecord into the legacy Entity shape the wire
// contract with server-api depends on. Only `absoluteName` and a
// comma-joined `addresses` string are surfaced as properties — the two keys
// server-api parses today.
func (r V2HostRecord) ToEntity() Entity {
	addrs := make([]string, 0, len(r.Addresses))
	for _, a := range r.Addresses {
		if a.Address == "" {
			continue
		}
		addrs = append(addrs, a.Address)
	}
	return Entity{
		ID:   r.ID,
		Name: r.Name,
		Type: r.Type,
		Properties: map[string]string{
			"absoluteName": r.AbsoluteName,
			"addresses":    strings.Join(addrs, ","),
		},
	}
}
