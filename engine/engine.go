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
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/creack/pty"
	tgengine "github.com/gruntwork-io/terragrunt-engine-go/proto"
	"github.com/hashicorp/go-plugin"
	"github.com/opentofu/tofudl"
	log "github.com/sirupsen/logrus"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
	"google.golang.org/grpc"
)

const (
	wgSize          = 2
	iacCommand      = "tofu"
	errorResultCode = 1
	installDirMode  = 0755
)

type TofuEngine struct {
	tgengine.UnimplementedEngineServer
	binaryPath string
}

func (c *TofuEngine) Init(req *tgengine.InitRequest, stream tgengine.Engine_InitServer) error {
	log.Info("Init Tofu plugin")

	version := ""
	installDir := ""

	if req.GetMeta() != nil {
		if versionAny, exists := req.GetMeta()["tofu_version"]; exists {
			if stringValue := versionAny.GetValue(); stringValue != nil {
				version = string(stringValue)
			}
		}

		if installDirAny, exists := req.GetMeta()["tofu_install_dir"]; exists {
			if stringValue := installDirAny.GetValue(); stringValue != nil {
				installDir = string(stringValue)
			}
		}
	}

	if version != "" {
		log.Debugf("Downloading OpenTofu binary (version: %s)...", version)

		binaryPath, downloadErr := c.downloadOpenTofu(version, installDir)
		if downloadErr != nil {
			log.Errorf("Failed to download OpenTofu: %v\n", downloadErr)

			return downloadErr
		}

		c.binaryPath = binaryPath

		log.Debugf("OpenTofu binary downloaded to: %s\n", binaryPath)
	} else {
		c.binaryPath = iacCommand

		log.Debug("Using system OpenTofu binary (no version specified)")
	}

	log.Info("Engine Initialization completed")

	return nil
}

const (
	cacheDir             = "tofudl-cache"
	cacheTimeout         = time.Minute * 10
	artifactCacheTimeout = time.Hour * 24
)

// downloadOpenTofu downloads the OpenTofu binary and returns the path to it
func (c *TofuEngine) downloadOpenTofu(version, installDir string) (string, error) {
	dl, err := tofudl.New()
	if err != nil {
		return "", fmt.Errorf("failed to create downloader: %w", err)
	}

	cacheDir := filepath.Join(os.TempDir(), cacheDir)

	storage, err := tofudl.NewFilesystemStorage(cacheDir)
	if err != nil {
		return "", fmt.Errorf("failed to create filesystem storage: %w", err)
	}

	mirror, err := tofudl.NewMirror(
		tofudl.MirrorConfig{
			AllowStale:           true,
			APICacheTimeout:      cacheTimeout,
			ArtifactCacheTimeout: artifactCacheTimeout,
		},
		storage,
		dl,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create mirror: %w", err)
	}

	var opts []tofudl.DownloadOpt

	// Handle "latest" version using stability option, otherwise use specific version
	if version == "latest" {
		opts = append(opts, tofudl.DownloadOptMinimumStability(tofudl.StabilityStable))

		log.Debug("Downloading latest stable OpenTofu version")
	} else {
		opts = append(opts, tofudl.DownloadOptVersion(tofudl.Version(version)))

		log.Debugf("Downloading OpenTofu version: %s", version)
	}

	ctx := context.Background()

	binary, err := mirror.Download(ctx, opts...)
	if err != nil {
		return "", fmt.Errorf("failed to download OpenTofu binary: %w", err)
	}

	if installDir == "" {
		installDir = os.TempDir()
	}

	if err := os.MkdirAll(installDir, installDirMode); err != nil {
		return "", fmt.Errorf("failed to create install directory: %w", err)
	}

	binaryName := "tofu"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}

	binaryPath := filepath.Join(installDir, binaryName)

	if err := os.WriteFile(binaryPath, binary, installDirMode); err != nil {
		return "", fmt.Errorf("failed to write OpenTofu binary: %w", err)
	}

	log.Debugf("OpenTofu binary cached and installed to: %s", binaryPath)

	return binaryPath, nil
}

func (c *TofuEngine) Run(req *tgengine.RunRequest, stream tgengine.Engine_RunServer) error {
	log.Infof("Run Tofu plugin %v", req.GetWorkingDir())

	cmdPath := c.binaryPath
	if cmdPath == "" {
		cmdPath = iacCommand
	}

	cmd := exec.Command(cmdPath, req.GetArgs()...)
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
