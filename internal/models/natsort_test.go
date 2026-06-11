package models

import "testing"

func TestCompareNatural(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"ether1", "ether2", -1},
		{"ether2", "ether11", -1},
		{"ether11", "ether12", -1},
		{"ether2", "ether11", -1},
		{"ether10", "ether2", 1},
		{"vlan10", "vlan2", 1},
		{"", "", 0},
		{"a", "a", 0},
		{"a", "b", -1},
		{"port2", "port10", -1},
	}
	for _, tc := range tests {
		got := CompareNatural(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("CompareNatural(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestSortMonitoredInterfaces(t *testing.T) {
	list := []MonitoredInterface{
		{InterfaceName: "ether11", InterfaceType: "ethernet"},
		{InterfaceName: "ether2", InterfaceType: "ethernet"},
		{InterfaceName: "ether1", InterfaceType: "ethernet"},
		{InterfaceName: "vlan10", InterfaceType: "vlan"},
		{InterfaceName: "vlan2", InterfaceType: "vlan"},
	}
	SortMonitoredInterfaces(list)
	want := []string{"ether1", "ether2", "ether11", "vlan2", "vlan10"}
	for i, name := range want {
		if list[i].InterfaceName != name {
			t.Fatalf("index %d: got %q, want %q", i, list[i].InterfaceName, name)
		}
	}
}
