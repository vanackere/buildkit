package local

import (
	"testing"

	"github.com/moby/buildkit/cache"
	"github.com/stretchr/testify/require"
)

// fakeRefMetadata is a minimal in-memory cache.RefMetadata used to
// exercise the GetString/SetString-backed accessors on cacheRefMetadata
// without standing up a full cache.Manager. The embedded interface is
// nil; only the two methods we override are safe to call.
type fakeRefMetadata struct {
	cache.RefMetadata
	values map[string]string
}

func (f *fakeRefMetadata) GetString(key string) string {
	return f.values[key]
}

func (f *fakeRefMetadata) SetString(key, val, _ string) error {
	if f.values == nil {
		f.values = map[string]string{}
	}
	f.values[key] = val
	return nil
}

func TestSyncedPathsRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		in   pathSet
	}{
		{"full", pathSet{full: true}},
		{"empty", pathSet{}},
		{"explicit", pathSet{paths: []string{"a", "b/c"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			md := cacheRefMetadata{&fakeRefMetadata{}}

			got, ok, err := md.getSyncedPaths()
			require.NoError(t, err)
			require.False(t, ok, "fresh ref must report no syncedPaths metadata")
			require.Equal(t, pathSet{}, got)

			require.NoError(t, md.setSyncedPaths(tc.in))

			got, ok, err = md.getSyncedPaths()
			require.NoError(t, err)
			require.True(t, ok)
			require.Equal(t, tc.in.full, got.full)
			require.Equal(t, tc.in.paths, got.paths)
		})
	}
}

// TestSyncedPathsAbsentDistinguishedFromFull guards the contract that
// an unset metadata record must not be confused with a recorded "full"
// state — the former needs a sync, the latter doesn't.
func TestSyncedPathsAbsentDistinguishedFromFull(t *testing.T) {
	md := cacheRefMetadata{&fakeRefMetadata{}}

	_, ok, err := md.getSyncedPaths()
	require.NoError(t, err)
	require.False(t, ok)

	require.NoError(t, md.setSyncedPaths(pathSet{full: true}))
	got, ok, err := md.getSyncedPaths()
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, got.full)
}

func TestDecodeSyncedPathsCanonicalizesStoredData(t *testing.T) {
	// Simulate non-canonical stored JSON: unsorted, with duplicates.
	raw := `{"paths":["b","a","b"]}`
	ps, err := decodeSyncedPaths(raw)
	require.NoError(t, err)
	require.False(t, ps.full)
	require.Equal(t, []string{"a", "b"}, ps.paths)
}
