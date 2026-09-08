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
	DataRoot               string `json:"dataRoot,omitempty"`
	BackupRoot             string `json:"backupRoot,omitempty"`
	Source                 string `json:"source,omitempty"`
	EnvironmentOverride    bool   `json:"environmentOverride"`
	SetupRequired          bool   `json:"setupRequired"`
	RecoveryRequired       bool   `json:"recoveryRequired"`
	PendingRestart         bool   `json:"pendingRestart"`
	PendingDataRoot        string `json:"pendingDataRoot,omitempty"`
	PendingBackupRoot      string `json:"pendingBackupRoot,omitempty"`
	PendingMigration       bool   `json:"pendingMigration"`
	PendingMigrationAction string `json:"pendingMigrationAction,omitempty"`
	PendingSelection       bool   `json:"pendingSelection"`
	DataRootChangeAllowed  bool   `json:"dataRootChangeAllowed"`
}

type StorageLocationError struct {
	Code          string `json:"code"`
	Message       string `json:"message"`
	Reason        string `json:"reason,omitempty"`
	Stage         string `json:"stage,omitempty"`
	Role          string `json:"role,omitempty"`
	OSErrorNumber int    `json:"osErrorNumber,omitempty"`
	DiagnosticID  string `json:"diagnosticId,omitempty"`
	Action        string `json:"action,omitempty"`
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
	reason := "保存場所を利用できませんでした。"
	action := "保存場所を確認してから再試行してください。"
	result := &StorageLocationError{Code: code, Message: message, Reason: reason, Action: action}
	if rootCode := config.RootErrorCodeOf(err); rootCode != "" {
		code = "STORAGE_LOCATION_" + string(rootCode)
		switch rootCode {
		case config.RootErrorNotWritable:
			message = "選択したフォルダへ書き込めません。フォルダの権限や同期・セキュリティ設定を確認してください。"
			reason = "書き込み不可"
			action = "別の通常フォルダを選択するか、フォルダの権限を確認して再試行してください。"
		case config.RootErrorUnsafeLink:
			message = "symlinkまたはreparse pointを含むフォルダは保存場所にできません。通常のローカルフォルダを選択してください。"
			reason = "安全でないリンクまたはreparse point"
			action = "リンク先ではない通常のローカルフォルダを選択してください。"
		case config.RootErrorUnrelatedContent:
			message = "選択したフォルダにはAtlas Note以外のファイルがあります。空のフォルダを選択してください。"
			reason = "Atlas Note以外の内容"
			action = "空のフォルダ、またはAtlas Noteの既存保存場所を選択してください。"
		case config.RootErrorMissingData:
			message = "Atlas Noteのデータが揃っていない保存場所です。既存データか空のフォルダを選択してください。"
			reason = "Atlas Noteデータ不足"
			action = "既存データの保存場所か、空のフォルダを選択してください。"
		case config.RootErrorNotDirectory:
			message = "選択したパスはフォルダではありません。通常のフォルダを選択してください。"
			reason = "フォルダではない"
			action = "通常のフォルダを選択してください。"
		case config.RootErrorReadFailed:
			message = "保存場所を読み取れません。フォルダの権限や同期・セキュリティ設定を確認してください。"
			reason = "読み取り失敗"
			action = "フォルダが利用可能か、権限や同期状態を確認して再試行してください。"
		case config.RootErrorInvalidPath:
			message = "保存場所のパスが正しくありません。別のフォルダを選択してください。"
			reason = "パス不正"
			action = "別の通常フォルダを選択してください。"
		case config.RootErrorOverlappingRoots:
			code = storageLocationErrorValidation
			message = "保存領域とバックアップ保存領域に重複するフォルダは指定できません。"
			reason = "保存領域とバックアップ領域の重複"
			action = "互いに重複しないフォルダを選択してください。"
		case config.RootErrorInvalidConfig:
			code = storageLocationErrorValidation
			message = "保存場所の設定を検証できません。保存場所を選び直してください。"
			reason = "保存場所の設定不正"
			action = "保存領域とバックアップ保存領域を確認して再試行してください。"
		}
		result.Code = code
		result.Message = message
		result.Reason = reason
		result.Action = action
		var validationErr *config.RootValidationError
		if errors.As(err, &validationErr) {
			result.Stage = string(validationErr.Stage)
			result.Role = string(validationErr.Role)
		}
		result.OSErrorNumber = config.RootErrorOSNumberOf(err)
		if rootCode == config.RootErrorNotWritable || rootCode == config.RootErrorReadFailed {
			if reason, action := storageLocationOSGuidance(err); reason != "" {
				result.Reason, result.Action = reason, action
			}
		}
		return result
	}
	switch {
	case errors.Is(err, config.ErrRootInvalid), errors.Is(err, config.ErrLocationsInvalid):
		code = storageLocationErrorValidation
		message = "選択したフォルダを保存場所として利用できません。空のフォルダ、またはAtlas Noteの保存場所を選択してください。"
		reason = "保存場所の検証失敗"
		action = "空のフォルダ、またはAtlas Noteの既存保存場所を選択してください。"
	case errors.Is(err, os.ErrPermission):
		code = "STORAGE_LOCATION_UNWRITABLE"
		message = "保存場所へアクセスできません。フォルダの権限や同期・セキュリティ設定を確認してください。"
		reason = "アクセス拒否"
		action = "フォルダの権限や利用可能状態を確認して再試行してください。"
	case errors.Is(err, errStorageLocationEnvironment):
		code = storageLocationErrorEnvironment
		message = "ATLAS_NOTE_DATA_DIR が設定されているため、保存場所は変更できません。"
		reason = "環境設定による固定"
		action = "ATLAS_NOTE_DATA_DIR の設定を解除してから再試行してください。"
	case errors.Is(err, errStorageLocationMigration):
		code = storageLocationErrorMigration
		message = "保存場所の変更が次回起動を待っています。先にAtlas Noteを再起動してください。"
		reason = "保存場所の変更が保留中"
		action = "Atlas Noteを再起動して保留中の変更を適用してください。"
	case errors.Is(err, errStorageLocationRestorePending):
		code = storageLocationErrorRestore
		message = "復元待機中は保存場所を変更できません。復元を適用または取り消してから再試行してください。"
		reason = "復元が保留中"
		action = "復元を適用または取り消してから再試行してください。"
	}
	result.Code = code
	result.Message = message
	result.Reason = reason
	result.Action = action
	result.OSErrorNumber = config.RootErrorOSNumberOf(err)
	return result
}

