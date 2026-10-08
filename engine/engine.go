// Package engine provides the implementation of Terragrunt IaC engine interface
package engine

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
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
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	wgSize          = 2
	iacCommand      = "tofu"
	latestVersion   = "latest"
	errorResultCode = 1
	installDirMode  = 0755

	envPluginCacheDir = "TF_PLUGIN_CACHE_DIR"
	// metaNoAutoProviderCacheDir matches Terragrunt's --no-auto-provider-cache-dir flag.
	metaNoAutoProviderCacheDir = "no_auto_provider_cache_dir"

	versionProbeTimeout = 30 * time.Second
	versionProbeMaxSize = 1 << 20

	// OpenTofu locks the plugin cache from 1.10 on. Older versions corrupt it when
	// several runs install the same provider at once.
	pluginCacheMinMajor = 1
	pluginCacheMinMinor = 10
)

// ErrPluginCacheUnsupported reports an OpenTofu version that cannot share a plugin cache between concurrent runs.
var ErrPluginCacheUnsupported = errors.New(
	"OpenTofu older than 1.10 cannot share a plugin cache between concurrent runs",
)

// TofuEngine runs OpenTofu on the machine Terragrunt runs on.
type TofuEngine struct {
	tgengine.UnimplementedEngineServer

	// PluginCacheDir is the provider cache that every run of this engine shares.
	// Empty means the providers directory in Terragrunt's user cache directory.
	PluginCacheDir string

	binaryPath        string
	runPluginCacheDir string
	mu                sync.RWMutex
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

// setRunPluginCacheDir safely sets the plugin cache directory that Run passes to OpenTofu
func (c *TofuEngine) setRunPluginCacheDir(dir string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.runPluginCacheDir = dir
}

// getRunPluginCacheDir safely gets the plugin cache directory that Run passes to OpenTofu
func (c *TofuEngine) getRunPluginCacheDir() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.runPluginCacheDir
}

// Init picks the OpenTofu binary for later runs, downloading it when the meta names a tofu_version.
//
// When that binary is OpenTofu 1.10 or newer, later runs share one provider cache, so units
// running in parallel do not each download their own copy of a provider. A true
// no_auto_provider_cache_dir in the meta turns the shared cache off.
//
// Returns an error wrapping [strconv.ErrSyntax] when no_auto_provider_cache_dir is not a boolean.
func (c *TofuEngine) Init(req *tgengine.InitRequest, stream tgengine.Engine_InitServer) error {
	log.Debug("Init Tofu plugin")

	if err := stream.Send(&tgengine.InitResponse{
		Response: &tgengine.InitResponse_Log{
			Log: &tgengine.LogMessage{
				Content: "Tofu Initialization started",
				Level:   tgengine.LogLevel_LOG_LEVEL_DEBUG,
			},
		},
	}); err != nil {
		return err
	}

	noAutoProviderCacheDir, err := metaBool(req.GetMeta(), metaNoAutoProviderCacheDir)
	if err != nil {
		return failInit(stream, err)
	}

	version := metaString(req.GetMeta()["tofu_version"])
	installDir := metaString(req.GetMeta()["tofu_install_dir"])

	if version != "" {
		log.Debugf("Downloading OpenTofu binary (version: %s)...", version)

		binaryPath, downloadErr := c.downloadOpenTofu(version, installDir)
		if downloadErr != nil {
			log.Errorf("Failed to download OpenTofu: %v\n", downloadErr)

			return failInit(stream, downloadErr)
		}

		c.setBinaryPath(binaryPath)

		log.Debugf("OpenTofu binary downloaded to: %s\n", binaryPath)
	} else {
		c.setBinaryPath(iacCommand)

		log.Debug("Using system OpenTofu binary (no version specified)")
	}

	pluginCacheDir := ""

	if !noAutoProviderCacheDir {
		pluginCacheDir, err = c.sharedPluginCacheDir(stream.Context(), version)
		if err != nil {
			log.Debugf("Runs do not share a plugin cache: %v", err)
		}
	}

	c.setRunPluginCacheDir(pluginCacheDir)

	log.Debug("Engine Initialization completed")

	if err := stream.Send(&tgengine.InitResponse{
		Response: &tgengine.InitResponse_Log{
			Log: &tgengine.LogMessage{
				Content: "Tofu Initialization completed",
				Level:   tgengine.LogLevel_LOG_LEVEL_DEBUG,
			},
		},
	}); err != nil {
		return err
	}

	return nil
}

