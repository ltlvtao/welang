package deps

import "sort"

// Conflict is resolution's failure: E2001 when a declared name answers to
// no package in any source, E2002 when the maximum-of-floors pick violates
// a ceiling or is a version no source holds. Which fields carry depends on
// the form; the caller renders the diagnostic from them.
type Conflict struct {
	Code   string     // "E2001" | "E2002"
	Name   string     // the package at fault
	Pick   Version    // the pick that faulted
	Con    Constraint // the violated ceiling constraint
	Chain  string     // that constraint's dependent chain, root-anchored ("root" or "root -> a@1.0.0")
	Raised Constraint // the constraint whose floor raised the pick
	Raiser string     // who imposed it ("a@1.0.0")
	Demand Constraint // the floor demand no source holds
	Dep    string     // who demanded it ("root" or "a@1.0.0")
}

// edge is one constraint on a package, with the dependent that imposed it
// and that dependent's own chain back to root.
type edge struct {
	con   Constraint
	dep   string // "root" or "pkg@1.0.0"
	chain string // the dependent's chain, inclusive ("root -> a@1.0.0")
}

// Resolve runs chapter 22 R3's algorithm: every constraint contributes a
// floor, each package picks the maximum of its floors, and a new pick's
// manifest contributes new constraints — iterated to a fixed point. Floors
// only rise and the graph only widens, so the fixed point arrives; picks
// are then a function of the manifests alone (order-independent and
// registry-state-independent — a newly published version changes nothing
// until some floor demands it). A pick no source holds is E2002 at once; a
// name no source answers is E2001; after the fixed point, a pick violating
// any ceiling is E2002, reported for the first such package in name order.
func Resolve(root Table, reg Registry) (map[string]Version, *Conflict) {
	edges := map[string][]edge{}
	for _, name := range sortedNames(root) {
		edges[name] = append(edges[name], edge{con: root[name], dep: "root", chain: "root"})
	}
	picks := map[string]Version{}
	maxFloor := func(name string) (Version, edge) {
		var best edge
		var floor Version
		first := true
		for _, e := range edges[name] {
			if first || Compare(e.con.Floor(), floor) > 0 {
				floor, best = e.con.Floor(), e
				first = false
			}
		}
		return floor, best
	}
	for {
		changed := false
		for _, name := range sortedEdgeKeys(edges) {
			floor, demand := maxFloor(name)
			if p, ok := picks[name]; ok && Compare(p, floor) >= 0 {
				continue // the pick already sits at or above this floor
			}
			picks[name] = floor
			changed = true
			vs := reg.Versions(name)
			if len(vs) == 0 {
				return nil, &Conflict{Code: "E2001", Name: name}
			}
			held := false
			for _, v := range vs {
				if Compare(v, floor) == 0 {
					held = true
					break
				}
			}
			if !held {
				return nil, &Conflict{Code: "E2002", Name: name, Pick: floor, Demand: demand.con, Dep: demand.dep}
			}
			table, _ := reg.Deps(name, floor)
			if len(table) == 0 {
				continue
			}
			self := name + "@" + floor.String()
			chain := demand.chain + " -> " + self
			for _, target := range sortedNames(table) {
				edges[target] = append(edges[target], edge{con: table[target], dep: self, chain: chain})
			}
		}
		if !changed {
			break
		}
	}
	// Terminal validation: every constraint must allow the pick — with the
	// pick at the maximum of floors, only a ceiling can fail (an exactly
	// pinned pick satisfies its own constraint; Allows carries that rule).
	// The first violating package in name order reports, and within it the
	// first violated constraint in edge (dependency) order — deterministic.
	for _, name := range sortedEdgeKeys(edges) {
		pick := picks[name]
		_, raiser := maxFloor(name)
		for _, e := range edges[name] {
			if !e.con.Allows(pick) {
				return nil, &Conflict{
					Code:   "E2002",
					Name:   name,
					Pick:   pick,
					Con:    e.con,
					Chain:  e.chain,
					Raised: raiser.con,
					Raiser: raiser.dep,
				}
			}
		}
	}
	return picks, nil
}

func sortedNames(t Table) []string {
	names := make([]string, 0, len(t))
	for n := range t {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func sortedEdgeKeys(edges map[string][]edge) []string {
	keys := make([]string, 0, len(edges))
	for k := range edges {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