var (
	errStorageLocationEnvironment    = errors.New("storage location is controlled by environment")
	errStorageLocationMigration      = errors.New("storage location migration is already pending")
	errStorageLocationUnavailable    = errors.New("storage location is unavailable")
	errStorageLocationRestorePending = errors.New("restore is pending")
)

func (a *App) GetStorageLocationStatus() StorageLocationStatusResult {
	status, err := a.storageLocationStatus()
	result := StorageLocationStatusResult{Status: &status}
	if err != nil {
		result.Error = a.storageLocationStatusFailure(err)
	}
	return result
}

func (a *App) SelectStorageLocation(kind string) StorageLocationSelectionResult {
	locationKind := StorageLocationKind(strings.TrimSpace(kind))
	if locationKind != StorageLocationDataRoot && locationKind != StorageLocationBackupRoot {
		return a.storageLocationSelectionFailure(kind, "", config.ErrRootInvalid)
	}
	if a.locationResolution.Environment || strings.TrimSpace(os.Getenv("ATLAS_NOTE_DATA_DIR")) != "" {
		return a.storageLocationSelectionFailure(string(locationKind), "", errStorageLocationEnvironment)
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
		return a.storageLocationSelectionFailure(string(locationKind), "", err)
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
		return a.storageLocationSelectionFailure(string(locationKind), path, err)
	}
	path = probe.Path
	candidateDataRoot, candidateBackupRoot := a.storageLocationCandidate(locationKind, path)
	if err := config.ValidateStorageLocationPaths(config.StorageLocations{Version: 1, DataRoot: candidateDataRoot, BackupRoot: candidateBackupRoot}); err != nil {
		return a.storageLocationSelectionFailure(string(locationKind), path, err)
	}
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
	a.pendingStorageSelection = true
	a.locationMu.Unlock()
	status, _ := a.storageLocationStatus()
	return StorageLocationSelectionResult{Kind: string(locationKind), Path: path, Probe: &probe, Status: &status}
}

