package engine_test

import (
	"context"
	"encoding/json"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"os"
	"os/exec"

	tgengine "github.com/gruntwork-io/terragrunt-engine-go/proto"
	"github.com/gruntwork-io/terragrunt-engine-opentofu/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

// MockInitServer is a mock implementation of the InitServer interface
type MockInitServer struct {
	mock.Mock
	Responses []*tgengine.InitResponse
}

func (m *MockInitServer) Send(resp *tgengine.InitResponse) error {
	m.Responses = append(m.Responses, resp)
	return nil
}

func (m *MockInitServer) SetHeader(md metadata.MD) error {
	return nil
}

func (m *MockInitServer) SendHeader(md metadata.MD) error {
	return nil
}

func (m *MockInitServer) SetTrailer(md metadata.MD) {
}

func (m *MockInitServer) Context() context.Context {
	return context.TODO()
}

func (m *MockInitServer) SendMsg(msg any) error {
	return nil
}

func (m *MockInitServer) RecvMsg(msg any) error {
	return nil
}

// MockRunServer is a mock implementation of the RunServer interface
type MockRunServer struct {
	mock.Mock
	Responses []*tgengine.RunResponse
}

func (m *MockRunServer) Send(resp *tgengine.RunResponse) error {
	m.Responses = append(m.Responses, resp)
	return nil
}

func (m *MockRunServer) SetHeader(md metadata.MD) error {
	return nil
}

func (m *MockRunServer) SendHeader(md metadata.MD) error {
	return nil
}

func (m *MockRunServer) SetTrailer(md metadata.MD) {
}

func (m *MockRunServer) Context() context.Context {
	return context.TODO()
}

func (m *MockRunServer) SendMsg(msg any) error {
	return nil
}

func (m *MockRunServer) RecvMsg(msg any) error {
	return nil
}

// MockShutdownServer is a mock implementation of the ShutdownServer interface
type MockShutdownServer struct {
	mock.Mock
	Responses []*tgengine.ShutdownResponse
}

func (m *MockShutdownServer) Send(resp *tgengine.ShutdownResponse) error {
	m.Responses = append(m.Responses, resp)
	return nil
}

func (m *MockShutdownServer) SetHeader(md metadata.MD) error {
	return nil
}

func (m *MockShutdownServer) SendHeader(md metadata.MD) error {
	return nil
}

func (m *MockShutdownServer) SetTrailer(md metadata.MD) {
}

func (m *MockShutdownServer) Context() context.Context {
	return context.TODO()
}

func (m *MockShutdownServer) SendMsg(msg any) error {
	return nil
}

func (m *MockShutdownServer) RecvMsg(msg any) error {
	return nil
}

func TestTofuEngine_Init(t *testing.T) {
	t.Parallel()

	engine := &engine.TofuEngine{PluginCacheDir: t.TempDir()}
	mockStream := &MockInitServer{}

	err := engine.Init(&tgengine.InitRequest{}, mockStream)
	require.NoError(t, err)
	assert.Len(t, mockStream.Responses, 2)
	assert.NotNil(t, mockStream.Responses[0].GetLog())
	assert.Equal(t, "Tofu Initialization started", mockStream.Responses[0].GetLog().GetContent())
	assert.NotNil(t, mockStream.Responses[1].GetLog())
	assert.Equal(t, "Tofu Initialization completed", mockStream.Responses[1].GetLog().GetContent())
}

func TestTofuEngine_InitRejectsNonBooleanNoAutoProviderCacheDir(t *testing.T) {
	t.Parallel()

	value, err := anypb.New(structpb.NewStringValue(`"sometimes"`))
	require.NoError(t, err)

	engine := &engine.TofuEngine{PluginCacheDir: t.TempDir()}
	mockStream := &MockInitServer{}

	err = engine.Init(&tgengine.InitRequest{
		Meta: map[string]*anypb.Any{"no_auto_provider_cache_dir": value},
	}, mockStream)
	require.ErrorIs(t, err, strconv.ErrSyntax)

	require.Len(t, mockStream.Responses, 3)
	assert.Equal(t, int32(1), mockStream.Responses[2].GetExitResult().GetCode())
}

func TestTofuEngine_InitRejectsTFPathWithTofuVersion(t *testing.T) {
	t.Parallel()

	tfPath, err := anypb.New(structpb.NewStringValue(`"tofu"`))
	require.NoError(t, err)

	version, err := anypb.New(structpb.NewStringValue(`"v1.11.2"`))
	require.NoError(t, err)

	eng := &engine.TofuEngine{PluginCacheDir: t.TempDir()}
	mockStream := &MockInitServer{}

	err = eng.Init(&tgengine.InitRequest{
		Meta: map[string]*anypb.Any{"tf_path": tfPath, "tofu_version": version},
	}, mockStream)
	require.ErrorIs(t, err, engine.ErrConflictingMeta)

	require.Len(t, mockStream.Responses, 3)
	assert.Equal(t, int32(1), mockStream.Responses[2].GetExitResult().GetCode())
}

func TestTofuEngine_InitRejectsMissingTFPath(t *testing.T) {
	t.Parallel()

	missing, err := json.Marshal(filepath.Join(t.TempDir(), "tofu"))
	require.NoError(t, err)

	tfPath, err := anypb.New(structpb.NewStringValue(string(missing)))
	require.NoError(t, err)

	eng := &engine.TofuEngine{PluginCacheDir: t.TempDir()}
	mockStream := &MockInitServer{}

	err = eng.Init(&tgengine.InitRequest{
		Meta: map[string]*anypb.Any{"tf_path": tfPath},
	}, mockStream)
	require.ErrorIs(t, err, fs.ErrNotExist)

	require.Len(t, mockStream.Responses, 3)
	assert.Equal(t, int32(1), mockStream.Responses[2].GetExitResult().GetCode())
}

func TestTofuEngine_Run(t *testing.T) {
	t.Parallel()

	engine := &engine.TofuEngine{}
	mockStream := &MockRunServer{}

	cmd := "tofu"
	args := []string{"--help"}
	req := &tgengine.RunRequest{
		Command: cmd,
		Args:    args,
		EnvVars: map[string]string{"FOO": "bar"},
	}
	err := engine.Run(req, mockStream)
	require.NoError(t, err)
	assert.NotEmpty(t, mockStream.Responses)
	// merge stdout from all responses to a string
	var output strings.Builder

	for _, response := range mockStream.Responses {
		if stdout := response.GetStdout(); stdout != nil {
			output.WriteString(stdout.GetContent())
		}
	}

	assert.Contains(t, output.String(), "Usage: tofu [global options] <subcommand> [args]")
}

func TestTofuEngineError(t *testing.T) {
	t.Parallel()

	engine := &engine.TofuEngine{}
	mockStream := &MockRunServer{}

	cmd := "tofu"
	args := []string{"not-a-valid-command"}
	req := &tgengine.RunRequest{
		Command: cmd,
		Args:    args,
	}
	err := engine.Run(req, mockStream)
	require.NoError(t, err)
	assert.NotEmpty(t, mockStream.Responses)
	// merge stderr from all responses to a string
	var output strings.Builder

	for _, response := range mockStream.Responses {
		if stderr := response.GetStderr(); stderr != nil {
			output.WriteString(stderr.GetContent())
		}
	}
	// get status code from last response
	var code int32

	for i := len(mockStream.Responses) - 1; i >= 0; i-- {
		if exitResult := mockStream.Responses[i].GetExitResult(); exitResult != nil {
			code = exitResult.GetCode()
			break
		}
	}

	assert.Contains(t, output.String(), "OpenTofu has no command named \"not-a-valid-command\"")
	assert.NotEqual(t, int32(0), code)
}

func TestTofuEngine_Shutdown(t *testing.T) {
	t.Parallel()

	engine := &engine.TofuEngine{}
	mockStream := &MockShutdownServer{}

	err := engine.Shutdown(&tgengine.ShutdownRequest{}, mockStream)
	require.NoError(t, err)
	assert.Len(t, mockStream.Responses, 2)
	assert.NotNil(t, mockStream.Responses[0].GetLog())
	assert.Equal(t, "Tofu Shutdown completed", mockStream.Responses[0].GetLog().GetContent())
	assert.NotNil(t, mockStream.Responses[1].GetExitResult())
	assert.Equal(t, int32(0), mockStream.Responses[1].GetExitResult().GetCode())
}

func TestHelperProcess(t *testing.T) {
	t.Parallel()

	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	cmd := exec.CommandContext(t.Context(), os.Args[3], os.Args[4:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}
