package test

import (
	"context"
	"github.com/gruntwork-io/terragrunt-engine-opentofu/engine"
	"testing"

	tgengine "github.com/gruntwork-io/terragrunt-engine-go/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	"net"
	"strings"
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

func bufDialer(context.Context, string) (net.Conn, error) {
	return lis.Dial()
}

func TestInit(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	require.NoError(t, err)
	defer conn.Close()

	client := tgengine.NewEngineClient(conn)
	stream, err := client.Init(ctx, &tgengine.InitRequest{})
	require.NoError(t, err)

	var responses []*tgengine.InitResponse
	for {
		resp, err := stream.Recv()
		if err != nil {
			break
		}
		responses = append(responses, resp)
	}

	require.Len(t, responses, 2)
	assert.Equal(t, "Tofu Initialization started\n", responses[0].Stdout)
	assert.Equal(t, "Tofu Initialization completed\n", responses[1].Stdout)
}

func TestRun(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	require.NoError(t, err)
	defer conn.Close()

	client := tgengine.NewEngineClient(conn)
	stream, err := client.Run(ctx, &tgengine.RunRequest{
		Command:    "echo",
		Args:       []string{"Hello, World!"},
		WorkingDir: "/",
		EnvVars:    map[string]string{},
	})
	require.NoError(t, err)

	var responses []*tgengine.RunResponse
	for {
		resp, err := stream.Recv()
		if err != nil {
			break
		}
		responses = append(responses, resp)
	}

	require.NotEmpty(t, responses)
	assert.Contains(t, concatRunResponses(responses), "Hello, World!")
}

func TestShutdown(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	require.NoError(t, err)
	defer conn.Close()

	client := tgengine.NewEngineClient(conn)
	stream, err := client.Shutdown(ctx, &tgengine.ShutdownRequest{})
	require.NoError(t, err)

	var responses []*tgengine.ShutdownResponse
	for {
		resp, err := stream.Recv()
		if err != nil {
			break
		}
		responses = append(responses, resp)
	}

	require.Len(t, responses, 1)
	assert.Equal(t, "Tofu Shutdown completed\n", responses[0].Stdout)
}

func concatRunResponses(responses []*tgengine.RunResponse) string {
	var stdoutBuilder strings.Builder
	for _, resp := range responses {
		stdoutBuilder.WriteString(resp.Stdout)
	}
	return stdoutBuilder.String()
}
