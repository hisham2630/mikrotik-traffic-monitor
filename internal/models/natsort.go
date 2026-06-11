package models

import (
	"sort"
	"strconv"
)

// CompareNatural compares strings with numeric-aware ordering.
// "ether2" sorts before "ether11".
func CompareNatural(a, b string) int {
	ai, bi := 0, 0
	for ai < len(a) && bi < len(b) {
		ca, cb := a[ai], b[bi]
		da := ca >= '0' && ca <= '9'
		db := cb >= '0' && cb <= '9'
		if da && db {
			ja, ka := ai, ai
			for ka < len(a) && a[ka] >= '0' && a[ka] <= '9' {
				ka++
			}
			jb, kb := bi, bi
			for kb < len(b) && b[kb] >= '0' && b[kb] <= '9' {
				kb++
			}
			na, _ := strconv.ParseUint(a[ja:ka], 10, 64)
			nb, _ := strconv.ParseUint(b[jb:kb], 10, 64)
			if na != nb {
				if na < nb {
					return -1
				}
				return 1
			}
			if ka-ja != kb-jb {
				if ka-ja < kb-jb {
					return -1
				}
				return 1
			}
			ai, bi = ka, kb
			continue
		}
		if ca != cb {
			if ca < cb {
				return -1
			}
			return 1
		}
		ai++
		bi++
	}
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return 0
}

func SortMonitoredInterfaces(list []MonitoredInterface) {
	sort.Slice(list, func(i, j int) bool {
		if list[i].InterfaceType != list[j].InterfaceType {
			return list[i].InterfaceType < list[j].InterfaceType
		}
		return CompareNatural(list[i].InterfaceName, list[j].InterfaceName) < 0
	})
}

func SortDiscoveredInterfaces(list []DiscoveredInterface) {
	sort.Slice(list, func(i, j int) bool {
		if list[i].Type != list[j].Type {
			return list[i].Type < list[j].Type
		}
		return CompareNatural(list[i].Name, list[j].Name) < 0
	})
}

func SortDiscoveredGrouped(grouped map[string][]DiscoveredInterface) {
	for typ := range grouped {
		slice := grouped[typ]
		sort.Slice(slice, func(i, j int) bool {
			return CompareNatural(slice[i].Name, slice[j].Name) < 0
		})
		grouped[typ] = slice
	}
}
