package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"atlasnote/internal/config"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type StorageLocationKind string

const (
	StorageLocationDataRoot   StorageLocationKind = "data"
	StorageLocationBackupRoot StorageLocationKind = "backup"
)

type StorageLocationStatus struct {
	DataRoot              string `json:"dataRoot,omitempty"`
	BackupRoot            string `json:"backupRoot,omitempty"`
	Source                string `json:"source,omitempty"`
	EnvironmentOverride   bool   `json:"environmentOverride"`
	SetupRequired         bool   `json:"setupRequired"`
	PendingRestart        bool   `json:"pendingRestart"`
	PendingDataRoot       string `json:"pendingDataRoot,omitempty"`
	PendingBackupRoot     string `json:"pendingBackupRoot,omitempty"`
	DataRootChangeAllowed bool   `json:"dataRootChangeAllowed"`
}

type StorageLocationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type StorageLocationStatusResult struct {
	Status *StorageLocationStatus `json:"status,omitempty"`
	Error  *StorageLocationError  `json:"error,omitempty"`
}

type StorageLocationSelectionResult struct {
	Kind     string                 `json:"kind"`
	Path     string                 `json:"path,omitempty"`
	Probe    *config.RootProbe      `json:"probe,omitempty"`
	Status   *StorageLocationStatus `json:"status,omitempty"`
	Error    *StorageLocationError  `json:"error,omitempty"`
	Canceled bool                   `json:"canceled"`
}

type StorageLocationMutationResult struct {
	Status          *StorageLocationStatus `json:"status,omitempty"`
	RestartRequired bool                   `json:"restartRequired"`
	Error           *StorageLocationError  `json:"error,omitempty"`
}

const (
	storageLocationErrorUnavailable = "STORAGE_LOCATION_UNAVAILABLE"
	storageLocationErrorValidation  = "STORAGE_LOCATION_VALIDATION_FAILED"
	storageLocationErrorEnvironment = "STORAGE_LOCATION_ENVIRONMENT_LOCKED"
	storageLocationErrorMigration   = "STORAGE_LOCATION_MIGRATION_PENDING"
	storageLocationErrorRestore     = "STORAGE_LOCATION_RESTORE_PENDING"
)

func storageLocationError(err error) *StorageLocationError {
	if err == nil {
		return nil
	}
	code := storageLocationErrorUnavailable
	message := "保存場所を利用できませんでした。現在のデータは変更していません。"
	switch {
	case errors.Is(err, config.ErrRootInvalid), errors.Is(err, config.ErrLocationsInvalid):
		code = storageLocationErrorValidation
		message = "選択したフォルダを保存場所として利用できません。空のフォルダ、またはAtlas Noteの保存場所を選択してください。"
	case errors.Is(err, errStorageLocationEnvironment):
		code = storageLocationErrorEnvironment
		message = "ATLAS_NOTE_DATA_DIR が設定されているため、保存場所は変更できません。"
	case errors.Is(err, errStorageLocationMigration):
		code = storageLocationErrorMigration
		message = "保存場所の変更が次回起動を待っています。先にAtlas Noteを再起動してください。"
	case errors.Is(err, errStorageLocationRestorePending):
		code = storageLocationErrorRestore
		message = "復元待機中は保存場所を変更できません。復元を適用または取り消してから再試行してください。"
	}
	return &StorageLocationError{Code: code, Message: message}
}

var (
	errStorageLocationEnvironment    = errors.New("storage location is controlled by environment")
	errStorageLocationMigration      = errors.New("storage location migration is already pending")
	errStorageLocationUnavailable    = errors.New("storage location is unavailable")
	errStorageLocationRestorePending = errors.New("restore is pending")
)

func (a *App) GetStorageLocationStatus() StorageLocationStatusResult {
	status, err := a.storageLocationStatus()
	if err != nil {
		return StorageLocationStatusResult{Error: storageLocationError(err)}
	}
	return StorageLocationStatusResult{Status: &status}
}

