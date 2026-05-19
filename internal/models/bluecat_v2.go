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
type V2Address struct {
	ID      int    `json:"id"`
	Type    string `json:"type"`
	Address string `json:"address"`
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
