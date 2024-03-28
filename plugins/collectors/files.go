package collectors

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/ImDuong/vola-auto/config"
	"github.com/ImDuong/vola-auto/datastore"
	"github.com/ImDuong/vola-auto/plugins"
	"github.com/ImDuong/vola-auto/plugins/volatility/filescan"
	"github.com/alitto/pond"
)

type (
	FilesPlugin struct {
		WorkerPool *pond.WorkerPool
	}
)

func (colp *FilesPlugin) GetName() string {
	return "FILES COLLECTION PLUGIN"
}

func (colp *FilesPlugin) GetArtifactsCollectionPath() string {
	return ""
}

// Run() only processes & stores info about files in memory, not dump files
func (colp *FilesPlugin) Run() error {
	correspPlg := filescan.FilescanPlugin{}
	filescanArtifactFiles, err := os.Open(correspPlg.GetArtifactsExtractionPath())
	if err != nil {
		return err
	}
	defer filescanArtifactFiles.Close()
	scanner := bufio.NewScanner(filescanArtifactFiles)
	isProcessDataFound := false

	for scanner.Scan() {
		line := scanner.Text()

		if len(line) == 0 {
			continue
		}
		if !isProcessDataFound {
			if strings.Contains(line, "Offset") && strings.Contains(line, "Name") && strings.Contains(line, "Size") {
				isProcessDataFound = true
			}
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}

		fileInfo := datastore.FileInfo{
			Path: parts[1],
		}

		if datastore.HostInfo.Profile == "win10" {
			fileInfo.VirtualAddrOffset = parts[0]
		} else {
			fileInfo.PhysicalAddrOffset = parts[0]
		}

		datastore.FileList = append(datastore.FileList, fileInfo)
	}

	if err := scanner.Err(); err != nil {
		fmt.Println(colp.GetName(), ":got some errors when collecting artifacts")
	}
	return nil
}

func (colp *FilesPlugin) FindFilesByRegex(regex string) ([]datastore.FileInfo, error) {
	var matchingItems []datastore.FileInfo
	re, err := regexp.Compile(regex)
	if err != nil {
		return matchingItems, err
	}

	for _, fileInfo := range datastore.FileList {
		if re.MatchString(fileInfo.Path) {
			matchingItems = append(matchingItems, fileInfo)
		}
	}

	return matchingItems, nil
}

func (colp *FilesPlugin) DumpFile(dumpFile datastore.FileInfo, outputFolder string) error {
	var offset string
	var offsetTypeFlag string
	if len(dumpFile.PhysicalAddrOffset) != 0 {
		offset = dumpFile.PhysicalAddrOffset
		offsetTypeFlag = "--physaddr"
	} else if len(dumpFile.VirtualAddrOffset) != 0 {
		offset = dumpFile.VirtualAddrOffset
		offsetTypeFlag = "--virtaddr"
	}
	if len(offset) == 0 {
		return fmt.Errorf("empty offset to dump file %s", dumpFile.Path)
	}

	args := []string{config.Default.VolRunConfig.Binary,
		"-f", config.Default.MemoryDumpPath,
		"-o", outputFolder,
		"windows.dumpfiles.DumpFiles",
		offsetTypeFlag, offset,
	}
	return plugins.RunVolatilityPluginAndWriteResult(args, "", true)
}

func (colp *FilesPlugin) DumpFiles(dumpFiles []datastore.FileInfo, outputFolder string) error {
	var aggregatedError error
	var aggregateErrorMutex sync.Mutex
	taskGroup := colp.WorkerPool.Group()
	for i := range dumpFiles {
		copiedIdx := i
		taskGroup.Submit(func() {
			err := colp.DumpFile(dumpFiles[copiedIdx], outputFolder)
			if err != nil {
				aggregateErrorMutex.Lock()
				aggregatedError = fmt.Errorf("%w;%w", aggregatedError, err)
				aggregateErrorMutex.Unlock()
			}
		})
	}
	taskGroup.Wait()
	if aggregatedError != nil {
		return aggregatedError
	}
	return nil
}

func (colp *FilesPlugin) RenameDumpedFilesExtention(matchSuffix, newSuffix, outputFolder string) error {
	return filepath.Walk(outputFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), matchSuffix) {
			newName := strings.TrimSuffix(info.Name(), filepath.Ext(info.Name())) + newSuffix
			err := os.Rename(path, filepath.Join(filepath.Dir(path), newName))
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// TODO: dump all files and put them in original folder structure
func (colp *FilesPlugin) DumpAllFiles() error {
	return nil
}