func (a *App) SelectStorageLocation(kind string) StorageLocationSelectionResult {
	locationKind := StorageLocationKind(strings.TrimSpace(kind))
	if locationKind != StorageLocationDataRoot && locationKind != StorageLocationBackupRoot {
		return StorageLocationSelectionResult{Kind: kind, Error: storageLocationError(config.ErrRootInvalid)}
	}
	if a.locationResolution.Environment || strings.TrimSpace(os.Getenv("ATLAS_NOTE_DATA_DIR")) != "" {
		return StorageLocationSelectionResult{Kind: string(locationKind), Error: storageLocationError(errStorageLocationEnvironment)}
	}
	ctx := a.operationContext()
	options := runtime.OpenDialogOptions{}
	if current := a.storageLocationPath(locationKind); current != "" {
		if info, err := os.Stat(current); err == nil && info.IsDir() {
			options.DefaultDirectory = current
		}
	}
	if locationKind == StorageLocationDataRoot {
		options.Title = "保存領域を選択"
	} else {
		options.Title = "バックアップ保存領域を選択"
	}
	path, err := a.openStorageDirectory(ctx, options)
	if err != nil {
		return StorageLocationSelectionResult{Kind: string(locationKind), Error: storageLocationError(err)}
	}
	if strings.TrimSpace(path) == "" {
		status, _ := a.storageLocationStatus()
		return StorageLocationSelectionResult{Kind: string(locationKind), Status: &status, Canceled: true}
	}
	path = filepath.Clean(path)
	var probe config.RootProbe
	if locationKind == StorageLocationDataRoot {
		probe, err = config.ProbeDataRoot(path)
	} else {
		probe, err = config.ProbeBackupRoot(path)
		if err != nil && a.isCurrentDataRoot(path) {
			// The default archive is the data root itself. In that layout the
			// directory also contains the database/catalog, so it cannot pass the
			// archive-only probe; validate it as a data root instead.
			probe, err = config.ProbeDataRoot(path)
		}
	}
	if err != nil {
		return StorageLocationSelectionResult{Kind: string(locationKind), Path: path, Error: storageLocationError(err)}
	}
	path = probe.Path
	a.locationMu.Lock()
	if locationKind == StorageLocationDataRoot {
		a.pendingDataRoot = path
		archiveRoot := a.archiveRoot
		if archiveRoot == "" {
			archiveRoot = a.locationResolution.Locations.BackupRoot
		}
		managementRoot := a.managementRoot
		if managementRoot == "" {
			managementRoot = a.locationResolution.Locations.DataRoot
		}
		if a.pendingBackupFollowsData || (a.pendingBackupRoot == "" && filepath.Clean(archiveRoot) == filepath.Clean(managementRoot)) {
			a.pendingBackupRoot = path
			a.pendingBackupFollowsData = true
		}
	} else {
		a.pendingBackupRoot = path
		a.pendingBackupFollowsData = false
	}
	a.locationMu.Unlock()
	status, _ := a.storageLocationStatus()
	return StorageLocationSelectionResult{Kind: string(locationKind), Path: path, Probe: &probe, Status: &status}
}

func (a *App) isCurrentDataRoot(path string) bool {
	path = filepath.Clean(path)
	a.locationMu.Lock()
	defer a.locationMu.Unlock()
	dataRoot := a.pendingDataRoot
	if dataRoot == "" {
		dataRoot = a.managementRoot
	}
	if dataRoot == "" {
		dataRoot = a.locationResolution.Locations.DataRoot
	}
	return strings.TrimSpace(dataRoot) != "" && filepath.Clean(dataRoot) == path
}

func (a *App) ApplyStorageLocations() StorageLocationMutationResult {
	if a.startupPhase != StartupPhaseSetupRequired && a.startupPhase != StartupPhaseReady {
		return StorageLocationMutationResult{Error: storageLocationError(errStorageLocationUnavailable)}
	}
	if a.locationResolution.Environment || strings.TrimSpace(os.Getenv("ATLAS_NOTE_DATA_DIR")) != "" {
		return StorageLocationMutationResult{Error: storageLocationError(errStorageLocationEnvironment)}
	}
	if a.startupPhase == StartupPhaseReady && a.backupService != nil {
		backupStatus, err := a.backupService.Status(a.operationContext())
		if err != nil {
			return StorageLocationMutationResult{Error: storageLocationError(err)}
		}
		if backupStatus.PendingRestore {
			return StorageLocationMutationResult{Error: storageLocationError(errStorageLocationRestorePending)}
		}
	}
	a.locationMu.Lock()
	dataRoot := a.pendingDataRoot
	backupRoot := a.pendingBackupRoot
	a.locationMu.Unlock()
	if dataRoot == "" {
		dataRoot = a.managementRoot
	}
	if strings.TrimSpace(dataRoot) == "" {
		if defaultRoot, err := config.DefaultDataRoot(); err == nil {
			dataRoot = defaultRoot
		}
	}
	if backupRoot == "" {
		// The default archive follows the data root. Keep an explicitly
		// configured external archive when only the data root is changed.
		if a.startupPhase == StartupPhaseSetupRequired || a.archiveRoot == "" || filepath.Clean(a.archiveRoot) == filepath.Clean(a.managementRoot) {
			backupRoot = dataRoot
		} else {
			backupRoot = a.archiveRoot
		}
	}
	if backupRoot == "" {
		backupRoot = dataRoot
	}
	dataRoot = filepath.Clean(dataRoot)
	backupRoot = filepath.Clean(backupRoot)
	if err := config.ValidateStorageLocations(config.StorageLocations{Version: 1, DataRoot: dataRoot, BackupRoot: backupRoot}); err != nil {
		return StorageLocationMutationResult{Error: storageLocationError(err)}
	}

	if a.startupPhase == StartupPhaseSetupRequired {
		if err := config.SaveStorageLocations(config.StorageLocations{Version: 1, DataRoot: dataRoot, BackupRoot: backupRoot}); err != nil {
			return StorageLocationMutationResult{Error: storageLocationError(err)}
		}
		a.locationResolution = config.LocationResolution{
			Locations: config.StorageLocations{Version: 1, DataRoot: dataRoot, BackupRoot: backupRoot},
			Source:    config.LocationSourceSaved,
		}
		a.managementRoot, a.archiveRoot = dataRoot, backupRoot
		a.locationMu.Lock()
		a.pendingDataRoot, a.pendingBackupRoot = "", ""
		a.pendingBackupFollowsData = false
		a.locationMu.Unlock()
		status, _ := a.storageLocationStatus()
		return StorageLocationMutationResult{Status: &status, RestartRequired: true}
	}

	if pending, err := config.LoadPendingStorageLocationMigration(); err == nil {
		_ = pending
		return StorageLocationMutationResult{Error: storageLocationError(errStorageLocationMigration)}
	} else if !errors.Is(err, os.ErrNotExist) {
		return StorageLocationMutationResult{Error: storageLocationError(err)}
	}

	currentDataRoot := filepath.Clean(a.managementRoot)
	currentBackupRoot := filepath.Clean(a.archiveRoot)
	if currentBackupRoot == "." || currentBackupRoot == "" {
		currentBackupRoot = currentDataRoot
	}
	if dataRoot == currentDataRoot && backupRoot == currentBackupRoot {
		a.locationMu.Lock()
		a.pendingDataRoot, a.pendingBackupRoot = "", ""
		a.pendingBackupFollowsData = false
		a.locationMu.Unlock()
		status, _ := a.storageLocationStatus()
		return StorageLocationMutationResult{Status: &status}
	}
	migration := config.PendingStorageLocationMigration{
		Version: 1, ID: strconv.FormatInt(time.Now().UnixNano(), 10),
		SourceDataRoot: currentDataRoot, TargetDataRoot: dataRoot,
		SourceBackupRoot: currentBackupRoot, TargetBackupRoot: backupRoot,
	}
	if err := config.SavePendingStorageLocationMigration(migration); err != nil {
		return StorageLocationMutationResult{Error: storageLocationError(err)}
	}
	a.locationMu.Lock()
	a.pendingDataRoot, a.pendingBackupRoot = dataRoot, backupRoot
	a.pendingBackupFollowsData = false
	a.locationMu.Unlock()
	status, _ := a.storageLocationStatus()
	return StorageLocationMutationResult{Status: &status, RestartRequired: true}
}

