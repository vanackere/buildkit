package dockerui

import (
	"context"
	"testing"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/solver/pb"
	"github.com/stretchr/testify/require"
)

// findLocalAttr unmarshals an llb.Definition produced by llb.Local() and
// returns the value of the named attribute on the local source op (or "" if
// the attribute is absent). Test helper for inspecting the marshaled op
// without going through a full solve.
func findLocalAttr(t *testing.T, def *llb.Definition, attr string) string {
	t.Helper()
	for _, dt := range def.Def {
		var op pb.Op
		require.NoError(t, op.Unmarshal(dt))
		src := op.GetSource()
		if src == nil {
			continue
		}
		if v, ok := src.Attrs[attr]; ok {
			return v
		}
	}
	return ""
}

// TestMainContextLocalOptsSharedSessionDropsFollowPaths is a regression
// test for the cross-target cache duplication observed when concurrent
// bake solves over the same context have divergent per-target ctxPaths.
// The frontend produces an llb.Local op for the build context whose
// digest depends on FollowPaths, so per-target FollowPaths prevents the
// solver from merging cold, in-flight local source work across solves.
// mainContextLocalOpts is the helper that, when buildx wired up a shared
// session for the context local, appends a final llb.FollowPaths(nil) to
// override any per-target paths the caller passed in.
func TestMainContextLocalOptsSharedSessionDropsFollowPaths(t *testing.T) {
	ctx := context.Background()
	callerOpts := []llb.LocalOption{
		llb.FollowPaths([]string{"file-a", "file-b"}),
	}

	t.Run("non-shared session preserves caller FollowPaths", func(t *testing.T) {
		opts := mainContextLocalOpts("context", "sess-A", nil, callerOpts, false)
		st := llb.Local("context", opts...)
		def, err := st.Marshal(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, findLocalAttr(t, def, pb.AttrFollowPaths),
			"FollowPaths must be preserved when no shared session is in use")
	})

	t.Run("shared session strips FollowPaths", func(t *testing.T) {
		opts := mainContextLocalOpts("context", "sess-shared", nil, callerOpts, true)
		st := llb.Local("context", opts...)
		def, err := st.Marshal(ctx)
		require.NoError(t, err)
		require.Empty(t, findLocalAttr(t, def, pb.AttrFollowPaths),
			"FollowPaths must be stripped when the context resolves through a shared session")
	})

	t.Run("op digest is invariant across divergent caller FollowPaths under shared session", func(t *testing.T) {
		// The whole point: two concurrent solves with different per-target
		// FollowPaths must produce identical llb.Local ops (same digest) so
		// the local source vertex is shared.
		optsA := mainContextLocalOpts("context", "sess-shared", nil,
			[]llb.LocalOption{llb.FollowPaths([]string{"file-a", "file-b"})},
			true)
		optsB := mainContextLocalOpts("context", "sess-shared", nil,
			[]llb.LocalOption{llb.FollowPaths([]string{"file-a", "file-b", "file-c"})},
			true)

		stA := llb.Local("context", optsA...)
		stB := llb.Local("context", optsB...)

		defA, err := stA.Marshal(ctx)
		require.NoError(t, err)
		defB, err := stB.Marshal(ctx)
		require.NoError(t, err)

		require.Equal(t, len(defA.Def), len(defB.Def), "ops should have the same op count")
		for i := range defA.Def {
			require.Equal(t, defA.Def[i], defB.Def[i],
				"op[%d] must be byte-identical so concurrent solves dedupe to one ref", i)
		}
	})

	t.Run("session id is taken from the shared session when provided", func(t *testing.T) {
		opts := mainContextLocalOpts("context", "sess-shared", nil, nil, true)
		st := llb.Local("context", opts...)
		def, err := st.Marshal(ctx)
		require.NoError(t, err)
		require.Equal(t, "sess-shared", findLocalAttr(t, def, pb.AttrLocalSessionID))
	})
}
