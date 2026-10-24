package eqemuserver

import (
	"bufio"
	"encoding/json"
	"errors"
	"github.com/EQEmu/spire/internal/database"
	"github.com/EQEmu/spire/internal/eqemuserverconfig"
	"github.com/EQEmu/spire/internal/github"
	"github.com/EQEmu/spire/internal/logger"
	"github.com/EQEmu/spire/internal/pathmgmt"
	"github.com/EQEmu/spire/internal/spire"
	"github.com/EQEmu/spire/internal/unzip"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Updater struct {
	db           *database.Resolver
	logger       *logger.AppLogger
	serverconfig *eqemuserverconfig.Config
	settings     *spire.Settings
	pathmgmt     *pathmgmt.PathManagement
	unzipper     *unzip.Unzipper
}

func NewUpdater(
	db *database.Resolver,
	logger *logger.AppLogger,
	serverconfig *eqemuserverconfig.Config,
	settings *spire.Settings,
	pathmgmt *pathmgmt.PathManagement,
	unzipper *unzip.Unzipper,
) *Updater {
	return &Updater{
		db:           db,
		logger:       logger,
		serverconfig: serverconfig,
		settings:     settings,
		pathmgmt:     pathmgmt,
		unzipper:     unzipper,
	}
}

const (
	updateTypeSetting      = "SERVER_UPDATE_TYPE"
	updateTypeRelease      = "release"
	updateTypeSelfCompiled = "self-compiled"
)

// ErrReleaseInstallsDisabled is returned for any attempt to install an
// official EQEmu release. Starfall runs custom server code; a release zip
// would overwrite those binaries with stock ones. Build from source instead.
var ErrReleaseInstallsDisabled = errors.New("release installs are disabled: this server runs Starfall custom code, so update by building from source (self-compiled)")

func (u *Updater) SetUpdateType(updateType string) {
	u.settings.SetSetting(updateTypeSetting, updateType)
}

// GetUpdateType is always self-compiled; a stored "release" (set at Spire
// init or by an older build) is ignored. See ErrReleaseInstallsDisabled.
func (u *Updater) GetUpdateType() string {
	return updateTypeSelfCompiled
}

func (u *Updater) GetBuildLocation() string {
	return u.settings.GetSetting(spire.SettingBuildLocation).Value
}

type ServerVersionInfo struct {
	BotsDatabaseVersion   int    `json:"bots_database_version"`
	CompileDate           string `json:"compile_date"`
	CompileTime           string `json:"compile_time"`
	CustomDatabaseVersion int    `json:"custom_database_version"`
	DatabaseVersion       int    `json:"database_version"`
	ServerVersion         string `json:"server_version"`
}

func (u *Updater) GetVersionInfo() (ServerVersionInfo, error) {
	worldBin := u.pathmgmt.GetWorldBinPath()
	if _, err := os.Stat(worldBin); errors.Is(err, os.ErrNotExist) {
		return ServerVersionInfo{}, errors.New("Failed to find World binary to fetch version")
	}

	cmd := exec.Command(worldBin, "world:version")
	cmd.Dir = u.pathmgmt.GetEQEmuServerPath()
	output, err := cmd.Output()
	if err != nil {
		return ServerVersionInfo{}, err
	}

	// not all binaries simply output json alone
	// there was an output bug
	o := string(output)
	var n string
	startWatch := false
	for _, s := range strings.Split(o, "\n") {
		if strings.Contains(s, "{") {
			startWatch = true
		}
		if startWatch {
			n += s
		}
	}

	var v ServerVersionInfo
	err = json.Unmarshal([]byte(n), &v)
	if err != nil {
		return ServerVersionInfo{}, err
	}

	return v, err
}

// InstallRelease is disabled for Starfall; see ErrReleaseInstallsDisabled.
func (u *Updater) InstallRelease(release string) error {
	return ErrReleaseInstallsDisabled
}

type BuildInfo struct {
	SourceDirectory string `json:"source_directory"`
	BuildTool       string `json:"build_tool"`
	BuildCores      string `json:"build_cores"`
}

// GetBuildInfo tries to auto discovery source directory and returns build tool
func (u *Updater) GetBuildInfo() (BuildInfo, error) {
	var dirname string
	var err error
	s := u.settings.GetSetting("BUILD_LOCATION")
	if s.ID != 0 {
		dirname = s.Value
	}

	if len(dirname) == 0 {
		dirname, err = os.UserHomeDir()
		if err != nil {
			return BuildInfo{}, err
		}
	}

	if strings.HasPrefix(dirname, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return BuildInfo{}, err
		}
		// Replace ~ with the actual home directory path
		dirname = filepath.Join(home, strings.TrimPrefix(dirname, "~"))
	}

	foundPath := ""
	err = filepath.Walk(
		dirname,
		func(path string, info os.FileInfo, err error) error {
			if strings.Contains(path, "build/CMakeCache.txt") {
				foundPath = path
			}

			return nil
		},
	)
	if err != nil {
		return BuildInfo{}, err
	}

	buildTool := ""
	if len(foundPath) > 0 {
		file, err := os.Open(foundPath)
		if err != nil {
			return BuildInfo{}, err
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		// optionally, resize scanner's capacity for lines over 64K, see next example
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), "CMAKE_MAKE_PROGRAM:FILEPATH=") {
				buildTool = strings.ReplaceAll(scanner.Text(), "CMAKE_MAKE_PROGRAM:FILEPATH=", "")
			}
		}

		if err := scanner.Err(); err != nil {
			return BuildInfo{}, err
		}
	}

	return BuildInfo{
		SourceDirectory: filepath.Dir(foundPath),
		BuildTool:       buildTool,
		BuildCores:      u.settings.GetSetting(spire.SettingBuildCores).Value,
	}, nil
}

func (u *Updater) GetLatestReleaseVersion() (string, error) {
	// get latest version from https://github.com/EQEmu/EQEmu
	type Release struct {
		TagName string `json:"tag_name"`
	}

	// get latest release version
	resp, err := http.Get("https://api.github.com/repos/EQEmu/EQEmu/releases/latest")
	if err != nil {
		u.logger.Fatal().Err(err).Msg("could not get latest release version")
	}

	defer resp.Body.Close()

	// read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		u.logger.Fatal().Err(err).Msg("could not read response body")
	}

	err = github.HandleRateLimitBodyResponseFormatter(body, resp.Header)
	if err != nil {
		u.logger.Fatal().Err(err).Msg("could not handle rate limit")
	}

	// bind body to struct
	var release Release
	err = json.Unmarshal(body, &release)
	if err != nil {
		u.logger.Fatal().Err(err).Msg("could not unmarshal response body")
	}

	return strings.ReplaceAll(release.TagName, "v", ""), nil
}
