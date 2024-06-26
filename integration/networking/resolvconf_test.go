package networking

import (
	"os"
	"strings"
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/network"
	"github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

// Regression test for https://github.com/moby/moby/issues/46968
func TestResolvConfLocalhostIPv6(t *testing.T) {
	// No "/etc/resolv.conf" on Windows.
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

	ctx := setupTest(t)

	// Write a resolv.conf that only contains a loopback address.
	// Not using t.TempDir() here because in rootless mode, while the temporary
	// directory gets mode 0777, it's a subdir of an 0700 directory owned by root.
	// So, it's not accessible by the daemon.
	f, err := os.CreateTemp("", "resolv.conf")
	assert.NilError(t, err)
	defer os.Remove(f.Name())
	err = f.Chmod(0644)
	assert.NilError(t, err)
	f.Write([]byte("nameserver 127.0.0.53\n"))

	d := daemon.New(t, daemon.WithEnvVars("DOCKER_TEST_RESOLV_CONF_PATH="+f.Name()))
	d.StartWithBusybox(ctx, t, "--experimental", "--ip6tables")
	defer d.Stop(t)

	c := d.NewClientT(t)
	defer c.Close()

	netName := "nnn"
	network.CreateNoError(ctx, t, c, netName,
		network.WithDriver("bridge"),
		network.WithIPv6(),
		network.WithIPAM("fd49:b5ef:36d9::/64", "fd49:b5ef:36d9::1"),
	)
	defer network.RemoveNoError(ctx, t, c, netName)

	result := container.RunAttach(ctx, t, c,
		container.WithImage("busybox:latest"),
		container.WithNetworkMode(netName),
		container.WithCmd("cat", "/etc/resolv.conf"),
	)
	defer c.ContainerRemove(ctx, result.ContainerID, containertypes.RemoveOptions{
		Force: true,
	})

	output := strings.ReplaceAll(result.Stdout.String(), f.Name(), "RESOLV.CONF")
	assert.Check(t, is.Equal(output, `# Generated by Docker Engine.
# This file can be edited; Docker Engine will not make further changes once it
# has been modified.

nameserver 127.0.0.11
options ndots:0

# Based on host file: 'RESOLV.CONF' (internal resolver)
# ExtServers: [host(127.0.0.53)]
# Overrides: []
# Option ndots from: internal
`))
}
