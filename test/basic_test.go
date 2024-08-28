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
	"google.golang.org/grpc/test/bufconn"
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

func bufDialer(context.Context, string) (net.Conn, error) {
	return lis.Dial()
}

func runTofuCommand(t *testing.T, ctx context.Context, command string, args []string, workingDir string, envVars map[string]string) (string, string, error) {
	t.Helper()

	// TODO: Update the deprecated usage of DialContext and WithInsecure below

	// nolint:staticcheck
	conn, err := grpc.DialContext(ctx, "", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
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