func (a *App) storageLocationCandidate(kind StorageLocationKind, path string) (string, string) {
	a.locationMu.Lock()
	defer a.locationMu.Unlock()
	dataRoot := a.pendingDataRoot
	if dataRoot == "" {
		dataRoot = a.managementRoot
	}
	if dataRoot == "" {
		dataRoot = a.locationResolution.Locations.DataRoot
	}
	if dataRoot == "" {
		if defaultRoot, err := config.DefaultDataRoot(); err == nil {
			dataRoot = defaultRoot
		}
	}
	backupRoot := a.pendingBackupRoot
	if kind == StorageLocationDataRoot {
		dataRoot = path
		managementRoot := a.managementRoot
		if managementRoot == "" {
			managementRoot = a.locationResolution.Locations.DataRoot
		}
		archiveRoot := a.archiveRoot
		if archiveRoot == "" {
			archiveRoot = a.locationResolution.Locations.BackupRoot
		}
		if a.pendingBackupFollowsData || (backupRoot == "" && filepath.Clean(archiveRoot) == filepath.Clean(managementRoot)) {
			backupRoot = path
		}
	} else {
		backupRoot = path
	}
	if backupRoot == "" {
		if a.startupPhase == StartupPhaseSetupRequired || a.archiveRoot == "" || filepath.Clean(a.archiveRoot) == filepath.Clean(a.managementRoot) {
			backupRoot = dataRoot
		} else {
			backupRoot = a.archiveRoot
		}
	}
	if backupRoot == "" {
		backupRoot = dataRoot
	}
	return filepath.Clean(dataRoot), filepath.Clean(backupRoot)
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

func (a *App) clearPendingStorageSelection() {
	a.locationMu.Lock()
	a.pendingDataRoot = ""
	a.pendingBackupRoot = ""
	a.pendingBackupFollowsData = false
	a.pendingStorageSelection = false
	a.locationMu.Unlock()
}

func (a *App) ApplyStorageLocations() StorageLocationMutationResult {
	if a.startupPhase != StartupPhaseSetupRequired && a.startupPhase != StartupPhaseStorageRecovery && a.startupPhase != StartupPhaseReady {
		return a.storageLocationMutationFailure(errStorageLocationUnavailable, "storage-location.apply", string(config.RootValidationRoleUnknown))
	}
	if a.locationResolution.Environment || strings.TrimSpace(os.Getenv("ATLAS_NOTE_DATA_DIR")) != "" {
		return a.storageLocationMutationFailure(errStorageLocationEnvironment, "storage-location.apply", string(config.RootValidationRoleUnknown))
	}
	if a.startupPhase == StartupPhaseReady && a.backupService != nil {
		backupStatus, err := a.backupService.Status(a.operationContext())
		if err != nil {
			return a.storageLocationMutationFailure(err, "storage-location.apply", string(config.RootValidationRoleBackup))
		}
		if backupStatus.PendingRestore {
			return a.storageLocationMutationFailure(errStorageLocationRestorePending, "storage-location.apply", string(config.RootValidationRoleBackup))
		}
	}
	a.locationMu.Lock()
	dataRoot := a.pendingDataRoot
	backupRoot := a.pendingBackupRoot
	candidateSelected := a.pendingStorageSelection
	a.locationMu.Unlock()
	if a.startupPhase == StartupPhaseStorageRecovery && strings.TrimSpace(dataRoot) == "" {
		if candidateSelected {
			a.clearPendingStorageSelection()
		}
		return a.storageLocationMutationFailure(config.ErrRootInvalid, "storage-location.apply", string(config.RootValidationRoleData))
	}
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
		if candidateSelected {
			a.clearPendingStorageSelection()
		}
		return a.storageLocationMutationFailure(err, "storage-location.apply", string(config.RootValidationRoleUnknown))
	}
	currentDataRoot := filepath.Clean(a.managementRoot)
	currentBackupRoot := filepath.Clean(a.archiveRoot)
	if currentDataRoot == "." || currentDataRoot == "" {
		currentDataRoot = filepath.Clean(a.locationResolution.Locations.DataRoot)
	}
	if currentBackupRoot == "." || currentBackupRoot == "" {
		currentBackupRoot = filepath.Clean(a.locationResolution.Locations.BackupRoot)
	}
	if currentBackupRoot == "." || currentBackupRoot == "" {
		currentBackupRoot = currentDataRoot
	}

	if a.startupPhase == StartupPhaseStorageRecovery {
		migration := config.PendingStorageLocationMigration{
			Version: config.PendingStorageMigrationVersion, ID: strconv.FormatInt(time.Now().UnixNano(), 10),
			Action:         config.PendingStorageMigrationActionSwitch,
			SourceDataRoot: currentDataRoot, TargetDataRoot: dataRoot,
			SourceBackupRoot: currentBackupRoot, TargetBackupRoot: backupRoot,
		}
		if err := config.SavePendingStorageLocationMigration(migration); err != nil {
			return a.storageLocationMutationFailure(err, "storage-location.apply", string(config.RootValidationRoleUnknown))
		}
		a.locationMu.Lock()
		a.pendingDataRoot, a.pendingBackupRoot = dataRoot, backupRoot
		a.pendingBackupFollowsData = filepath.Clean(dataRoot) == filepath.Clean(backupRoot)
		a.pendingStorageSelection = false
		a.locationMu.Unlock()
		status, _ := a.storageLocationStatus()
		return StorageLocationMutationResult{Status: &status, RestartRequired: true}
	}

	if a.startupPhase == StartupPhaseSetupRequired {
		if err := config.SaveStorageLocations(config.StorageLocations{Version: 1, DataRoot: dataRoot, BackupRoot: backupRoot}); err != nil {
			return a.storageLocationMutationFailure(err, "storage-location.apply", string(config.RootValidationRoleUnknown))
		}
		a.locationResolution = config.LocationResolution{
			Locations: config.StorageLocations{Version: 1, DataRoot: dataRoot, BackupRoot: backupRoot},
			Source:    config.LocationSourceSaved,
		}
		a.managementRoot, a.archiveRoot = dataRoot, backupRoot
		a.locationMu.Lock()
		a.pendingDataRoot, a.pendingBackupRoot = "", ""
		a.pendingBackupFollowsData = false
		a.pendingStorageSelection = false
		a.locationMu.Unlock()
		status, _ := a.storageLocationStatus()
		return StorageLocationMutationResult{Status: &status, RestartRequired: true}
	}

	if _, err := config.LoadPendingStorageLocationMigration(); err == nil {
		return a.storageLocationMutationFailure(errStorageLocationMigration, "storage-location.apply", string(config.RootValidationRoleUnknown))
	} else if !errors.Is(err, os.ErrNotExist) {
		return a.storageLocationMutationFailure(err, "storage-location.apply", string(config.RootValidationRoleUnknown))
	}

	if dataRoot == currentDataRoot && backupRoot == currentBackupRoot {
		a.locationMu.Lock()
		a.pendingDataRoot, a.pendingBackupRoot = "", ""
		a.pendingBackupFollowsData = false
		a.pendingStorageSelection = false
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
		return a.storageLocationMutationFailure(err, "storage-location.apply", string(config.RootValidationRoleUnknown))
	}
	a.locationMu.Lock()
	a.pendingDataRoot, a.pendingBackupRoot = dataRoot, backupRoot
	a.pendingBackupFollowsData = false
	a.pendingStorageSelection = false
	a.locationMu.Unlock()
	status, _ := a.storageLocationStatus()
	return StorageLocationMutationResult{Status: &status, RestartRequired: true}
}

