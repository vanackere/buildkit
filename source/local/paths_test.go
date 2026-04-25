package local

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCanonicalPaths(t *testing.T) {
	t.Run("nil is full", func(t *testing.T) {
		require.True(t, canonicalPaths(nil).full)
	})
	t.Run("empty is full", func(t *testing.T) {
		require.True(t, canonicalPaths([]string{}).full)
	})
	t.Run("dot promotes to full", func(t *testing.T) {
		// "." denotes the whole context; mixing it with explicit paths
		// would be ambiguous, so we collapse to full.
		require.True(t, canonicalPaths([]string{"a", "."}).full)
	})
	t.Run("clean and dedupe and sort", func(t *testing.T) {
		// path.Clean strips "./" prefix and trailing slashes; the result
		// must be a sorted, deduplicated slice.
		got := canonicalPaths([]string{"./b", "a/", "b", "a"})
		require.False(t, got.full)
		require.Equal(t, []string{"a", "b"}, got.paths)
	})
	t.Run("nested paths preserved", func(t *testing.T) {
		got := canonicalPaths([]string{"app/cfg", "tools/cfg"})
		require.Equal(t, []string{"app/cfg", "tools/cfg"}, got.paths)
	})
}

func TestPathSetCovers(t *testing.T) {
	full := pathSet{full: true}
	ab := canonicalPaths([]string{"a", "b"})
	a := canonicalPaths([]string{"a"})
	c := canonicalPaths([]string{"c"})
	empty := pathSet{}

	require.True(t, full.covers(ab), "full covers any explicit set")
	require.True(t, full.covers(full), "full covers full")
	require.True(t, full.covers(empty), "full covers empty")

	require.False(t, ab.covers(full), "explicit cannot cover full")
	require.True(t, ab.covers(a), "{a,b} covers {a}")
	require.True(t, ab.covers(ab), "{a,b} covers {a,b}")
	require.False(t, ab.covers(c), "{a,b} does not cover {c}")
	require.True(t, ab.covers(empty), "any set covers an empty request")

	require.False(t, empty.covers(a), "empty does not cover {a}")
	require.True(t, empty.covers(empty), "empty covers empty")
}

func TestPathSetMissing(t *testing.T) {
	full := pathSet{full: true}
	ab := canonicalPaths([]string{"a", "b"})
	abc := canonicalPaths([]string{"a", "b", "c"})

	require.True(t, full.missing(abc).empty(), "full lacks nothing")

	// {a,b} ⊂ {a,b,c}: c is the only missing path.
	missing := ab.missing(abc)
	require.False(t, missing.full)
	require.Equal(t, []string{"c"}, missing.paths)

	// Non-full receiver against a full request must report full
	// missing, signalling FSSync should sync everything.
	require.True(t, ab.missing(full).full)

	// Disjoint sets: every requested path is missing.
	bd := canonicalPaths([]string{"b", "d"})
	missing = ab.missing(bd)
	require.Equal(t, []string{"d"}, missing.paths)
}

func TestPathSetUnion(t *testing.T) {
	full := pathSet{full: true}
	ab := canonicalPaths([]string{"a", "b"})
	bc := canonicalPaths([]string{"b", "c"})

	require.True(t, full.union(ab).full)
	require.True(t, ab.union(full).full)

	// Sorted, deduplicated merge.
	require.Equal(t, []string{"a", "b", "c"}, ab.union(bc).paths)

	// Empty union with explicit returns explicit.
	empty := pathSet{}
	require.Equal(t, []string{"a", "b"}, empty.union(ab).paths)
	require.Equal(t, []string{"a", "b"}, ab.union(empty).paths)
}

func TestPathSetFollowPaths(t *testing.T) {
	require.Nil(t, pathSet{full: true}.followPaths(),
		"full set must round-trip to a nil filter")
	require.Equal(t, []string{"a", "b"}, canonicalPaths([]string{"b", "a"}).followPaths())
	require.Nil(t, pathSet{}.followPaths(),
		"empty (zero) set has no paths and contributes no filter")
}
