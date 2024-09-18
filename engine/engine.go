// Package engine provides the implementation of Terragrunt IaC engine interface
package engine

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	tgengine "github.com/gruntwork-io/terragrunt-engine-go/proto"
	"github.com/hashicorp/go-plugin"
	"github.com/kr/pty"
	log "github.com/sirupsen/logrus"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
	"google.golang.org/grpc"
)

const (
	wgSize          = 2
	iacCommand      = "tofu"
	errorResultCode = 1
)

type TofuEngine struct {
	tgengine.UnimplementedEngineServer
}

func (c *TofuEngine) Init(req *tgengine.InitRequest, stream tgengine.Engine_InitServer) error {
	log.Info("Init Tofu plugin")

	err := stream.Send(&tgengine.InitResponse{Stdout: "Tofu Initialization started\n", Stderr: "", ResultCode: 0})
	if err != nil {
		return err
	}

	err = stream.Send(&tgengine.InitResponse{Stdout: "Tofu Initialization completed\n", Stderr: "", ResultCode: 0})
	if err != nil {
		return err
	}

	return nil
}

func (c *TofuEngine) Run(req *tgengine.RunRequest, stream tgengine.Engine_RunServer) error {
	log.Infof("Run Tofu plugin %v", req.GetWorkingDir())
	cmd := exec.Command(iacCommand, req.GetArgs()...)
	cmd.Dir = req.GetWorkingDir()

	env := make([]string, 0, len(req.GetEnvVars()))
	for key, value := range req.GetEnvVars() {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	cmd.Env = append(cmd.Env, env...)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		sendError(stream, err)
		return err
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		sendError(stream, err)
		return err
	}

	if req.GetAllocatePseudoTty() {
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
		sendError(stream, err)
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
				if !errors.Is(err, io.EOF) {
					log.Errorf("Error reading stdout: %v", err)
				}

				break
			}

			if err = stream.Send(&tgengine.RunResponse{Stdout: string(char)}); err != nil {
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
				if !errors.Is(err, io.EOF) {
					log.Errorf("Error reading stderr: %v", err)
				}

				break
			}

			if err = stream.Send(&tgengine.RunResponse{Stderr: string(char)}); err != nil {
				log.Errorf("Error sending stderr: %v", err)
				return
			}
		}
	}()
	wg.Wait()

	resultCode := 0

	if err := cmd.Wait(); err != nil {
		var exitError *exec.ExitError
		if ok := errors.As(err, &exitError); ok {
			resultCode = exitError.ExitCode()
		} else {
			resultCode = 1
		}
	}

	if err := stream.Send(&tgengine.RunResponse{ResultCode: int32(resultCode)}); err != nil {
		return err
	}

	return nil
}

func sendError(stream tgengine.Engine_RunServer, err error) {
	if err = stream.Send(&tgengine.RunResponse{Stderr: fmt.Sprintf("%v", err), ResultCode: errorResultCode}); err != nil {
		log.Warnf("Error sending response: %v", err)
	}
}

func (c *TofuEngine) Shutdown(req *tgengine.ShutdownRequest, stream tgengine.Engine_ShutdownServer) error {
	log.Info("Shutdown Tofu plugin")

	if err := stream.Send(&tgengine.ShutdownResponse{Stdout: "Tofu Shutdown completed\n", Stderr: "", ResultCode: 0}); err != nil {
		return err
	}

	return nil
}

// GRPCServer is used to register the TofuEngine with the gRPC server
func (c *TofuEngine) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	tgengine.RegisterEngineServer(s, c)
	return nil
}

// GRPCClient is used to create a client that connects to the TofuEngine
func (c *TofuEngine) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, client *grpc.ClientConn) (interface{}, error) {
	return tgengine.NewEngineClient(client), nil
}
