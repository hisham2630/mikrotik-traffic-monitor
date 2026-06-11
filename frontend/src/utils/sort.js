/** Natural (numeric-aware) string compare — ether2 before ether11. */
export function compareNatural(a, b) {
  return String(a ?? '').localeCompare(String(b ?? ''), undefined, {
    numeric: true,
    sensitivity: 'base',
  });
}

/** Return a new array sorted by interface_name (or custom getter). */
export function sortByName(items, getName = (item) => item.interface_name) {
  return [...items].sort((a, b) => compareNatural(getName(a), getName(b)));
}

/** Sort discovered MikroTik interfaces ({ name, type }). */
export function sortDiscovered(items) {
  return [...items].sort((a, b) => {
    const typeCmp = compareNatural(a.type, b.type);
    if (typeCmp !== 0) return typeCmp;
    return compareNatural(a.name, b.name);
  });
}

/** Sort grouped discovered map values in place; returns the same object. */
export function sortDiscoveredGrouped(grouped) {
  if (!grouped) return grouped;
  for (const type of Object.keys(grouped)) {
    grouped[type] = sortByName(grouped[type] || [], (item) => item.name);
  }
  return grouped;
}
