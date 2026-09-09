package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	storageLocationsVersion = 1
	storageLocationsFile    = "storage-locations.json"
	storageLocationsPathEnv = "ATLAS_NOTE_STORAGE_LOCATIONS_FILE"
	defaultDataRootEnv      = "ATLAS_NOTE_DEFAULT_DATA_ROOT"
	storageLocationsTempDir = ".atlasnote-location-tmp"
)

// LocationSource describes where the active storage roots came from. An
// environment override is intentionally kept separate from the persisted
// configuration so deployments can pin the data root without changing it from
// the UI.
type LocationSource string

const (
	LocationSourceEnvironment LocationSource = "environment"
	LocationSourceSaved       LocationSource = "saved"
	LocationSourceDefault     LocationSource = "default"
	LocationSourceLegacy      LocationSource = "legacy"
)

// StorageLocations is the small, versioned bootstrap file used before the
// application opens a database. BackupRoot is an archive root, not a logical
// storage-space directory. An empty BackupRoot means the data root's internal
// backup area for compatibility with existing installations.
type StorageLocations struct {
	Version    int    `json:"version"`
	DataRoot   string `json:"dataRoot"`
	BackupRoot string `json:"backupRoot,omitempty"`
}

type LocationResolution struct {
	Locations     StorageLocations
	Source        LocationSource
	SetupRequired bool
	Environment   bool
}

type RootKind string

const (
	RootEmpty    RootKind = "empty"
	RootExisting RootKind = "existing"
)

type RootProbe struct {
	Path         string   `json:"path"`
	Kind         RootKind `json:"kind"`
	Exists       bool     `json:"exists"`
	HasAtlasData bool     `json:"hasAtlasData"`
	HasBackups   bool     `json:"hasBackups"`
	Writable     bool     `json:"writable"`
}

// RootErrorCode identifies the safe, user-actionable reason why a folder was
// rejected. The underlying error is still ErrRootInvalid so existing callers
// can keep using errors.Is.
type RootErrorCode string

const (
	RootErrorInvalidPath      RootErrorCode = "INVALID_PATH"
	RootErrorNotDirectory     RootErrorCode = "NOT_DIRECTORY"
	RootErrorNotWritable      RootErrorCode = "UNWRITABLE"
	RootErrorUnsafeLink       RootErrorCode = "UNSAFE_LINK"
	RootErrorReadFailed       RootErrorCode = "READ_FAILED"
	RootErrorUnrelatedContent RootErrorCode = "UNRELATED_CONTENT"
	RootErrorMissingData      RootErrorCode = "MISSING_ATLAS_DATA"
	RootErrorOverlappingRoots RootErrorCode = "OVERLAPPING_ROOTS"
	RootErrorInvalidConfig    RootErrorCode = "INVALID_CONFIGURATION"
)

// RootValidationStage identifies the operation which rejected a root. It is
// intentionally a small, stable allowlist so callers can explain an OS error
// without exposing its path or raw message.
type RootValidationStage string

const (
	RootValidationStagePath            RootValidationStage = "path"
	RootValidationStageConfig          RootValidationStage = "configuration"
	RootValidationStageRootLookup      RootValidationStage = "root-lookup"
	RootValidationStageParentLookup    RootValidationStage = "parent-lookup"
	RootValidationStageReadDirectory   RootValidationStage = "read-directory"
	RootValidationStageReadEntry       RootValidationStage = "read-entry"
	RootValidationStageCreateFile      RootValidationStage = "create-file"
	RootValidationStageWriteFile       RootValidationStage = "write-file"
	RootValidationStageCloseFile       RootValidationStage = "close-file"
	RootValidationStageRenameFile      RootValidationStage = "rename-file"
	RootValidationStageRemoveFile      RootValidationStage = "remove-file"
	RootValidationStageCreateDirectory RootValidationStage = "create-directory"
	RootValidationStageRenameDirectory RootValidationStage = "rename-directory"
	RootValidationStageRemoveDirectory RootValidationStage = "remove-directory"
	RootValidationStageContent         RootValidationStage = "content"
	RootValidationStageValidation      RootValidationStage = "validation"
)

