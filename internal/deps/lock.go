package deps

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// LockEntry is one package's lock record: the name, the resolved version,
// and the digest of its acquired contents.
type LockEntry struct {
	Name    string
	Version Version
	Digest  string
}

const lockHeader = "# We lockfile — machine-written by resolution; commit it, do not edit."

// WriteLock writes the lockfile in its machine shape: the comment line,
// one [[package]] block per entry in name order, a blank line between
// sections, single trailing newline. The caller sorts nothing — entries
// write sorted.
func WriteLock(path string, entries []LockEntry) error {
	sorted := append([]LockEntry(nil), entries...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	var b strings.Builder
	b.WriteString(lockHeader + "\n\n")
	for i, e := range sorted {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("[[package]]\n")
		b.WriteString("name = \"" + e.Name + "\"\n")
		b.WriteString("version = \"" + e.Version.String() + "\"\n")
		b.WriteString("digest = \"" + e.Digest + "\"\n")
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// ReadLock parses the lockfile. A missing, unparseable, or incomplete
// lock reads as false — the caller regenerates silently, with no
// diagnostic (chapter 22 R4: machine-written, never hand-edited).
func ReadLock(path string) ([]LockEntry, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var entries []LockEntry
	cur := -1 // index of the entry being read, -1 before the first block
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if line == "[[package]]" {
			entries = append(entries, LockEntry{})
			cur = len(entries) - 1
			continue
		}
		i := strings.Index(line, "=")
		if i < 0 || cur < 0 {
			return nil, false // not the machine shape
		}
		key := strings.TrimSpace(line[:i])
		val := strings.TrimSpace(line[i+1:])
		if len(val) < 2 || val[0] != '"' || val[len(val)-1] != '"' {
			return nil, false
		}
		val = val[1 : len(val)-1]
		switch key {
		case "name":
			entries[cur].Name = val
		case "version":
			v, ok := ParseVersion(val)
			if !ok {
				return nil, false
			}
			entries[cur].Version = v
		case "digest":
			entries[cur].Digest = val
		default:
			return nil, false
		}
	}
	for _, e := range entries {
		if e.Name == "" || e.Digest == "" {
			return nil, false // incomplete block
		}
	}
	return entries, len(entries) > 0
}

// LockState is the lockfile's verdict against the root graph.
type LockState int

const (
	// LockWins: the lock satisfies the graph — every reachable package
	// locked, every constraint allowed by its locked version, every cache
	// entry present and digesting to its record. The locked versions are
	// the build's truth; no source is touched.
	LockWins LockState = iota
	// LockStale: the lock cannot witness the graph — regenerate by full
	// resolution. A stale lock is silent, not diagnosed.
	LockStale
	// LockTampered: cached content hashes away from its record — E2005.
	LockTampered
)

// Integrity names the E2005 mismatch: the package, its version, what the
// cache hashes to, and what the lock records.
type Integrity struct {
	Name    string
	Version Version
	Got     string
	Want    string
}

// LockStatus judges the lockfile against root's dependency graph using
// only the cache (design D4's lock-win path): the closure walks from root
// through the locked packages' own cached manifests; every edge's
// constraint must allow the locked pick and every cache entry must be
// present. Only then are digests compared — the matching entry is the
// source-existence witness offline (what the source answered at
// acquisition is pinned by the lock). Reachable roots come back for the
// loader's cache leg; non-reachable entries ride along harmlessly.
func LockStatus(root Table, entries []LockEntry, cacheRoot string) (LockState, map[string]string, *Integrity) {
	byName := map[string]LockEntry{}
	for _, e := range entries {
		byName[e.Name] = e
	}
	// Closure: breadth-first from root's names, constraints carried from
	// the root table then each locked package's cached manifest.
	type step struct {
		name string
		deps Table
	}
	var queue []step
	seen := map[string]bool{}
	enqueue := func(name string) {
		if seen[name] {
			return
		}
		seen[name] = true
		e, ok := byName[name]
		if !ok {
			return // missing entry: stale (the walk keeps it reachable)
		}
		table, ok := readCachedDeps(cacheRoot, name, e.Version)
		if !ok {
			table = nil // no cache copy yet: stale by presence below
		}
		queue = append(queue, step{name: name, deps: table})
	}
	for _, name := range sortedNames(root) {
		enqueue(name)
	}
	roots := map[string]string{}
	for i := 0; i < len(queue); i++ {
		s := queue[i]
		e, locked := byName[s.name]
		if !locked {
			return LockStale, nil, nil
		}
		if con, ok := root[s.name]; ok && !con.Allows(e.Version) {
			return LockStale, nil, nil // root's own constraint outruns the lock
		}
		dir := PackageDir(cacheRoot, s.name, e.Version)
		if _, err := os.Stat(dir); err != nil {
			return LockStale, nil, nil // cache cannot witness the lock
		}
		roots[s.name] = dir
		for target, con := range s.deps {
			te, ok := byName[target]
			if !ok || !con.Allows(te.Version) {
				return LockStale, nil, nil // the graph outruns the lock
			}
			enqueue(target)
		}
	}
	// Digests last, in name order: the first mismatch is the E2005 face.
	names := make([]string, 0, len(roots))
	for n := range roots {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		e := byName[n]
		got, err := Digest(PackageDir(cacheRoot, n, e.Version))
		if err != nil || got != e.Digest {
			return LockTampered, nil, &Integrity{Name: n, Version: e.Version, Got: got, Want: e.Digest}
		}
	}
	return LockWins, roots, nil
}

// readCachedDeps reads a cached package's manifest dependencies for the
// closure walk. An unreadable or invalid manifest reads as absent — the
// lock then cannot witness the graph and regenerates.
func readCachedDeps(cacheRoot, name string, v Version) (Table, bool) {
	raw, err := os.ReadFile(filepath.Join(PackageDir(cacheRoot, name, v), "we.toml"))
	if err != nil {
		return nil, false
	}
	table, fault := ReadDeps(parseDepsSection(string(raw)))
	if fault != nil {
		return nil, false
	}
	return table, true
}
