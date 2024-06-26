package engine

import (
	"context"
	"testing"

	tgengine "github.com/gruntwork-io/terragrunt-engine-go/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/metadata"
	"os"
	"os/exec"
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

func (m *MockInitServer) SendMsg(msg interface{}) error {
	return nil
}

func (m *MockInitServer) RecvMsg(msg interface{}) error {
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

func (m *MockRunServer) SendMsg(msg interface{}) error {
	return nil
}

func (m *MockRunServer) RecvMsg(msg interface{}) error {
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

func (m *MockShutdownServer) SendMsg(msg interface{}) error {
	return nil
}

func (m *MockShutdownServer) RecvMsg(msg interface{}) error {
	return nil
}

func TestTofuEngine_Init(t *testing.T) {
	engine := &TofuEngine{}
	mockStream := &MockInitServer{}

	err := engine.Init(&tgengine.InitRequest{}, mockStream)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(mockStream.Responses))
	assert.Equal(t, "Tofu Initialization started\n", mockStream.Responses[0].Stdout)
	assert.Equal(t, "Tofu Initialization completed\n", mockStream.Responses[1].Stdout)
}

func TestTofuEngine_Run(t *testing.T) {
	engine := &TofuEngine{}
	mockStream := &MockRunServer{}

	cmd := "echo"
	args := []string{"Hello, World!"}
	req := &tgengine.RunRequest{
		Command: cmd,
		Args:    args,
		EnvVars: map[string]string{"FOO": "bar"},
	}
	err := engine.Run(req, mockStream)
	assert.NoError(t, err)
	assert.True(t, len(mockStream.Responses) > 0)
	// merge stdout from all responses to a string
	var output string
	for _, response := range mockStream.Responses {
		if response.Stdout != "" {
			output += response.Stdout
		}
	}
	assert.Contains(t, output, "Hello, World!")
}

func TestTofuEngine_Shutdown(t *testing.T) {
	engine := &TofuEngine{}
	mockStream := &MockShutdownServer{}

	err := engine.Shutdown(&tgengine.ShutdownRequest{}, mockStream)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(mockStream.Responses))
	assert.Equal(t, "Tofu Shutdown completed\n", mockStream.Responses[0].Stdout)
}

func TestHelperProcess(*testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	cmd := exec.Command(os.Args[3], os.Args[4:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}
