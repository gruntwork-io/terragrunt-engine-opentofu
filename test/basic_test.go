package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
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
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	bufSize         = 1024 * 1024
	versionCmd      = "version"
	tofuVersionMeta = "tofu_version"

	noAutoProviderCacheDirMeta = "no_auto_provider_cache_dir"
)

// createStringAny encodes a meta value the way Terragrunt sends it: a JSON-encoded string
// wrapped in a google.protobuf.Value (see ConvertMetaToProtobuf in Terragrunt).
func createStringAny(value string) (*anypb.Any, error) {
	jsonData, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	protoValue, err := structpb.NewValue(string(jsonData))
	if err != nil {
		return nil, err
	}

	return anypb.New(protoValue)
}

func TestRun(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	stdout, stderr, err := runTofuCommand(
		t,
		ctx,
		&engine.TofuEngine{PluginCacheDir: t.TempDir()},
		"tofu",
		[]string{"init"},
		"fixture-basic-project",
		map[string]string{},
	)
	require.NoError(t, err)

	require.NotEmpty(t, stdout)
	require.Empty(t, stderr)
	assert.Contains(t, stdout, "Initializing the backend...")
}

func TestVarPassing(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	_, _, err := runTofuCommand(
		t,
		ctx,
		&engine.TofuEngine{PluginCacheDir: t.TempDir()},
		"tofu",
		[]string{"init"},
		"fixture-variables",
		map[string]string{},
	)
	require.NoError(t, err)

	testValue := fmt.Sprintf("test_value_%v", time.Now().Unix())
	stdout, stderr, err := runTofuCommand(
		t,
		ctx,
		&engine.TofuEngine{PluginCacheDir: t.TempDir()},
		"tofu",
		[]string{"plan"},
		"fixture-variables",
		map[string]string{"TF_VAR_test_var": testValue},
	)
	require.NoError(t, err)

	require.NotEmpty(t, stdout)
	require.Empty(t, stderr)
	assert.Contains(t, stdout, testValue)
}

func TestAutoInstallExplicitVersion(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Test with explicit version v1.9.1
	version := "v1.9.1"
	versionAny, err := createStringAny(version)
	require.NoError(t, err)

	meta := map[string]*anypb.Any{
		tofuVersionMeta: versionAny,
	}

	stdout, stderr, err := runTofuCommandWithInit(
		t,
		ctx,
		&engine.TofuEngine{PluginCacheDir: t.TempDir()},
		"tofu",
		[]string{versionCmd},
		"fixture-basic-project",
		map[string]string{},
		meta,
	)
	require.NoError(t, err)

	require.NotEmpty(t, stdout)
	require.Empty(t, stderr)
	// Verify that the correct version was downloaded and used
	assert.Contains(t, stdout, "OpenTofu v1.9.1")
}

func TestAutoInstallInvalidVersion(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	version := "v0.0.0"
	versionAny, err := createStringAny(version)
	require.NoError(t, err)

	meta := map[string]*anypb.Any{
		tofuVersionMeta: versionAny,
	}

	_, _, err = runTofuCommandWithInit(
		t,
		ctx,
		&engine.TofuEngine{PluginCacheDir: t.TempDir()},
		"tofu",
		[]string{versionCmd},
		"fixture-basic-project",
		map[string]string{},
		meta,
	)
	require.ErrorIs(t, err, ErrFailedToInitialize)

	assert.Contains(t, err.Error(), "failed to download OpenTofu: No such version: 0.0.0")
}

func TestAutoInstallLatestVersion(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Test with "latest" version
	versionAny, err := createStringAny("latest")
	require.NoError(t, err)

	meta := map[string]*anypb.Any{
		tofuVersionMeta: versionAny,
	}

	stdout, stderr, err := runTofuCommandWithInit(
		t,
		ctx,
		&engine.TofuEngine{PluginCacheDir: t.TempDir()},
		"tofu",
		[]string{versionCmd},
		"fixture-basic-project",
		map[string]string{},
		meta,
	)
	require.NoError(t, err)

	require.NotEmpty(t, stdout)
	require.Empty(t, stderr)
	// Verify that a valid OpenTofu version was downloaded and used
	assert.Contains(t, stdout, "OpenTofu v")
}