func (a *App) CancelStorageLocationSelection() StorageLocationStatusResult {
	a.locationMu.Lock()
	a.pendingDataRoot, a.pendingBackupRoot = "", ""
	a.pendingBackupFollowsData = false
	a.locationMu.Unlock()
	status, err := a.storageLocationStatus()
	if err != nil {
		return StorageLocationStatusResult{Error: storageLocationError(err)}
	}
	return StorageLocationStatusResult{Status: &status}
}

func (a *App) storageLocationPath(kind StorageLocationKind) string {
	a.locationMu.Lock()
	defer a.locationMu.Unlock()
	if kind == StorageLocationDataRoot && a.pendingDataRoot != "" {
		return a.pendingDataRoot
	}
	if kind == StorageLocationBackupRoot && a.pendingBackupRoot != "" {
		return a.pendingBackupRoot
	}
	if kind == StorageLocationDataRoot {
		return a.managementRoot
	}
	return a.archiveRoot
}

func (a *App) openStorageDirectory(ctx context.Context, options runtime.OpenDialogOptions) (string, error) {
	if a.openDirectory != nil {
		return a.openDirectory(ctx, options)
	}
	return runtime.OpenDirectoryDialog(ctx, options)
}

func (a *App) storageLocationStatus() (StorageLocationStatus, error) {
	a.locationMu.Lock()
	defer a.locationMu.Unlock()
	dataRoot := a.managementRoot
	backupRoot := a.archiveRoot
	if dataRoot == "" {
		dataRoot = a.locationResolution.Locations.DataRoot
	}
	if backupRoot == "" {
		backupRoot = a.locationResolution.Locations.BackupRoot
	}
	if backupRoot == "" {
		backupRoot = dataRoot
	}
	environmentOverride := a.locationResolution.Environment || strings.TrimSpace(os.Getenv("ATLAS_NOTE_DATA_DIR")) != ""
	status := StorageLocationStatus{
		DataRoot: dataRoot, BackupRoot: backupRoot,
		Source:                string(a.locationResolution.Source),
		EnvironmentOverride:   environmentOverride,
		SetupRequired:         a.startupPhase == StartupPhaseSetupRequired || a.locationResolution.SetupRequired,
		DataRootChangeAllowed: !environmentOverride,
		PendingDataRoot:       a.pendingDataRoot, PendingBackupRoot: a.pendingBackupRoot,
	}
	status.PendingRestart = status.PendingDataRoot != "" || status.PendingBackupRoot != ""
	if _, err := config.LoadPendingStorageLocationMigration(); err == nil {
		status.PendingRestart = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return status, err
	}
	return status, nil
}