// RootValidationRole identifies which selected root failed validation.
type RootValidationRole string

const (
	RootValidationRoleUnknown RootValidationRole = "unknown"
	RootValidationRoleData    RootValidationRole = "data"
	RootValidationRoleBackup  RootValidationRole = "backup"
)

// RootValidationError reports a classified root validation failure while
// preserving ErrRootInvalid for callers that only need the legacy contract.
type RootValidationError struct {
	Code  RootErrorCode
	Stage RootValidationStage
	Role  RootValidationRole
	Cause error
}

func (e *RootValidationError) Error() string {
	if e.Cause == nil {
		return string(e.Code)
	}
	return fmt.Sprintf("%s: %v", e.Code, e.Cause)
}

func (e *RootValidationError) Unwrap() error {
	return e.Cause
}

// Is preserves both legacy sentinels even when Cause is the concrete OS error
// that caused a failed read/create/write/rename/remove operation.
func (e *RootValidationError) Is(target error) bool {
	return target == ErrRootInvalid || errors.Is(e.Cause, target)
}

// RootErrorCodeOf returns a stable reason code when a root validation error
// was classified, or an empty value for an unrelated error.
func RootErrorCodeOf(err error) RootErrorCode {
	var validationErr *RootValidationError
	if errors.As(err, &validationErr) {
		return validationErr.Code
	}
	return ""
}

// RootErrorOSNumberOf returns the platform error number when the validation
// retained one, or zero when the failure did not originate from an OS errno.
func RootErrorOSNumberOf(err error) int {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return int(errno)
	}
	return 0
}

var (
	ErrLocationsInvalid = errors.New("storage locations are invalid")
	ErrRootInvalid      = errors.New("storage root is invalid")
)

func DefaultDataRoot() (string, error) {
	if override := strings.TrimSpace(os.Getenv(defaultDataRootEnv)); override != "" {
		return normalizeAbsolutePath(override)
	}
	return defaultDataRootFromPlatform()
}

func legacyDataRoot() (string, error) {
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Clean(filepath.Join(userConfigDir, "AtlasNote")), nil
}

func StorageLocationsPath() (string, error) {
	if override := strings.TrimSpace(os.Getenv(storageLocationsPathEnv)); override != "" {
		return normalizeAbsolutePath(override)
	}
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(userConfigDir, "AtlasNote", storageLocationsFile), nil
}

func LoadStorageLocations() (StorageLocations, error) {
	path, err := StorageLocationsPath()
	if err != nil {
		return StorageLocations{}, err
	}
	return LoadStorageLocationsFrom(path)
}

func LoadStorageLocationsFrom(filePath string) (StorageLocations, error) {
	return loadStorageLocationsFrom(filePath, true)
}

// LoadStorageLocationsForRecovery reads a valid bootstrap file without
// probing its roots. It is used only to show a recovery screen when the
// previously selected root is no longer available.
func LoadStorageLocationsForRecovery() (StorageLocations, error) {
	path, err := StorageLocationsPath()
	if err != nil {
		return StorageLocations{}, err
	}
	return loadStorageLocationsFrom(path, false)
}

// LoadStorageLocationsForRecoveryFrom reads a bootstrap file without probing
// or modifying either storage root. Standalone maintenance commands use it
// so an uninstall helper can inspect the saved locations without starting the
// normal migration/recovery path.
func LoadStorageLocationsForRecoveryFrom(filePath string) (StorageLocations, error) {
	return loadStorageLocationsFrom(filePath, false)
}

