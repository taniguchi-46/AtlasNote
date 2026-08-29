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

var (
	ErrLocationsInvalid = errors.New("storage locations are invalid")
	ErrRootInvalid      = errors.New("storage root is invalid")
)

func DefaultDataRoot() (string, error) {
	if override := strings.TrimSpace(os.Getenv(defaultDataRootEnv)); override != "" {
		return normalizeAbsolutePath(override)
	}
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
	root, err := DefaultDataRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, storageLocationsFile), nil
}

func LoadStorageLocations() (StorageLocations, error) {
	path, err := StorageLocationsPath()
	if err != nil {
		return StorageLocations{}, err
	}
	return LoadStorageLocationsFrom(path)
}

func LoadStorageLocationsFrom(filePath string) (StorageLocations, error) {
	filePath, err := normalizeAbsolutePath(filePath)
	if err != nil {
		return StorageLocations{}, err
	}
	info, err := os.Lstat(filePath)
	if err != nil {
		return StorageLocations{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() > 1<<20 {
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
	if err := validateStorageLocations(locations); err != nil {
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
	return probeRoot(root, false)
}

func ProbeBackupRoot(root string) (RootProbe, error) {
	return probeRoot(root, true)
}

func probeRoot(root string, backup bool) (RootProbe, error) {
	absolute, err := normalizeAbsolutePath(root)
	if err != nil {
		return RootProbe{}, err
	}
	probe := RootProbe{Path: absolute}
	info, err := os.Lstat(absolute)
	if errors.Is(err, os.ErrNotExist) {
		parent, parentErr := writableExistingParent(absolute)
		if parentErr != nil {
			return RootProbe{}, parentErr
		}
		probe.Writable = parent != ""
		probe.Kind = RootEmpty
		return probe, nil
	}
	if err != nil {
		return RootProbe{}, fmt.Errorf("inspect storage root: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return RootProbe{}, ErrRootInvalid
	}
	probe.Exists = true
	probe.Writable = writableDirectory(absolute)
	if !probe.Writable {
		return RootProbe{}, ErrRootInvalid
	}
	entries, err := os.ReadDir(absolute)
	if err != nil {
		return RootProbe{}, fmt.Errorf("read storage root: %w", err)
	}
	hasRecognized := false
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			return RootProbe{}, ErrRootInvalid
		}
		name := entry.Name()
		if !backup && name == storageLocationsFile {
			// The bootstrap file lives beside the default data root and is not
			// itself evidence that a database has been initialized.
			continue
		}
		if backup && name == ".atlasnote-backups" {
			if !entry.IsDir() {
				return RootProbe{}, ErrRootInvalid
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
	if len(entries) == 0 {
		probe.Kind = RootEmpty
		return probe, nil
	}
	if !hasRecognized {
		return RootProbe{}, ErrRootInvalid
	}
	if !backup && !probe.HasAtlasData {
		return RootProbe{}, ErrRootInvalid
	}
	probe.Kind = RootExisting
	return probe, nil
}

func validateStorageLocations(locations StorageLocations) error {
	if locations.Version != storageLocationsVersion || strings.TrimSpace(locations.DataRoot) == "" {
		return ErrLocationsInvalid
	}
	dataRoot, err := normalizeAbsolutePath(locations.DataRoot)
	if err != nil || dataRoot != filepath.Clean(locations.DataRoot) {
		return ErrLocationsInvalid
	}
	backupValue := locations.BackupRoot
	backupRoot := backupValue
	if backupValue == "" {
		backupRoot = dataRoot
	}
	backupRoot, err = normalizeAbsolutePath(backupRoot)
	if err != nil || (backupValue != "" && backupRoot != filepath.Clean(backupValue)) {
		return ErrLocationsInvalid
	}
	if backupRoot != dataRoot && (isWithinOrEqual(dataRoot, backupRoot) || isWithinOrEqual(backupRoot, dataRoot)) {
		return ErrLocationsInvalid
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

func writableDirectory(path string) bool {
	temporary, err := os.CreateTemp(path, ".atlasnote-write-test-"+time.Now().UTC().Format("20060102150405.000000000"))
	if err != nil {
		return false
	}
	temporaryPath := temporary.Name()
	_ = temporary.Close()
	_ = os.Remove(temporaryPath)
	return true
}

func writableExistingParent(path string) (string, error) {
	current := filepath.Clean(filepath.Dir(path))
	for {
		info, err := os.Lstat(current)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || !writableDirectory(current) {
				return "", ErrRootInvalid
			}
			return current, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", ErrRootInvalid
		}
		current = parent
	}
}

func writeAtomic(filePath string, contents []byte) error {
	temporaryDir := filepath.Dir(filePath)
	if err := os.MkdirAll(temporaryDir, 0o700); err != nil {
		return err
	}
	if info, err := os.Lstat(filePath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
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
