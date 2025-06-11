package integration_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	tgengine "github.com/gruntwork-io/terragrunt-engine-go/proto"
	"github.com/gruntwork-io/terragrunt-engine-opentofu/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/anypb"
)

const bufSize = 1024 * 1024

var lis *bufconn.Listener

func init() {
	lis = bufconn.Listen(bufSize)
	server := grpc.NewServer()
	tgengine.RegisterEngineServer(server, &engine.TofuEngine{})
	go func() {
		if err := server.Serve(lis); err != nil {
			panic(err)
		}
	}()
}

// Helper function to create anypb.Any from a string value
func createStringAny(value string) (*anypb.Any, error) {
	anyValue := &anypb.Any{
		Value: []byte(value),
	}
	return anyValue, nil
}

func TestRun(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	stdout, stderr, err := runTofuCommand(t, ctx, "tofu", []string{"init"}, "fixture-basic-project", map[string]string{})
	require.NoError(t, err)

	require.NotEmpty(t, stdout)
	require.Empty(t, stderr)
	assert.Contains(t, stdout, "Initializing the backend...")
}

func TestVarPassing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	_, _, err := runTofuCommand(t, ctx, "tofu", []string{"init"}, "fixture-variables", map[string]string{})
	require.NoError(t, err)

	testValue := fmt.Sprintf("test_value_%v", time.Now().Unix())
	stdout, stderr, err := runTofuCommand(t, ctx, "tofu", []string{"plan"}, "fixture-variables", map[string]string{"TF_VAR_test_var": testValue})
	require.NoError(t, err)

	require.NotEmpty(t, stdout)
	require.Empty(t, stderr)
	assert.Contains(t, stdout, testValue)
}

func TestAutoInstallExplicitVersion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Test with explicit version v1.9.1
	version := "v1.9.1"
	versionAny, err := createStringAny(version)
	require.NoError(t, err)

	meta := map[string]*anypb.Any{
		"tofu_version": versionAny,
	}

	stdout, stderr, err := runTofuCommandWithInit(t, ctx, "tofu", []string{"version"}, "fixture-basic-project", map[string]string{}, meta)
	require.NoError(t, err)

	require.NotEmpty(t, stdout)
	require.Empty(t, stderr)
	// Verify that the correct version was downloaded and used
	assert.Contains(t, stdout, "OpenTofu v1.9.1")
}

func TestAutoInstallLatestVersion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Test with "latest" version
	versionAny, err := createStringAny("latest")
	require.NoError(t, err)

	meta := map[string]*anypb.Any{
		"tofu_version": versionAny,
	}

	stdout, stderr, err := runTofuCommandWithInit(t, ctx, "tofu", []string{"version"}, "fixture-basic-project", map[string]string{}, meta)
	require.NoError(t, err)

	require.NotEmpty(t, stdout)
	require.Empty(t, stderr)
	// Verify that a valid OpenTofu version was downloaded and used
	assert.Contains(t, stdout, "OpenTofu v")
}

func TestNoAutoInstallWithoutVersion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Test without specifying version (should use system binary)
	meta := map[string]*anypb.Any{}

	stdout, _, err := runTofuCommandWithInit(t, ctx, "tofu", []string{"version"}, "fixture-basic-project", map[string]string{}, meta)

	// This test might fail if system doesn't have tofu installed, which is expected behavior
	if err != nil {
		// Verify that it attempted to use system binary (no auto-download)
		assert.Contains(t, err.Error(), "executable file not found")
		return
	}

	require.NotEmpty(t, stdout)
	// If system tofu is available, verify it's being used
	assert.Contains(t, stdout, "OpenTofu v")
}

func TestAutoInstallWithCustomInstallDir(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Test with explicit version and custom install directory
	version := "v1.9.1"
	installDir := "/tmp/test-tofu-install"

	versionAny, err := createStringAny(version)
	require.NoError(t, err)

	installDirAny, err := createStringAny(installDir)
	require.NoError(t, err)

	meta := map[string]*anypb.Any{
		"tofu_version":     versionAny,
		"tofu_install_dir": installDirAny,
	}

	stdout, stderr, err := runTofuCommandWithInit(t, ctx, "tofu", []string{"version"}, "fixture-basic-project", map[string]string{}, meta)
	require.NoError(t, err)

	require.NotEmpty(t, stdout)
	require.Empty(t, stderr)
	// Verify that the correct version was downloaded and used
	assert.Contains(t, stdout, "OpenTofu v1.9.1")

	// Clean up the custom install directory
	defer func() {
		_ = os.RemoveAll(installDir)
	}()
}

func bufDialer(context.Context, string) (net.Conn, error) {
	return lis.Dial()
}

func runTofuCommand(t *testing.T, ctx context.Context, command string, args []string, workingDir string, envVars map[string]string) (string, string, error) {
	t.Helper()

	conn, err := grpc.NewClient("passthrough://bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return "", "", err
	}
	defer func() {
		err := conn.Close()
		require.NoError(t, err)
	}()

	client := tgengine.NewEngineClient(conn)
	stream, err := client.Run(ctx, &tgengine.RunRequest{
		Command:    command,
		Args:       args,
		WorkingDir: workingDir,
		EnvVars:    envVars,
	})
	if err != nil {
		return "", "", err
	}

	var stdout strings.Builder
	var stderr strings.Builder

	for {
		resp, err := stream.Recv()
		if err != nil {
			break
		}

		stdout.WriteString(resp.GetStdout())
		stderr.WriteString(resp.GetStderr())

		_, err = fmt.Fprint(os.Stdout, resp.GetStdout())
		if err != nil {
			return "", "", err
		}

		_, err = fmt.Fprint(os.Stderr, resp.GetStderr())
		if err != nil {
			return "", "", err
		}
	}

	return stdout.String(), stderr.String(), nil
}

func runTofuCommandWithInit(t *testing.T, ctx context.Context, command string, args []string, workingDir string, envVars map[string]string, meta map[string]*anypb.Any) (string, string, error) {
	t.Helper()

	conn, err := grpc.NewClient("passthrough://bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return "", "", err
	}
	defer func() {
		err := conn.Close()
		require.NoError(t, err)
	}()

	client := tgengine.NewEngineClient(conn)

	// First call Init with the specified metadata
	initStream, err := client.Init(ctx, &tgengine.InitRequest{
		Meta: meta,
	})
	if err != nil {
		return "", "", err
	}

	// Read init response (if any)
	for {
		_, err := initStream.Recv()
		if err != nil {
			break
		}
	}

	// Then run the command
	stream, err := client.Run(ctx, &tgengine.RunRequest{
		Command:    command,
		Args:       args,
		WorkingDir: workingDir,
		EnvVars:    envVars,
	})
	if err != nil {
		return "", "", err
	}

	var stdout strings.Builder
	var stderr strings.Builder

	for {
		resp, err := stream.Recv()
		if err != nil {
			break
		}

		stdout.WriteString(resp.GetStdout())
		stderr.WriteString(resp.GetStderr())

		_, err = fmt.Fprint(os.Stdout, resp.GetStdout())
		if err != nil {
			return "", "", err
		}

		_, err = fmt.Fprint(os.Stderr, resp.GetStderr())
		if err != nil {
			return "", "", err
		}
	}

	return stdout.String(), stderr.String(), nil
}