func TestNoAutoInstallWithoutVersion(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Test without specifying version (should use system binary)
	meta := map[string]*anypb.Any{}

	stdout, _, err := runTofuCommandWithInit(
		t,
		ctx,
		&engine.TofuEngine{PluginCacheDir: t.TempDir()},
		"tofu",
		[]string{versionCmd},
		"fixture-basic-project",
		map[string]string{},
		meta,
	)

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
	ctx := t.Context()

	// Test with explicit version and custom install directory
	version := "v1.9.1"
	installDir := "/tmp/test-tofu-install"

	versionAny, err := createStringAny(version)
	require.NoError(t, err)

	installDirAny, err := createStringAny(installDir)
	require.NoError(t, err)

	meta := map[string]*anypb.Any{
		tofuVersionMeta:    versionAny,
		"tofu_install_dir": installDirAny,
	}

	stdout, stderr, err := runTofuCommandWithInit(
		t,
		ctx,
		&engine.TofuEngine{PluginCacheDir: t.TempDir()},
		"tofu",
		[]string{versionCmd},
		"fixture-basic-project",
		map[string]string{},
		meta,
	)
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

func TestPluginCacheSharing(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		envVars     map[string]string
		meta        map[string]string
		name        string
		tofuVersion string
		wantCached  bool
	}{
		{
			name:        "tofu 1.10 or newer shares the cache",
			tofuVersion: "v1.11.2",
			envVars:     map[string]string{},
			wantCached:  true,
		},
		{
			name:        "tofu older than 1.10 does not share the cache",
			tofuVersion: "v1.9.1",
			envVars:     map[string]string{},
			wantCached:  false,
		},
		{
			name:        "request that names the variable keeps its value",
			tofuVersion: "v1.11.2",
			envVars:     map[string]string{"TF_PLUGIN_CACHE_DIR": ""},
			wantCached:  false,
		},
		{
			name:        "true no_auto_provider_cache_dir meta does not share the cache",
			tofuVersion: "v1.11.2",
			envVars:     map[string]string{},
			meta:        map[string]string{noAutoProviderCacheDirMeta: "true"},
			wantCached:  false,
		},
		{
			name:        "false no_auto_provider_cache_dir meta shares the cache",
			tofuVersion: "v1.11.2",
			envVars:     map[string]string{},
			meta:        map[string]string{noAutoProviderCacheDirMeta: "false"},
			wantCached:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			versionAny, err := createStringAny(tc.tofuVersion)
			require.NoError(t, err)

			config, err := os.ReadFile(filepath.Join("fixture-basic-project", "main.tf"))
			require.NoError(t, err)

			workingDir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(workingDir, "main.tf"), config, 0600))

			pluginCacheDir := t.TempDir()
			meta := map[string]*anypb.Any{tofuVersionMeta: versionAny}

			for key, value := range tc.meta {
				valueAny, err := createStringAny(value)
				require.NoError(t, err)

				meta[key] = valueAny
			}

			_, _, err = runTofuCommandWithInit(
				t,
				t.Context(),
				&engine.TofuEngine{PluginCacheDir: pluginCacheDir},
				"tofu",
				[]string{"init"},
				workingDir,
				tc.envVars,
				meta,
			)
			require.NoError(t, err)

			cached, err := os.ReadDir(pluginCacheDir)
			require.NoError(t, err)

			if tc.wantCached {
				assert.NotEmpty(t, cached)
				return
			}

			assert.Empty(t, cached)
		})
	}
}

