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
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/gofrs/flock"
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
	mu         sync.RWMutex
	binaryPath string
}

// setBinaryPath safely sets the binary path
func (c *TofuEngine) setBinaryPath(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.binaryPath = path
}

// getBinaryPath safely gets the binary path
func (c *TofuEngine) getBinaryPath() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.binaryPath
}

func (c *TofuEngine) Init(req *tgengine.InitRequest, stream tgengine.Engine_InitServer) error {
	log.Info("Init Tofu plugin")

	if err := stream.Send(&tgengine.InitResponse{Stdout: "Tofu Initialization started\n"}); err != nil {
		return err
	}

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

		c.setBinaryPath(binaryPath)

		log.Debugf("OpenTofu binary downloaded to: %s\n", binaryPath)
	} else {
		c.setBinaryPath(iacCommand)

		log.Debug("Using system OpenTofu binary (no version specified)")
	}

	log.Info("Engine Initialization completed")

	if err := stream.Send(&tgengine.InitResponse{Stdout: "Tofu Initialization completed\n"}); err != nil {
		return err
	}

	return nil
}

const (
	cacheDir             = "tofudl-cache"
	cacheTimeout         = time.Minute * 10
	artifactCacheTimeout = time.Hour * 24
)

// getDefaultCacheDir returns the default cache directory following Terragrunt's pattern
func getDefaultCacheDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	cacheDir := filepath.Join(homeDir, ".cache", "terragrunt", "tofudl", "cache")

	return cacheDir, nil
}

// getDefaultBinDir returns the default binary directory for a specific version
func getDefaultBinDir(version string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	binDir := filepath.Join(homeDir, ".cache", "terragrunt", "tofudl", "bin", version)

	return binDir, nil
}

// normalizeVersion strips the leading 'v' from version strings if present
// This is needed because tofudl expects versions without the 'v' prefix
func normalizeVersion(version string) string {
	return strings.TrimPrefix(version, "v")
}

// getDefaultLockDir returns the default lock directory for file locking
func getDefaultLockDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	lockDir := filepath.Join(homeDir, ".cache", "terragrunt", "tofudl", "locks")

	if err := os.MkdirAll(lockDir, installDirMode); err != nil {
		return "", fmt.Errorf("failed to create lock directory: %w", err)
	}

	return lockDir, nil
}

// getLockFilePath returns the lock file path for a specific version
func getLockFilePath() (string, error) {
	lockDir, err := getDefaultLockDir()
	if err != nil {
		return "", err
	}

	// Use a single global lock file to prevent tofudl library race conditions
	// The race condition occurs in tofudl.New() which affects a global config,
	// so we need to serialize all downloads regardless of version
	lockFileName := "global-download.lock"
	return filepath.Join(lockDir, lockFileName), nil
}

// downloadOpenTofu downloads the OpenTofu binary and returns the path to it
func (c *TofuEngine) downloadOpenTofu(version, installDir string) (string, error) {
	lockFilePath, err := getLockFilePath()
	if err != nil {
		log.Warnf("Failed to get lock file path, continuing without locking: %v", err)
		return c.downloadOpenTofuUnsafe(version, installDir)
	}

	fileLock := flock.New(lockFilePath)

	log.Debugf("Acquiring download lock for OpenTofu version %s: %s", version, lockFilePath)

	locked, err := fileLock.TryLock()
	if err != nil {
		log.Warnf("Failed to acquire download lock, continuing without locking: %v", err)
		return c.downloadOpenTofuUnsafe(version, installDir)
	}

	if !locked {
		log.Debug("Download lock is held by another process, waiting...")

		err = fileLock.Lock()
		if err != nil {
			log.Warnf("Failed to acquire blocking download lock, continuing without locking: %v", err)
			return c.downloadOpenTofuUnsafe(version, installDir)
		}
	}

	log.Debugf("Acquired download lock for OpenTofu version %s", version)

	defer func() {
		if unlockErr := fileLock.Unlock(); unlockErr != nil {
			log.Warnf("Failed to release download lock: %v", unlockErr)
		} else {
			log.Debugf("Released download lock for OpenTofu version %s", version)
		}
	}()

	return c.downloadOpenTofuUnsafe(version, installDir)
}

// downloadOpenTofuUnsafe performs the actual download without locking
// This is separated to allow fallback when locking fails
func (c *TofuEngine) downloadOpenTofuUnsafe(version, installDir string) (string, error) {
	dl, err := tofudl.New()
	if err != nil {
		return "", fmt.Errorf("failed to create downloader: %w", err)
	}

	cacheDir, err := getDefaultCacheDir()
	if err != nil {
		log.Warnf("Failed to get default cache directory, falling back to temp: %v", err)

		cacheDir = filepath.Join(os.TempDir(), "tofudl-cache")
	}

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
		normalizedVersion := normalizeVersion(version)
		opts = append(opts, tofudl.DownloadOptVersion(tofudl.Version(normalizedVersion)))
		log.Debugf("Downloading OpenTofu version: %s (normalized: %s)", version, normalizedVersion)
	}

	ctx := context.Background()

	binary, err := mirror.Download(ctx, opts...)
	if err != nil {
		return "", fmt.Errorf("failed to download OpenTofu binary: %w", err)
	}

	// Use versioned bin directory if installDir not specified
	if installDir == "" {
		installDir, err = getDefaultBinDir(version)
		if err != nil {
			log.Warnf("Failed to get default bin directory, falling back to temp: %v", err)

			installDir = os.TempDir()
		}
	}

	if err := os.MkdirAll(installDir, installDirMode); err != nil {
		return "", fmt.Errorf("failed to create install directory: %w", err)
	}

	binaryName := "tofu"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}

	binaryPath := filepath.Join(installDir, binaryName)

	if info, err := os.Stat(binaryPath); err == nil && info.Size() > 0 {
		log.Debugf("OpenTofu binary already exists at: %s", binaryPath)
		return binaryPath, nil
	}

	if err := os.WriteFile(binaryPath, binary, installDirMode); err != nil {
		return "", fmt.Errorf("failed to write OpenTofu binary: %w", err)
	}

	log.Debugf("OpenTofu binary cached and installed to: %s", binaryPath)

	return binaryPath, nil
}

func (c *TofuEngine) Run(req *tgengine.RunRequest, stream tgengine.Engine_RunServer) error {
	log.Infof("Run Tofu plugin %v", req.GetWorkingDir())

	cmdPath := c.getBinaryPath()
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
