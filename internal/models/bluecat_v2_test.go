package models

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func TestV2HostRecord_Unmarshal(t *testing.T) {
	var r V2HostRecord
	if err := json.Unmarshal(readFixture(t, "v2_host_record.json"), &r); err != nil {
		t.Fatalf("unmarshal v2_host_record.json: %v", err)
	}
	if r.ID != 100913 {
		t.Errorf("ID = %d, want 100913", r.ID)
	}
	if r.Type != "HostRecord" {
		t.Errorf("Type = %q, want HostRecord", r.Type)
	}
	if r.Name != "example" {
		t.Errorf("Name = %q, want example", r.Name)
	}
	if r.AbsoluteName != "example.spinuptest.internal" {
		t.Errorf("AbsoluteName = %q, want example.spinuptest.internal", r.AbsoluteName)
	}
	if len(r.Addresses) != 2 {
		t.Fatalf("Addresses len = %d, want 2", len(r.Addresses))
	}
	if r.Addresses[0].Address != "10.5.0.10" || r.Addresses[1].Address != "10.5.0.11" {
		t.Errorf("Addresses = %+v, want [10.5.0.10, 10.5.0.11]", r.Addresses)
	}
}

func TestV2HostRecord_ToEntity(t *testing.T) {
	var r V2HostRecord
	if err := json.Unmarshal(readFixture(t, "v2_host_record.json"), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	e := r.ToEntity()
	if e.ID != 100913 {
		t.Errorf("ID = %d, want 100913", e.ID)
	}
	if e.Name != "example" {
		t.Errorf("Name = %q, want example", e.Name)
	}
	if e.Type != "HostRecord" {
		t.Errorf("Type = %q, want HostRecord", e.Type)
	}
	if got := e.Properties["absoluteName"]; got != "example.spinuptest.internal" {
		t.Errorf("properties.absoluteName = %q, want example.spinuptest.internal", got)
	}
	if got := e.Properties["addresses"]; got != "10.5.0.10,10.5.0.11" {
		t.Errorf("properties.addresses = %q, want 10.5.0.10,10.5.0.11", got)
	}
}

func TestV2HostRecord_ToEntity_NoAddresses(t *testing.T) {
	r := V2HostRecord{ID: 1, Name: "alias", Type: "AliasRecord", AbsoluteName: "alias.example.com"}
	e := r.ToEntity()
	if e.Properties["addresses"] != "" {
		t.Errorf("addresses = %q, want empty string for record with no addresses", e.Properties["addresses"])
	}
	if e.Properties["absoluteName"] != "alias.example.com" {
		t.Errorf("absoluteName not preserved")
	}
}

func TestV2HostRecord_ToEntity_SkipsEmptyAddress(t *testing.T) {
	r := V2HostRecord{
		ID: 1, Name: "n", Type: "HostRecord", AbsoluteName: "n.x",
		Addresses: []struct {
			Address string `json:"address"`
		}{
			{Address: "10.5.0.1"},
			{Address: ""},
			{Address: "10.5.0.2"},
		},
	}
	if got := r.ToEntity().Properties["addresses"]; got != "10.5.0.1,10.5.0.2" {
		t.Errorf("addresses = %q, want 10.5.0.1,10.5.0.2 (empty entries skipped)", got)
	}
}

func TestV2Address_Unmarshal(t *testing.T) {
	var a V2Address
	if err := json.Unmarshal(readFixture(t, "v2_address.json"), &a); err != nil {
		t.Fatalf("unmarshal v2_address.json: %v", err)
	}
	if a.ID != 100914 {
		t.Errorf("ID = %d, want 100914", a.ID)
	}
	if a.Type != "IP4Address" {
		t.Errorf("Type = %q, want IP4Address", a.Type)
	}
	if a.Address != "10.5.0.10" {
		t.Errorf("Address = %q, want 10.5.0.10", a.Address)
	}
}

func TestV2Collection_HostRecord(t *testing.T) {
	var c V2Collection[V2HostRecord]
	if err := json.Unmarshal(readFixture(t, "v2_host_record_collection.json"), &c); err != nil {
		t.Fatalf("unmarshal collection: %v", err)
	}
	if c.Count != 2 {
		t.Errorf("Count = %d, want 2", c.Count)
	}
	if len(c.Data) != 2 {
		t.Fatalf("Data len = %d, want 2", len(c.Data))
	}
	if c.Data[0].ID != 100913 || c.Data[1].ID != 100923 {
		t.Errorf("IDs = [%d, %d], want [100913, 100923]", c.Data[0].ID, c.Data[1].ID)
	}
	if got := c.Data[1].ToEntity().Properties["addresses"]; got != "10.5.0.20,10.5.0.21" {
		t.Errorf("second record addresses = %q, want 10.5.0.20,10.5.0.21", got)
	}
}

func TestV2Collection_Address(t *testing.T) {
	var c V2Collection[V2Address]
	if err := json.Unmarshal(readFixture(t, "v2_address_collection.json"), &c); err != nil {
		t.Fatalf("unmarshal collection: %v", err)
	}
	if c.Count != 1 || len(c.Data) != 1 {
		t.Fatalf("collection shape: count=%d data=%d", c.Count, len(c.Data))
	}
	if c.Data[0].Address != "10.5.0.10" {
		t.Errorf("Address = %q, want 10.5.0.10", c.Data[0].Address)
	}
}