// newEngineClient starts an engine server for a single command, so engine state such as
// the OpenTofu binary path set by Init does not leak between parallel tests.
func newEngineClient(t *testing.T, eng *engine.TofuEngine) tgengine.EngineClient {
	t.Helper()

	lis := bufconn.Listen(bufSize)
	server := grpc.NewServer()
	tgengine.RegisterEngineServer(server, eng)

	go func() {
		if err := server.Serve(lis); err != nil {
			panic(err)
		}
	}()

	dialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}

	conn, err := grpc.NewClient(
		"passthrough://bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, conn.Close())
		server.Stop()
	})

	return tgengine.NewEngineClient(conn)
}

func runTofuCommand(
	t *testing.T,
	ctx context.Context,
	eng *engine.TofuEngine,
	command string,
	args []string,
	workingDir string,
	envVars map[string]string,
) (string, string, error) {
	t.Helper()

	client := newEngineClient(t, eng)

	stream, err := client.Run(ctx, &tgengine.RunRequest{
		Command:    command,
		Args:       args,
		WorkingDir: workingDir,
		EnvVars:    envVars,
	})
	if err != nil {
		return "", "", err
	}

	var (
		stdout strings.Builder
		stderr strings.Builder
	)

	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return "", "", err
		}

		if stdoutMsg := resp.GetStdout(); stdoutMsg != nil {
			stdout.WriteString(stdoutMsg.GetContent())

			_, err = fmt.Fprint(os.Stdout, stdoutMsg.GetContent())
			if err != nil {
				return "", "", err
			}
		}

		if stderrMsg := resp.GetStderr(); stderrMsg != nil {
			stderr.WriteString(stderrMsg.GetContent())

			_, err = fmt.Fprint(os.Stderr, stderrMsg.GetContent())
			if err != nil {
				return "", "", err
			}
		}
	}

	return stdout.String(), stderr.String(), nil
}

var ErrFailedToInitialize = errors.New("failed to initialize")

func runTofuCommandWithInit(
	t *testing.T,
	ctx context.Context,
	eng *engine.TofuEngine,
	command string,
	args []string,
	workingDir string,
	envVars map[string]string,
	meta map[string]*anypb.Any,
) (string, string, error) {
	t.Helper()

	client := newEngineClient(t, eng)

	// First call Init with the specified metadata
	initStream, err := client.Init(ctx, &tgengine.InitRequest{
		Meta: meta,
	})
	if err != nil {
		return "", "", err
	}

	// Read init response (if any)
	var stderrContent strings.Builder

	for {
		res, err := initStream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return "", "", err
		}

		if stderrMsg := res.GetStderr(); stderrMsg != nil {
			stderrContent.WriteString(stderrMsg.GetContent())
		}

		// Also capture error log messages
		if logMsg := res.GetLog(); logMsg != nil &&
			logMsg.GetLevel() == tgengine.LogLevel_LOG_LEVEL_ERROR {
			stderrContent.WriteString(logMsg.GetContent())
		}

		if exitResult := res.GetExitResult(); exitResult != nil && exitResult.GetCode() != 0 {
			return "", "", fmt.Errorf("%w: %s", ErrFailedToInitialize, stderrContent.String())
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

	var (
		stdout strings.Builder
		stderr strings.Builder
	)

	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return "", "", err
		}

		if stdoutMsg := resp.GetStdout(); stdoutMsg != nil {
			stdout.WriteString(stdoutMsg.GetContent())

			_, err = fmt.Fprint(os.Stdout, stdoutMsg.GetContent())
			if err != nil {
				return "", "", err
			}
		}

		if stderrMsg := resp.GetStderr(); stderrMsg != nil {
			stderr.WriteString(stderrMsg.GetContent())

			_, err = fmt.Fprint(os.Stderr, stderrMsg.GetContent())
			if err != nil {
				return "", "", err
			}
		}
	}

	return stdout.String(), stderr.String(), nil
}
