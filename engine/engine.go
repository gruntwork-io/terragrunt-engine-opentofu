package engine

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/gruntwork-io/terragrunt-engine-go/engine"
	"github.com/hashicorp/go-plugin"
	"github.com/kr/pty"
	log "github.com/sirupsen/logrus"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
	"google.golang.org/grpc"
)

const (
	wgSize = 2
)

type TofuCommandExecutor struct {
	engine.UnimplementedCommandExecutorServer
}

func (c *TofuCommandExecutor) Init(req *engine.InitRequest, stream engine.CommandExecutor_InitServer) error {
	log.Info("Init Tofu plugin")
	err := stream.Send(&engine.InitResponse{Stdout: "Tofu Initialization started\n", Stderr: "", ResultCode: 0})
	if err != nil {
		return err
	}

	// Stream some metadata as stdout for demonstration
	for key, value := range req.Meta {
		err := stream.Send(&engine.InitResponse{Stdout: fmt.Sprintf("\nTofu Metadata: %s = %s\n", key, value), Stderr: "", ResultCode: 0})
		if err != nil {
			return err
		}
	}

	err = stream.Send(&engine.InitResponse{Stdout: "\nTofu Initialization completed\n", Stderr: "", ResultCode: 0})
	if err != nil {
		return err
	}
	return nil
}

func (c *TofuCommandExecutor) Run(req *engine.RunRequest, stream engine.CommandExecutor_RunServer) error {
	log.Info("Run Tofu plugin")
	cmd := exec.Command(req.Command, req.Args...)
	cmd.Dir = req.WorkingDir

	env := make([]string, 0, len(req.EnvVars))
	for key, value := range req.EnvVars {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	cmd.Env = append(cmd.Env, env...)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if req.AllocatePseudoTty {
		ptmx, err := pty.Start(cmd)
		if err != nil {
			log.Errorf("Error allocating pseudo-TTY: %v", err)
			return err
		}
		defer func() { _ = ptmx.Close() }()

		go func() {
			_, _ = io.Copy(ptmx, os.Stdin)
		}()
		go func() {
			_, _ = io.Copy(os.Stdout, ptmx)
		}()
		go func() {
			_, _ = io.Copy(os.Stderr, ptmx)
		}()
	} else {
		cmd.Stdin = os.Stdin
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	var wg sync.WaitGroup

	// 2 streams to send stdout and stderr
	wg.Add(wgSize)

	// Stream stdout
	go func() {
		defer wg.Done()
		reader := transform.NewReader(stdoutPipe, unicode.UTF8.NewDecoder())
		bufReader := bufio.NewReader(reader)
		for {
			char, _, err := bufReader.ReadRune()
			if err != nil {
				if err != io.EOF {
					log.Errorf("Error reading stdout: %v", err)
				}
				break
			}
			err = stream.Send(&engine.RunResponse{
				Stdout: string(char),
			})
			if err != nil {
				log.Errorf("Error sending stdout: %v", err)
				return
			}
		}
	}()

	// Stream stderr
	go func() {
		defer wg.Done()
		reader := transform.NewReader(stderrPipe, unicode.UTF8.NewDecoder())
		bufReader := bufio.NewReader(reader)
		for {
			char, _, err := bufReader.ReadRune()
			if err != nil {
				if err != io.EOF {
					log.Errorf("Error reading stderr: %v", err)
				}
				break
			}
			err = stream.Send(&engine.RunResponse{
				Stderr: string(char),
			})
			if err != nil {
				log.Errorf("Error sending stderr: %v", err)
				return
			}
		}
	}()

	wg.Wait()
	err = cmd.Wait()
	resultCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			resultCode = exitError.ExitCode()
		} else {
			resultCode = 1
		}
	}

	err = stream.Send(&engine.RunResponse{
		ResultCode: int32(resultCode),
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *TofuCommandExecutor) Shutdown(req *engine.ShutdownRequest, stream engine.CommandExecutor_ShutdownServer) error {
	log.Info("Shutdown Tofu plugin")

	return nil
}

// GRPCServer is used to register the TofuCommandExecutor with the gRPC server
func (c *TofuCommandExecutor) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	engine.RegisterCommandExecutorServer(s, c)
	return nil
}

// GRPCClient is used to create a client that connects to the TofuCommandExecutor
func (c *TofuCommandExecutor) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, client *grpc.ClientConn) (interface{}, error) {
	return engine.NewCommandExecutorClient(client), nil
}