func (a *App) CancelPendingStorageLocationMigration() StorageLocationMutationResult {
	if a.startupPhase != StartupPhaseStorageRecovery {
		return a.storageLocationMutationFailure(errStorageLocationUnavailable, "storage-location.cancel", string(config.RootValidationRoleUnknown))
	}
	if a.locationResolution.Environment || strings.TrimSpace(os.Getenv("ATLAS_NOTE_DATA_DIR")) != "" {
		return a.storageLocationMutationFailure(errStorageLocationEnvironment, "storage-location.cancel", string(config.RootValidationRoleUnknown))
	}
	migration, err := config.LoadPendingStorageLocationMigrationForRecovery()
	if err != nil {
		return a.storageLocationMutationFailure(err, "storage-location.cancel", string(config.RootValidationRoleUnknown))
	}
	migration.Action = config.PendingStorageMigrationActionCancel
	if err := config.ValidatePendingStorageLocationMigration(migration); err != nil {
		return a.storageLocationMutationFailure(err, "storage-location.cancel", string(config.RootValidationRoleUnknown))
	}
	if err := config.SavePendingStorageLocationMigration(migration); err != nil {
		return a.storageLocationMutationFailure(err, "storage-location.cancel", string(config.RootValidationRoleUnknown))
	}
	a.locationMu.Lock()
	a.pendingDataRoot, a.pendingBackupRoot = "", ""
	a.pendingBackupFollowsData = false
	a.pendingStorageSelection = false
	a.locationMu.Unlock()
	status, statusErr := a.storageLocationStatus()
	if statusErr != nil {
		return StorageLocationMutationResult{Status: &status, Error: a.storageLocationFailure(statusErr, "storage-location.cancel", string(config.RootValidationRoleUnknown))}
	}
	return StorageLocationMutationResult{Status: &status, RestartRequired: true}
}