func loadStorageLocationsFrom(filePath string, probeRoots bool) (StorageLocations, error) {
	filePath, err := normalizeAbsolutePath(filePath)
	if err != nil {
		return StorageLocations{}, err
	}
	info, err := os.Lstat(filePath)
	if err != nil {
		return StorageLocations{}, err
	}
	if isUnsafeStoragePath(filePath, info) || !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return StorageLocations{}, ErrLocationsInvalid
	}
	encoded, err := os.ReadFile(filePath)
	if err != nil {
		return StorageLocations{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var locations StorageLocations
	if err := decoder.Decode(&locations); err != nil {
		return StorageLocations{}, fmt.Errorf("%w: %v", ErrLocationsInvalid, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return StorageLocations{}, ErrLocationsInvalid
	}
	var validationErr error
	if probeRoots {
		validationErr = validateStorageLocations(locations)
	} else {
		validationErr = validateStorageLocationPaths(locations)
	}
	if err := validationErr; err != nil {
		return StorageLocations{}, err
	}
	return locations, nil
}

func SaveStorageLocations(locations StorageLocations) error {
	path, err := StorageLocationsPath()
	if err != nil {
		return err
	}
	return SaveStorageLocationsTo(path, locations)
}

func SaveStorageLocationsTo(filePath string, locations StorageLocations) error {
	filePath, err := normalizeAbsolutePath(filePath)
	if err != nil {
		return err
	}
	if locations.Version == 0 {
		locations.Version = storageLocationsVersion
	}
	if locations.BackupRoot == "" {
		locations.BackupRoot = locations.DataRoot
	}
	if err := validateStorageLocations(locations); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(locations, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if err := os.MkdirAll(filepath.Dir(filePath), 0o700); err != nil {
		return fmt.Errorf("create storage locations directory: %w", err)
	}
	return writeAtomic(filePath, encoded)
}

// ValidateStorageLocations checks the bootstrap roots without writing the
// configuration file. Callers use it before recording a restart-time change.
func ValidateStorageLocations(locations StorageLocations) error {
	if locations.Version == 0 {
		locations.Version = storageLocationsVersion
	}
	if locations.BackupRoot == "" {
		locations.BackupRoot = locations.DataRoot
	}
	return validateStorageLocations(locations)
}

func ResolveStorageLocations() (LocationResolution, error) {
	if raw := strings.TrimSpace(os.Getenv(dataDirEnv)); raw != "" {
		dataRoot, err := normalizeAbsolutePath(raw)
		if err != nil {
			return LocationResolution{}, err
		}
		return LocationResolution{
			Locations: StorageLocations{Version: storageLocationsVersion, DataRoot: dataRoot, BackupRoot: dataRoot},
			Source:    LocationSourceEnvironment, Environment: true,
		}, nil
	}

	locations, err := LoadStorageLocations()
	if err == nil {
		return LocationResolution{Locations: locations, Source: LocationSourceSaved}, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return LocationResolution{}, err
	}

	defaultRoot, err := DefaultDataRoot()
	if err != nil {
		return LocationResolution{}, err
	}

	// Before the Documents default was introduced, Atlas Note stored data in
	// the user config directory. Preserve an existing Atlas root there without
	// moving or deleting it.
	legacyRoot, legacyErr := legacyDataRoot()
	if legacyErr == nil && filepath.Clean(legacyRoot) != filepath.Clean(defaultRoot) {
		legacyProbe, probeErr := ProbeDataRoot(legacyRoot)
		if probeErr == nil && legacyProbe.Kind == RootExisting && legacyProbe.HasAtlasData {
			return LocationResolution{
				Locations: StorageLocations{Version: storageLocationsVersion, DataRoot: legacyRoot, BackupRoot: legacyRoot},
				Source:    LocationSourceLegacy,
			}, nil
		}
	}

	probe, probeErr := ProbeDataRoot(defaultRoot)
	if probeErr != nil {
		return LocationResolution{}, probeErr
	}
	return LocationResolution{
		Locations:     StorageLocations{Version: storageLocationsVersion, DataRoot: defaultRoot, BackupRoot: defaultRoot},
		Source:        LocationSourceDefault,
		SetupRequired: probe.Kind == RootEmpty,
	}, nil
}

func ProbeDataRoot(root string) (RootProbe, error) {
	return probeRoot(root, false, RootValidationRoleData)
}

func ProbeBackupRoot(root string) (RootProbe, error) {
	return probeRoot(root, true, RootValidationRoleBackup)
}

func probeRoot(root string, backup bool, role RootValidationRole) (RootProbe, error) {
	absolute, err := normalizeAbsolutePath(root)
	if err != nil {
		return RootProbe{}, rootValidationErrorAt(RootErrorInvalidPath, RootValidationStagePath, role, err)
	}
	probe := RootProbe{Path: absolute}
	info, err := os.Lstat(absolute)
	if errors.Is(err, os.ErrNotExist) {
		parent, parentErr := writableExistingParent(absolute, role)
		if parentErr != nil {
			return RootProbe{}, parentErr
		}
		probe.Writable = parent != ""
		probe.Kind = RootEmpty
		return probe, nil
	}
	if err != nil {
		return RootProbe{}, rootValidationErrorAt(RootErrorReadFailed, RootValidationStageRootLookup, role, err)
	}
	if isUnsafeStoragePath(absolute, info) || !info.IsDir() {
		if isUnsafeStoragePath(absolute, info) {
			return RootProbe{}, rootValidationErrorAt(RootErrorUnsafeLink, RootValidationStageRootLookup, role, nil)
		}
		return RootProbe{}, rootValidationErrorAt(RootErrorNotDirectory, RootValidationStageRootLookup, role, nil)
	}
	probe.Exists = true
	if writableErr := writableDirectoryError(absolute, role); writableErr != nil {
		return RootProbe{}, writableErr
	}
	probe.Writable = true
	entries, err := os.ReadDir(absolute)
	if err != nil {
		return RootProbe{}, rootValidationErrorAt(RootErrorReadFailed, RootValidationStageReadDirectory, role, err)
	}
	hasRecognized := false
	effectiveEntries := 0
	for _, entry := range entries {
		entryPath := filepath.Join(absolute, entry.Name())
		entryInfo, entryErr := entry.Info()
		if entryErr != nil {
			return RootProbe{}, rootValidationErrorAt(RootErrorReadFailed, RootValidationStageReadEntry, role, entryErr)
		}
		if isUnsafeStoragePath(entryPath, entryInfo) {
			return RootProbe{}, rootValidationErrorAt(RootErrorUnsafeLink, RootValidationStageReadEntry, role, nil)
		}
		name := entry.Name()
		if !backup && name == storageLocationsFile {
			// The bootstrap file lives beside the default data root and is not
			// itself evidence that a database has been initialized.
			continue
		}
		effectiveEntries++
		if backup && name == ".atlasnote-backups" {
			if !entryInfo.IsDir() {
				return RootProbe{}, rootValidationErrorAt(RootErrorNotDirectory, RootValidationStageReadEntry, role, nil)
			}
			probe.HasBackups = true
			hasRecognized = true
			continue
		}
		if !backup && (name == "atlasnote.db" || name == "storage-spaces.json" || name == "notes" || name == "spaces" || name == ".sync-recovery") {
			hasRecognized = true
			if name == "atlasnote.db" || name == "storage-spaces.json" {
				probe.HasAtlasData = true
			}
		}
	}
	if effectiveEntries == 0 {
		probe.Kind = RootEmpty
		return probe, nil
	}
	if !hasRecognized {
		return RootProbe{}, rootValidationErrorAt(RootErrorUnrelatedContent, RootValidationStageContent, role, nil)
	}
	if !backup && !probe.HasAtlasData {
		return RootProbe{}, rootValidationErrorAt(RootErrorMissingData, RootValidationStageContent, role, nil)
	}
	probe.Kind = RootExisting
	return probe, nil
}

func validateStorageLocations(locations StorageLocations) error {
	if err := validateStorageLocationPaths(locations); err != nil {
		return err
	}
	dataRoot, _ := normalizeAbsolutePath(locations.DataRoot)
	backupRoot := dataRoot
	if strings.TrimSpace(locations.BackupRoot) != "" {
		backupRoot, _ = normalizeAbsolutePath(locations.BackupRoot)
	}
	if _, err := ProbeDataRoot(dataRoot); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if backupRoot != dataRoot {
		if _, err := ProbeBackupRoot(backupRoot); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// ValidateStorageLocationPaths checks only the pair's structural constraints.
// Candidate selection must not require the unchanged root to be accessible.
func ValidateStorageLocationPaths(locations StorageLocations) error {
	return validateStorageLocationPaths(locations)
}

func validateStorageLocationPaths(locations StorageLocations) error {
	if locations.Version != storageLocationsVersion || strings.TrimSpace(locations.DataRoot) == "" {
		return rootValidationErrorAt(RootErrorInvalidConfig, RootValidationStageConfig, RootValidationRoleUnknown, ErrLocationsInvalid)
	}
	dataRoot, err := normalizeAbsolutePath(locations.DataRoot)
	if err != nil || dataRoot != filepath.Clean(locations.DataRoot) {
		return rootValidationErrorAt(RootErrorInvalidPath, RootValidationStagePath, RootValidationRoleData, err)
	}
	backupValue := locations.BackupRoot
	backupRoot := backupValue
	if backupValue == "" {
		backupRoot = dataRoot
	}
	backupRoot, err = normalizeAbsolutePath(backupRoot)
	if err != nil || (backupValue != "" && backupRoot != filepath.Clean(backupValue)) {
		return rootValidationErrorAt(RootErrorInvalidPath, RootValidationStagePath, RootValidationRoleBackup, err)
	}
	if backupRoot != dataRoot && (isWithinOrEqual(dataRoot, backupRoot) || isWithinOrEqual(backupRoot, dataRoot)) {
		return rootValidationErrorAt(RootErrorOverlappingRoots, RootValidationStageConfig, RootValidationRoleUnknown, ErrLocationsInvalid)
	}
	return nil
}

func rootValidationError(code RootErrorCode) error {
	return rootValidationErrorAt(code, RootValidationStageValidation, RootValidationRoleUnknown, nil)
}

func rootValidationErrorAt(code RootErrorCode, stage RootValidationStage, role RootValidationRole, cause error) error {
	if cause == nil {
		cause = ErrRootInvalid
	}
	return &RootValidationError{Code: code, Stage: stage, Role: role, Cause: cause}
}

func normalizeAbsolutePath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", ErrRootInvalid
	}
	absolute, err := filepath.Abs(filepath.Clean(value))
	if err != nil || !filepath.IsAbs(absolute) {
		return "", ErrRootInvalid
	}
	return filepath.Clean(absolute), nil
}

// writableDirectory is kept as a small compatibility helper for package-local
// callers. Root probing uses writableDirectoryError so it can retain the
// exact failed operation and underlying OS error.
func writableDirectory(path string) bool {
	return writableDirectoryError(path, RootValidationRoleUnknown) == nil
}

func writableDirectoryError(path string, role RootValidationRole) error {
	temporary, err := os.CreateTemp(path, ".atlasnote-write-test-"+time.Now().UTC().Format("20060102150405.000000000"))
	if err != nil {
		return rootValidationErrorAt(RootErrorNotWritable, RootValidationStageCreateFile, role, err)
	}
	temporaryPath := temporary.Name()
	if _, err := temporary.Write([]byte("atlasnote-write-test")); err != nil {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
		return rootValidationErrorAt(RootErrorNotWritable, RootValidationStageWriteFile, role, err)
	}
	if err := temporary.Close(); err != nil {
		_ = os.Remove(temporaryPath)
		return rootValidationErrorAt(RootErrorNotWritable, RootValidationStageCloseFile, role, err)
	}
	renamedFilePath := temporaryPath + ".renamed"
	if err := os.Rename(temporaryPath, renamedFilePath); err != nil {
		_ = os.Remove(temporaryPath)
		return rootValidationErrorAt(RootErrorNotWritable, RootValidationStageRenameFile, role, err)
	}
	if err := os.Remove(renamedFilePath); err != nil {
		return rootValidationErrorAt(RootErrorNotWritable, RootValidationStageRemoveFile, role, err)
	}

	temporaryDirectory, err := os.MkdirTemp(path, ".atlasnote-directory-test-"+time.Now().UTC().Format("20060102150405.000000000"))
	if err != nil {
		return rootValidationErrorAt(RootErrorNotWritable, RootValidationStageCreateDirectory, role, err)
	}
	renamedDirectoryPath := temporaryDirectory + ".renamed"
	if err := os.Rename(temporaryDirectory, renamedDirectoryPath); err != nil {
		_ = os.Remove(temporaryDirectory)
		return rootValidationErrorAt(RootErrorNotWritable, RootValidationStageRenameDirectory, role, err)
	}
	if err := os.Remove(renamedDirectoryPath); err != nil {
		return rootValidationErrorAt(RootErrorNotWritable, RootValidationStageRemoveDirectory, role, err)
	}
	return nil
}

func writableExistingParent(path string, role RootValidationRole) (string, error) {
	current := filepath.Clean(filepath.Dir(path))
	for {
		info, err := os.Lstat(current)
		if err == nil {
			if isUnsafeStoragePath(current, info) {
				return "", rootValidationErrorAt(RootErrorUnsafeLink, RootValidationStageParentLookup, role, nil)
			}
			if !info.IsDir() {
				return "", rootValidationErrorAt(RootErrorNotDirectory, RootValidationStageParentLookup, role, nil)
			}
			if writableErr := writableDirectoryError(current, role); writableErr != nil {
				return "", writableErr
			}
			return current, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", rootValidationErrorAt(RootErrorReadFailed, RootValidationStageParentLookup, role, err)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", rootValidationErrorAt(RootErrorInvalidPath, RootValidationStageParentLookup, role, ErrRootInvalid)
		}
		current = parent
	}
}

func writeAtomic(filePath string, contents []byte) error {
	temporaryDir := filepath.Dir(filePath)
	if err := validateAtomicDirectoryPath(temporaryDir); err != nil {
		return err
	}
	if err := os.MkdirAll(temporaryDir, 0o700); err != nil {
		return err
	}
	if err := validateAtomicDirectoryPath(temporaryDir); err != nil {
		return err
	}
	if info, err := os.Lstat(filePath); err == nil {
		if isUnsafeStoragePath(filePath, info) || !info.Mode().IsRegular() {
			return ErrLocationsInvalid
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	temporaryPath := filepath.Join(temporaryDir, "."+filepath.Base(filePath)+"."+storageLocationsTempDir+"-"+fmt.Sprint(time.Now().UnixNano())+".tmp")
	file, err := os.OpenFile(temporaryPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = file.Close()
			_ = os.Remove(temporaryPath)
		}
	}()
	if _, err := file.Write(contents); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, filePath); err != nil {
		return err
	}
	committed = true
	return nil
}

func validateAtomicDirectoryPath(path string) error {
	current := filepath.Clean(path)
	for {
		info, err := os.Lstat(current)
		if err == nil {
			if isUnsafeStoragePath(current, info) || !info.IsDir() {
				return ErrLocationsInvalid
			}
			return nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ErrLocationsInvalid
		}
		current = parent
	}
}
