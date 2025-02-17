package plugins

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ImDuong/vola-auto/config"
	"github.com/ImDuong/vola-auto/utils"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type (
	VolPlugin interface {
		Run() error
		GetName() string
		GetArtifactsExtractionPath() string
	}

	CollectorPlugin interface {
		Run() error
		GetName() string
		GetArtifactsCollectionPath() string
	}

	AnalyticPlugin interface {
		Run() error
		GetName() string
		GetAnalyticResultPath() string
	}
)

// IsRunRequired checks if plugin is required to run, whenever one of this conditions happens:
// - user passes flags to force re-run
// - file does not exist
// - the passed path is an empty folder
func IsRunRequired(artifactExtractionFilepath string) bool {
	if config.Default.IsForcedRerun {
		return true
	}
	fileInfo, err := os.Stat(artifactExtractionFilepath)
	if os.IsNotExist(err) {
		return true
	}

	if !fileInfo.IsDir() {
		return false
	}

	f, err := os.Open(artifactExtractionFilepath)
	if err != nil {
		return true
	}
	defer f.Close()

	// the plugin does not need to re-run if the folder has at least 1 item
	_, err = f.Readdirnames(1)
	return err != nil
}

func GetFileOpenFlag(isOverride bool) int {
	perms := os.O_CREATE | os.O_WRONLY
	if !isOverride {
		perms = perms | os.O_APPEND
	} else {
		perms = perms | os.O_TRUNC
	}
	return perms
}

func RunVolatilityPluginAndWriteResult(args []string, resultFilepath string, isOverride bool) error {
	isDumpingFile := false
	if slices.Contains(args, "-o") {
		isDumpingFile = true
	}

	// allow caller to skip passing common flags when needed
	if len(args) > 0 && !strings.EqualFold(args[0], config.Default.VolRunConfig.Binary) {
		args = append([]string{config.Default.VolRunConfig.Binary, "-q", "-f", config.Default.MemoryDumpPath}, args...)
	}

	cmd := exec.Command(config.Default.VolRunConfig.Runner, args...)
	cmdLogging := utils.Logger.Debug
	if !isDumpingFile {
		if len(resultFilepath) == 0 {
			resultFilepath = filepath.Join(config.Default.BatchCmdFolder, uuid.New().String()[:8]+".txt")

			// log args with info level when running batch commands
			cmdLogging = utils.Logger.Info
		}
		outputFileWriter, err := os.OpenFile(resultFilepath, GetFileOpenFlag(isOverride), 0644)
		if err != nil {
			return fmt.Errorf("opening %s failed: %w", resultFilepath, err)
		}
		defer outputFileWriter.Close()
		cmd.Stdout = outputFileWriter
		cmd.Stderr = outputFileWriter
	}

	cmdLogging("Executing", zap.String("cmd", strings.Join(cmd.Args, " ")), zap.String("output", resultFilepath))
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}