func (a *App) RetryPendingStorageLocationMigration() StorageLocationMutationResult {
	if a.startupPhase != StartupPhaseStorageRecovery {
		return a.storageLocationMutationFailure(errStorageLocationUnavailable, "storage-location.retry", string(config.RootValidationRoleUnknown))
	}
	if a.locationResolution.Environment || strings.TrimSpace(os.Getenv("ATLAS_NOTE_DATA_DIR")) != "" {
		return a.storageLocationMutationFailure(errStorageLocationEnvironment, "storage-location.retry", string(config.RootValidationRoleUnknown))
	}
	migration, err := config.LoadPendingStorageLocationMigration()
	if err != nil {
		return a.storageLocationMutationFailure(err, "storage-location.retry", string(config.RootValidationRoleUnknown))
	}
	if migration.Action == config.PendingStorageMigrationActionCancel {
		migration.Action = config.PendingStorageMigrationActionMigrate
	}
	if migration.Version != config.PendingStorageMigrationVersion ||
		(migration.Action == config.PendingStorageMigrationActionMigrate &&
			(migration.DataPlan == "" || migration.BackupPlan == "" || migration.Phase == "")) {
		prepared, prepareErr := config.PreparePendingStorageLocationMigrationForRetry(migration)
		if prepareErr != nil {
			return a.storageLocationMutationFailure(prepareErr, "storage-location.retry", string(config.RootValidationRoleUnknown))
		}
		migration = prepared
	}
	if migration.Action == config.PendingStorageMigrationActionMigrate {
		if err := config.ValidatePendingStorageLocationMigrationForRetry(migration); err != nil {
			return a.storageLocationMutationFailure(err, "storage-location.retry", string(config.RootValidationRoleUnknown))
		}
	} else if err := config.ValidateStorageLocations(config.StorageLocations{
		Version: 1, DataRoot: migration.TargetDataRoot, BackupRoot: migration.TargetBackupRoot,
	}); err != nil {
		return a.storageLocationMutationFailure(err, "storage-location.retry", string(config.RootValidationRoleUnknown))
	}
	if err := config.SavePendingStorageLocationMigration(migration); err != nil {
		return a.storageLocationMutationFailure(err, "storage-location.retry", string(config.RootValidationRoleUnknown))
	}
	status, statusErr := a.storageLocationStatus()
	if statusErr != nil {
		return StorageLocationMutationResult{Status: &status, Error: a.storageLocationFailure(statusErr, "storage-location.retry", string(config.RootValidationRoleUnknown))}
	}
	return StorageLocationMutationResult{Status: &status, RestartRequired: true}
}

func (a *App) CancelStorageLocationSelection() StorageLocationStatusResult {
	a.locationMu.Lock()
	a.pendingDataRoot, a.pendingBackupRoot = "", ""
	a.pendingBackupFollowsData = false
	a.pendingStorageSelection = false
	a.locationMu.Unlock()
	status, err := a.storageLocationStatus()
	if err != nil {
		return StorageLocationStatusResult{Status: &status, Error: a.storageLocationFailure(err, "storage-location.cancel-selection", string(config.RootValidationRoleUnknown))}
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
		RecoveryRequired:      a.startupPhase == StartupPhaseStorageRecovery,
		DataRootChangeAllowed: !environmentOverride,
		PendingDataRoot:       a.pendingDataRoot, PendingBackupRoot: a.pendingBackupRoot,
		PendingSelection: a.pendingStorageSelection,
	}
	status.PendingRestart = status.PendingDataRoot != "" || status.PendingBackupRoot != ""
	if pending, err := config.LoadPendingStorageLocationMigrationForRecovery(); err == nil {
		status.PendingRestart = true
		status.PendingMigration = true
		status.PendingMigrationAction = pending.Action
		if validationErr := config.ValidatePendingStorageLocationMigration(pending); validationErr != nil {
			return status, validationErr
		}
		if pending.Action != config.PendingStorageMigrationActionCancel {
			if status.PendingDataRoot == "" {
				status.PendingDataRoot = pending.TargetDataRoot
			}
			if status.PendingBackupRoot == "" {
				status.PendingBackupRoot = pending.TargetBackupRoot
			}
		} else {
			// A cancel intent executes only against the source roots. Do not
			// expose stale or untrusted target fields from the old marker.
			status.PendingDataRoot = ""
			status.PendingBackupRoot = ""
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		status.PendingRestart = true
		status.PendingMigration = true
		return status, err
	}
	return status, nil
}