// failInit reports initErr to Terragrunt as an error log and a failed exit result, then returns it.
func failInit(stream tgengine.Engine_InitServer, initErr error) error {
	if err := stream.Send(&tgengine.InitResponse{
		Response: &tgengine.InitResponse_Log{
			Log: &tgengine.LogMessage{
				Content: initErr.Error(),
				Level:   tgengine.LogLevel_LOG_LEVEL_ERROR,
			},
		},
	}); err != nil {
		return err
	}

	if err := stream.Send(&tgengine.InitResponse{
		Response: &tgengine.InitResponse_ExitResult{
			ExitResult: &tgengine.ExitResultMessage{Code: errorResultCode},
		},
	}); err != nil {
		return err
	}

	return initErr
}

// metaBool returns the boolean value of the engine meta entry named key, and false when the meta has no such entry.
func metaBool(meta map[string]*anypb.Any, key string) (bool, error) {
	raw := metaString(meta[key])
	if raw == "" {
		return false, nil
	}

	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("invalid %s in engine meta: %w", key, err)
	}

	return value, nil
}

// metaString returns the string value of an engine meta entry.
//
// Terragrunt sends each meta value as a JSON-encoded string wrapped in a google.protobuf.Value
// (see ConvertMetaToProtobuf in Terragrunt). Values without that wrapper are read as raw bytes.
func metaString(value *anypb.Any) string {
	if value == nil {
		return ""
	}

	var protoValue structpb.Value
	if err := value.UnmarshalTo(&protoValue); err != nil {
		return string(value.GetValue())
	}

	raw := protoValue.GetStringValue()

	var decoded string
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return raw
	}

	return decoded
}

// sharedPluginCacheDir returns the plugin cache directory for the binary Init picked, creating it if needed.
//
// Returns [ErrPluginCacheUnsupported] when the binary is older than OpenTofu 1.10.
func (c *TofuEngine) sharedPluginCacheDir(
	ctx context.Context,
	requestedVersion string,
) (string, error) {
	major, minor, err := c.binaryVersion(ctx, requestedVersion)
	if err != nil {
		return "", err
	}

	if major < pluginCacheMinMajor ||
		(major == pluginCacheMinMajor && minor < pluginCacheMinMinor) {
		return "", fmt.Errorf("%w: found %d.%d", ErrPluginCacheUnsupported, major, minor)
	}

	dir := c.PluginCacheDir
	if dir == "" {
		userCacheDir, err := os.UserCacheDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user cache directory: %w", err)
		}

		// Terragrunt's default provider cache directory, so runs with and without the engine share downloads.
		dir = filepath.Join(userCacheDir, "terragrunt", "providers")
	}

	if err := os.MkdirAll(dir, installDirMode); err != nil {
		return "", fmt.Errorf("failed to create plugin cache directory: %w", err)
	}

	return dir, nil
}

// binaryVersion returns the major and minor version of the binary Init picked.
//
// It reads them from requestedVersion when the binary sits in the default install directory
// for that version, and asks the binary otherwise.
func (c *TofuEngine) binaryVersion(
	ctx context.Context,
	requestedVersion string,
) (major, minor int, err error) {
	binaryPath := c.getBinaryPath()

	if requestedVersion == "" || requestedVersion == latestVersion {
		return tofuVersion(ctx, binaryPath)
	}

	// Any other directory can already have a binary of another version, which the
	// download step reuses as it is.
	defaultBinDir, err := getDefaultBinDir(requestedVersion)
	if err != nil || filepath.Dir(binaryPath) != defaultBinDir {
		return tofuVersion(ctx, binaryPath)
	}

	return parseMajorMinor(normalizeVersion(requestedVersion))
}

