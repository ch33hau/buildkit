//go:build dfaddchecksum
// +build dfaddchecksum

package dockerfile

import (
	"fmt"
	"testing"

	"github.com/containerd/continuity/fs/fstest"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/frontend/dockerfile/builder"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/util/testutil/httpserver"
	"github.com/moby/buildkit/util/testutil/integration"
	digest "github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/require"
)

var addChecksumTests = integration.TestFuncs(
	testAddChecksum,
)

func init() {
	allTests = append(allTests, addChecksumTests...)
}

func testAddChecksum(t *testing.T, sb integration.Sandbox) {
	f := getFrontend(t, sb)
	f.RequiresBuildctl(t)

	resp := httpserver.Response{
		Etag:    identity.NewID(),
		Content: []byte("content1"),
	}
	server := httpserver.NewTestServer(map[string]httpserver.Response{
		"/foo": resp,
	})
	defer server.Close()

	c, err := client.New(sb.Context(), sb.Address())
	require.NoError(t, err)
	defer c.Close()

	t.Run("Valid", func(t *testing.T) {
		dockerfile := []byte(fmt.Sprintf(`
FROM scratch
ADD --checksum=%s %s /tmp/foo
`, digest.FromBytes(resp.Content).String(), server.URL+"/foo"))
		dir, err := integration.Tmpdir(
			t,
			fstest.CreateFile("Dockerfile", dockerfile, 0600),
		)
		require.NoError(t, err)
		_, err = f.Solve(sb.Context(), c, client.SolveOpt{
			LocalDirs: map[string]string{
				builder.DefaultLocalNameDockerfile: dir,
				builder.DefaultLocalNameContext:    dir,
			},
		}, nil)
		require.NoError(t, err)
	})
	t.Run("DigestMismatch", func(t *testing.T) {
		dockerfile := []byte(fmt.Sprintf(`
FROM scratch
ADD --checksum=%s %s /tmp/foo
`, digest.FromBytes(nil).String(), server.URL+"/foo"))
		dir, err := integration.Tmpdir(
			t,
			fstest.CreateFile("Dockerfile", dockerfile, 0600),
		)
		require.NoError(t, err)
		_, err = f.Solve(sb.Context(), c, client.SolveOpt{
			LocalDirs: map[string]string{
				builder.DefaultLocalNameDockerfile: dir,
				builder.DefaultLocalNameContext:    dir,
			},
		}, nil)
		require.Error(t, err, "digest mismatch")
	})
	t.Run("DigestWithKnownButUnsupportedAlgoName", func(t *testing.T) {
		dockerfile := []byte(fmt.Sprintf(`
FROM scratch
ADD --checksum=md5:7e55db001d319a94b0b713529a756623 %s /tmp/foo
`, server.URL+"/foo"))
		dir, err := integration.Tmpdir(
			t,
			fstest.CreateFile("Dockerfile", dockerfile, 0600),
		)
		require.NoError(t, err)
		_, err = f.Solve(sb.Context(), c, client.SolveOpt{
			LocalDirs: map[string]string{
				builder.DefaultLocalNameDockerfile: dir,
				builder.DefaultLocalNameContext:    dir,
			},
		}, nil)
		require.Error(t, err, "unsupported digest algorithm")
	})
	t.Run("DigestWithUnknownAlgoName", func(t *testing.T) {
		dockerfile := []byte(fmt.Sprintf(`
FROM scratch
ADD --checksum=unknown:%s %s /tmp/foo
`, digest.FromBytes(resp.Content).Encoded(), server.URL+"/foo"))
		dir, err := integration.Tmpdir(
			t,
			fstest.CreateFile("Dockerfile", dockerfile, 0600),
		)
		require.NoError(t, err)
		_, err = f.Solve(sb.Context(), c, client.SolveOpt{
			LocalDirs: map[string]string{
				builder.DefaultLocalNameDockerfile: dir,
				builder.DefaultLocalNameContext:    dir,
			},
		}, nil)
		require.Error(t, err, "unsupported digest algorithm")
	})
	t.Run("DigestWithoutAlgoName", func(t *testing.T) {
		dockerfile := []byte(fmt.Sprintf(`
FROM scratch
ADD --checksum=%s %s /tmp/foo
`, digest.FromBytes(resp.Content).Encoded(), server.URL+"/foo"))
		dir, err := integration.Tmpdir(
			t,
			fstest.CreateFile("Dockerfile", dockerfile, 0600),
		)
		require.NoError(t, err)
		_, err = f.Solve(sb.Context(), c, client.SolveOpt{
			LocalDirs: map[string]string{
				builder.DefaultLocalNameDockerfile: dir,
				builder.DefaultLocalNameContext:    dir,
			},
		}, nil)
		require.Error(t, err, "invalid checksum digest format")
	})
	t.Run("NonHTTPSource", func(t *testing.T) {
		foo := []byte("local file")
		dockerfile := []byte(fmt.Sprintf(`
FROM scratch
ADD --checksum=%s foo /tmp/foo
`, digest.FromBytes(foo).String()))
		dir, err := integration.Tmpdir(
			t,
			fstest.CreateFile("foo", foo, 0600),
			fstest.CreateFile("Dockerfile", dockerfile, 0600),
		)
		require.NoError(t, err)
		_, err = f.Solve(sb.Context(), c, client.SolveOpt{
			LocalDirs: map[string]string{
				builder.DefaultLocalNameDockerfile: dir,
				builder.DefaultLocalNameContext:    dir,
			},
		}, nil)
		require.Error(t, err, "checksum can't be specified for non-HTTP sources")
	})
}
