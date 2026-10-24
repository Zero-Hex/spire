package eqemuserver

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	sourceRootMu    sync.Mutex
	sourceRootCache string
)

// GetSourceRoot is the server source checkout the build was configured from,
// or "" when none is found. GetBuildInfo walks the home folder for the build,
// so a found root is remembered.
func (u *Updater) GetSourceRoot() string {
	sourceRootMu.Lock()
	defer sourceRootMu.Unlock()
	if sourceRootCache != "" {
		return sourceRootCache
	}

	info, err := u.GetBuildInfo()
	if err != nil || info.SourceDirectory == "" {
		return ""
	}
	sourceRootCache = SourceRootFromCMakeCache(filepath.Join(info.SourceDirectory, "CMakeCache.txt"))
	return sourceRootCache
}

// SourceRootFromCMakeCache reads the source directory a build was configured
// from (CMAKE_HOME_DIRECTORY); the build directory need not sit inside it
func SourceRootFromCMakeCache(cachePath string) string {
	file, err := os.Open(cachePath)
	if err != nil {
		return ""
	}
	defer file.Close()

	const key = "CMAKE_HOME_DIRECTORY:INTERNAL="
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), key) {
			return strings.TrimSpace(strings.TrimPrefix(scanner.Text(), key))
		}
	}

	return ""
}