// tofuVersion returns the major and minor version that the binary at binaryPath reports.
func tofuVersion(ctx context.Context, binaryPath string) (major, minor int, err error) {
	ctx, cancel := context.WithTimeout(ctx, versionProbeTimeout)
	defer cancel()

	var stdout bytes.Buffer

	cmd := exec.CommandContext(ctx, binaryPath, "version", "-json")
	cmd.Stdout = &limitedWriter{w: &stdout, remaining: versionProbeMaxSize}

	if err := cmd.Run(); err != nil {
		return 0, 0, fmt.Errorf("failed to run %s version: %w", iacCommand, err)
	}

	var output struct {
		Version string `json:"terraform_version"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		return 0, 0, fmt.Errorf("failed to parse %s version output: %w", iacCommand, err)
	}

	return parseMajorMinor(output.Version)
}

// parseMajorMinor returns the major and minor numbers of a version such as 1.10.3.
func parseMajorMinor(version string) (major, minor int, err error) {
	if _, err := fmt.Sscanf(version, "%d.%d", &major, &minor); err != nil {
		return 0, 0, fmt.Errorf("failed to parse %s version %q: %w", iacCommand, version, err)
	}

	return major, minor, nil
}

// limitedWriter writes to w until remaining reaches zero, then fails with [io.ErrShortWrite].
type limitedWriter struct {
	w         io.Writer
	remaining int
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if len(p) > l.remaining {
		return 0, io.ErrShortWrite
	}

	l.remaining -= len(p)

	return l.w.Write(p)
}

const (
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
			log.Warnf(
				"Failed to acquire blocking download lock, continuing without locking: %v",
				err,
			)

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

var ErrFailedToDownload = errors.New("failed to download OpenTofu")

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
	if version == latestVersion {
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
		return "", fmt.Errorf("%w: %w", ErrFailedToDownload, err)
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
	log.Debugf("Run Tofu plugin %v", req.GetWorkingDir())

	if err := stream.Send(&tgengine.RunResponse{
		Response: &tgengine.RunResponse_Log{
			Log: &tgengine.LogMessage{
				Content: "Tofu Run started in (" + req.GetWorkingDir() + "): " + strings.Join(
					append(
						[]string{req.GetCommand()}, req.GetArgs()...,
					),
					" ",
				),
				Level: tgengine.LogLevel_LOG_LEVEL_DEBUG,
			},
		},
	}); err != nil {
		return err
	}

	cmdPath := c.getBinaryPath()
	if cmdPath == "" {
		cmdPath = iacCommand
	}

	ctx := context.TODO()

	cmd := exec.CommandContext(ctx, cmdPath, req.GetArgs()...)
	cmd.Dir = req.GetWorkingDir()

	env := make([]string, 0, len(req.GetEnvVars()))
	for key, value := range req.GetEnvVars() {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	// Terragrunt's provider cache server sends the variable empty to turn tofu's own
	// cache off, so a request that names it at all keeps its value.
	if _, ok := req.GetEnvVars()[envPluginCacheDir]; !ok {
		if dir := c.getRunPluginCacheDir(); dir != "" {
			env = append(env, envPluginCacheDir+"="+dir)
		}
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

			if err = stream.Send(&tgengine.RunResponse{
				Response: &tgengine.RunResponse_Stdout{
					Stdout: &tgengine.StdoutMessage{Content: string(char)},
				},
			}); err != nil {
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

			if err = stream.Send(&tgengine.RunResponse{
				Response: &tgengine.RunResponse_Stderr{
					Stderr: &tgengine.StderrMessage{Content: string(char)},
				},
			}); err != nil {
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

	if err := stream.Send(&tgengine.RunResponse{
		Response: &tgengine.RunResponse_ExitResult{
			ExitResult: &tgengine.ExitResultMessage{Code: int32(resultCode)},
		},
	}); err != nil {
		return err
	}

	return nil
}

func sendError(stream tgengine.Engine_RunServer, err error) {
	if sendErr := stream.Send(&tgengine.RunResponse{
		Response: &tgengine.RunResponse_Log{
			Log: &tgengine.LogMessage{
				Content: fmt.Sprintf("%v", err),
				Level:   tgengine.LogLevel_LOG_LEVEL_ERROR,
			},
		},
	}); sendErr != nil {
		log.Warnf("Error sending stderr response: %v", sendErr)
	}

	if sendErr := stream.Send(&tgengine.RunResponse{
		Response: &tgengine.RunResponse_ExitResult{
			ExitResult: &tgengine.ExitResultMessage{Code: errorResultCode},
		},
	}); sendErr != nil {
		log.Warnf("Error sending exit result response: %v", sendErr)
	}
}

func (c *TofuEngine) Shutdown(
	req *tgengine.ShutdownRequest,
	stream tgengine.Engine_ShutdownServer,
) error {
	log.Debug("Shutdown Tofu plugin")

	if err := stream.Send(&tgengine.ShutdownResponse{
		Response: &tgengine.ShutdownResponse_Log{
			Log: &tgengine.LogMessage{
				Content: "Tofu Shutdown completed",
				Level:   tgengine.LogLevel_LOG_LEVEL_DEBUG,
			},
		},
	}); err != nil {
		return err
	}

	if err := stream.Send(&tgengine.ShutdownResponse{
		Response: &tgengine.ShutdownResponse_ExitResult{
			ExitResult: &tgengine.ExitResultMessage{Code: 0},
		},
	}); err != nil {
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
func (c *TofuEngine) GRPCClient(
	ctx context.Context,
	broker *plugin.GRPCBroker,
	client *grpc.ClientConn,
) (any, error) {
	return tgengine.NewEngineClient(client), nil
}
