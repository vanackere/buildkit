package local

import (
	"path"
	"sort"
)

// pathSet is the set of paths a local-source ref has fetched, or that a
// caller is requesting. A nil or empty FollowPaths list passed to
// filesync.FSSync syncs everything; canonicalPaths maps that case to a
// pathSet with full=true. The zero value is the empty set (no paths,
// not full); callers should check empty() before calling followPaths().
type pathSet struct {
	// full is true when the set represents the entire context (no path
	// filter). A full set covers every other set; a non-full set never
	// covers a full one.
	full bool
	// paths is the canonical (sorted, deduplicated, path.Clean-d, no
	// empty entries) sorted list of paths. Only meaningful when full is
	// false.
	paths []string
}

// canonicalPaths normalizes a FollowPaths slice into a pathSet. A nil or
// empty input is interpreted as "full context", matching the semantics of
// filesync.FSSync.
func canonicalPaths(in []string) pathSet {
	if len(in) == 0 {
		return pathSet{full: true}
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, p := range in {
		c := path.Clean(p)
		if c == "." {
			// A "." entry means "the whole context". Treat the entire
			// set as full so we don't conflate it with a real path.
			return pathSet{full: true}
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	sort.Strings(out)
	return pathSet{paths: out}
}

// covers reports whether the receiver's synced paths are a superset of
// want. A full set covers every request; a non-full set covers a full
// request only if both are full.
func (have pathSet) covers(want pathSet) bool {
	if have.full {
		return true
	}
	if want.full {
		return false
	}
	// Both are explicit, sorted, deduplicated path lists. Walk in
	// lockstep: every element of want must appear in have.
	i := 0
	for _, w := range want.paths {
		for i < len(have.paths) && have.paths[i] < w {
			i++
		}
		if i == len(have.paths) || have.paths[i] != w {
			return false
		}
	}
	return true
}

// missing returns the paths in want that the receiver does not yet cover.
// A full receiver returns the zero pathSet (nothing to add). A full want
// against a non-full receiver returns a full pathSet (sync everything).
func (have pathSet) missing(want pathSet) pathSet {
	if have.full {
		return pathSet{}
	}
	if want.full {
		return pathSet{full: true}
	}
	var diff []string
	i := 0
	for _, w := range want.paths {
		for i < len(have.paths) && have.paths[i] < w {
			i++
		}
		if i == len(have.paths) || have.paths[i] != w {
			diff = append(diff, w)
		}
	}
	if len(diff) == 0 {
		return pathSet{}
	}
	return pathSet{paths: diff}
}

// union returns the smallest pathSet that covers both inputs.
func (have pathSet) union(other pathSet) pathSet {
	if have.full || other.full {
		return pathSet{full: true}
	}
	if len(have.paths) == 0 {
		return pathSet{paths: append([]string(nil), other.paths...)}
	}
	if len(other.paths) == 0 {
		return pathSet{paths: append([]string(nil), have.paths...)}
	}
	merged := make([]string, 0, len(have.paths)+len(other.paths))
	i, j := 0, 0
	for i < len(have.paths) && j < len(other.paths) {
		switch {
		case have.paths[i] < other.paths[j]:
			merged = append(merged, have.paths[i])
			i++
		case have.paths[i] > other.paths[j]:
			merged = append(merged, other.paths[j])
			j++
		default:
			merged = append(merged, have.paths[i])
			i++
			j++
		}
	}
	merged = append(merged, have.paths[i:]...)
	merged = append(merged, other.paths[j:]...)
	return pathSet{paths: merged}
}

// followPaths converts a pathSet back to the []string form expected by
// filesync.FSSync's FollowPaths option. A full set returns nil, which
// FSSync treats as "no filter".
func (have pathSet) followPaths() []string {
	if have.full {
		return nil
	}
	return have.paths
}

// empty reports whether the set has no paths and is not full. A pathSet
// returned by missing() with empty=true means there is nothing to sync.
func (have pathSet) empty() bool {
	return !have.full && len(have.paths) == 0
}
