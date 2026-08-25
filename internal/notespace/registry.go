package notespace

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"atlasnote/internal/datalock"
	"golang.org/x/text/unicode/norm"
)

const (
	catalogVersion     = 1
	catalogFileName    = "storage-spaces.json"
	registryLockName   = "storage-spaces.lock"
	spacesDirectory    = "spaces"
	legacySpaceName    = "メイン"
	maximumCatalogSize = 1 << 20
	maximumSpaceCount  = 1000
)

var spaceIDPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

type catalog struct {
	Version       int            `json:"version"`
	ActiveSpaceID string         `json:"activeSpaceId"`
	Spaces        []catalogSpace `json:"spaces"`
}

type catalogSpace struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Legacy    bool      `json:"legacy"`
	CreatedAt time.Time `json:"createdAt"`
}

type PrepareFunc func(context.Context, string) error

type Registry struct {
	root        string
	catalogPath string
	lockPath    string
	catalogMu   sync.Mutex
	mutationMu  sync.Mutex
}

func Open(root string) (*Registry, error) {
	absoluteRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return nil, fmt.Errorf("resolve storage space root: %w", err)
	}
	registry := &Registry{
		root:        absoluteRoot,
		catalogPath: filepath.Join(absoluteRoot, catalogFileName),
		lockPath:    filepath.Join(absoluteRoot, registryLockName),
	}
	if err := registry.withLock(func() error {
		_, err := registry.readCatalog()
		if err == nil {
			return nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}

		id, err := randomID()
		if err != nil {
			return err
		}
		initial := catalog{
			Version:       catalogVersion,
			ActiveSpaceID: id,
			Spaces: []catalogSpace{{
				ID: id, Name: legacySpaceName, Legacy: true, CreatedAt: time.Now().UTC(),
			}},
		}
		return registry.writeCatalog(initial)
	}); err != nil {
		return nil, err
	}
	return registry, nil
}

func (r *Registry) Root() string {
	return r.root
}

func (r *Registry) Active() (Space, string, error) {
	var active Space
	var dataDir string
	err := r.withLock(func() error {
		current, err := r.readCatalog()
		if err != nil {
			return err
		}
		entry, ok := findSpace(current, current.ActiveSpaceID)
		if !ok {
			return ErrCatalogInvalid
		}
		dataDir, err = r.existingDataDir(entry)
		if err != nil {
			return err
		}
		active = publicSpace(entry, current.ActiveSpaceID)
		return nil
	})
	return active, dataDir, err
}

func (r *Registry) List() (ListResult, error) {
	result := ListResult{Spaces: make([]Space, 0)}
	err := r.withLock(func() error {
		current, err := r.readCatalog()
		if err != nil {
			return err
		}
		result.ActiveSpaceID = current.ActiveSpaceID
		result.Spaces = make([]Space, 0, len(current.Spaces))
		for _, entry := range current.Spaces {
			result.Spaces = append(result.Spaces, publicSpace(entry, current.ActiveSpaceID))
		}
		return nil
	})
	return result, err
}

func (r *Registry) Create(ctx context.Context, name string, prepare PrepareFunc) (Space, string, error) {
	name, err := normalizeName(name)
	if err != nil {
		return Space{}, "", err
	}
	if prepare == nil {
		return Space{}, "", ErrUnavailable
	}
	r.mutationMu.Lock()
	defer r.mutationMu.Unlock()

	var created Space
	var entry catalogSpace
	var dataDir string
	var activeSpaceID string
	err = r.withLock(func() error {
		current, err := r.readCatalog()
		if err != nil {
			return err
		}
		if len(current.Spaces) >= maximumSpaceCount {
			return ErrUnavailable
		}
		for _, entry := range current.Spaces {
			if strings.EqualFold(entry.Name, name) {
				return ErrNameConflict
			}
		}

		id, err := uniqueID(current)
		if err != nil {
			return err
		}
		dataDir, err = r.candidateDataDir(id)
		if err != nil {
			return err
		}
		entry = catalogSpace{ID: id, Name: name, CreatedAt: time.Now().UTC()}
		return nil
	})
	if err != nil {
		return Space{}, "", err
	}
	if err := prepare(ctx, dataDir); err != nil {
		return Space{}, "", err
	}
	if _, err := r.verifyNestedDataDir(entry.ID); err != nil {
		return Space{}, "", err
	}

	err = r.withLock(func() error {
		current, err := r.readCatalog()
		if err != nil {
			return err
		}
		if len(current.Spaces) >= maximumSpaceCount {
			return ErrUnavailable
		}
		for _, existing := range current.Spaces {
			if existing.ID == entry.ID {
				return ErrUnavailable
			}
			if strings.EqualFold(existing.Name, entry.Name) {
				return ErrNameConflict
			}
		}
		if _, err := r.verifyNestedDataDir(entry.ID); err != nil {
			return err
		}
		current.Spaces = append(current.Spaces, entry)
		if err := r.writeCatalog(current); err != nil {
			return err
		}
		activeSpaceID = current.ActiveSpaceID
		created = publicSpace(entry, current.ActiveSpaceID)
		return nil
	})
	return created, activeSpaceID, err
}

func (r *Registry) Select(ctx context.Context, id string, prepare PrepareFunc) (Space, bool, error) {
	if !spaceIDPattern.MatchString(id) {
		return Space{}, false, ErrNotFound
	}
	if prepare == nil {
		return Space{}, false, ErrUnavailable
	}
	r.mutationMu.Lock()
	defer r.mutationMu.Unlock()

	var selected Space
	var dataDir string
	var restartRequired bool
	err := r.withLock(func() error {
		current, err := r.readCatalog()
		if err != nil {
			return err
		}
		entry, ok := findSpace(current, id)
		if !ok {
			return ErrNotFound
		}
		dataDir, err = r.existingDataDir(entry)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return Space{}, false, err
	}
	if err := prepare(ctx, dataDir); err != nil {
		return Space{}, false, err
	}

	err = r.withLock(func() error {
		current, err := r.readCatalog()
		if err != nil {
			return err
		}
		entry, ok := findSpace(current, id)
		if !ok {
			return ErrNotFound
		}
		if _, err := r.existingDataDir(entry); err != nil {
			return err
		}
		if id == current.ActiveSpaceID {
			selected = publicSpace(entry, current.ActiveSpaceID)
			return nil
		}
		current.ActiveSpaceID = id
		if err := r.writeCatalog(current); err != nil {
			return err
		}
		restartRequired = true
		selected = publicSpace(entry, current.ActiveSpaceID)
		return nil
	})
	return selected, restartRequired, err
}

func (r *Registry) withLock(operation func() error) (returnErr error) {
	r.catalogMu.Lock()
	defer r.catalogMu.Unlock()

	lock, err := datalock.Acquire(r.lockPath)
	if err != nil {
		return fmt.Errorf("lock storage space catalog: %w", err)
	}
	defer func() {
		if err := lock.Release(); returnErr == nil && err != nil {
			returnErr = fmt.Errorf("release storage space catalog lock: %w", err)
		}
	}()
	return operation()
}

func (r *Registry) readCatalog() (catalog, error) {
	file, err := os.Open(r.catalogPath)
	if err != nil {
		return catalog{}, err
	}
	defer file.Close()

	encoded, err := io.ReadAll(io.LimitReader(file, maximumCatalogSize+1))
	if err != nil {
		return catalog{}, fmt.Errorf("read storage space catalog: %w", err)
	}
	if len(encoded) > maximumCatalogSize {
		return catalog{}, ErrCatalogInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var current catalog
	if err := decoder.Decode(&current); err != nil {
		return catalog{}, fmt.Errorf("%w: %v", ErrCatalogInvalid, err)
	}
	if err := ensureJSONEnd(decoder); err != nil {
		return catalog{}, err
	}
	if err := validateCatalog(current); err != nil {
		return catalog{}, err
	}
	return current, nil
}

func ensureJSONEnd(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return ErrCatalogInvalid
	}
	return nil
}

func validateCatalog(current catalog) error {
	if current.Version != catalogVersion || len(current.Spaces) == 0 || len(current.Spaces) > maximumSpaceCount || !spaceIDPattern.MatchString(current.ActiveSpaceID) {
		return ErrCatalogInvalid
	}
	ids := make(map[string]struct{}, len(current.Spaces))
	names := make([]string, 0, len(current.Spaces))
	activeFound := false
	legacyCount := 0
	for _, entry := range current.Spaces {
		if !spaceIDPattern.MatchString(entry.ID) || entry.CreatedAt.IsZero() {
			return ErrCatalogInvalid
		}
		if _, exists := ids[entry.ID]; exists {
			return ErrCatalogInvalid
		}
		ids[entry.ID] = struct{}{}
		name, err := normalizeName(entry.Name)
		if err != nil || name != entry.Name {
			return ErrCatalogInvalid
		}
		for _, existingName := range names {
			if strings.EqualFold(existingName, name) {
				return ErrCatalogInvalid
			}
		}
		names = append(names, name)
		if entry.ID == current.ActiveSpaceID {
			activeFound = true
		}
		if entry.Legacy {
			legacyCount++
		}
	}
	if !activeFound || legacyCount != 1 {
		return ErrCatalogInvalid
	}
	return nil
}

func (r *Registry) writeCatalog(current catalog) error {
	if err := validateCatalog(current); err != nil {
		return err
	}
	if err := os.MkdirAll(r.root, 0o700); err != nil {
		return fmt.Errorf("create storage space root: %w", err)
	}
	encoded, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return fmt.Errorf("encode storage space catalog: %w", err)
	}
	encoded = append(encoded, '\n')
	id, err := randomID()
	if err != nil {
		return err
	}
	temporaryPath := filepath.Join(r.root, "."+catalogFileName+"."+id+".tmp")
	file, err := os.OpenFile(temporaryPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create storage space catalog temporary file: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = file.Close()
			_ = os.Remove(temporaryPath)
		}
	}()
	if _, err := file.Write(encoded); err != nil {
		return fmt.Errorf("write storage space catalog: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync storage space catalog: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close storage space catalog: %w", err)
	}
	if err := os.Rename(temporaryPath, r.catalogPath); err != nil {
		return fmt.Errorf("commit storage space catalog: %w", err)
	}
	committed = true
	if err := syncDirectory(r.root); err != nil {
		return err
	}
	return nil
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open storage space root for sync: %w", err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil && runtime.GOOS != "windows" {
		return fmt.Errorf("sync storage space root: %w", err)
	}
	return nil
}

func normalizeName(name string) (string, error) {
	if !utf8.ValidString(name) {
		return "", ErrNameInvalid
	}
	name = norm.NFC.String(strings.TrimSpace(name))
	count := utf8.RuneCountInString(name)
	if count < 1 || count > 80 {
		return "", ErrNameInvalid
	}
	for _, value := range name {
		if unicode.IsControl(value) || unicode.Is(unicode.Cf, value) {
			return "", ErrNameInvalid
		}
	}
	return name, nil
}

func uniqueID(current catalog) (string, error) {
	for attempts := 0; attempts < 8; attempts++ {
		id, err := randomID()
		if err != nil {
			return "", err
		}
		if _, exists := findSpace(current, id); !exists {
			return id, nil
		}
	}
	return "", ErrUnavailable
}

func randomID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate storage space id: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}

func findSpace(current catalog, id string) (catalogSpace, bool) {
	for _, entry := range current.Spaces {
		if entry.ID == id {
			return entry, true
		}
	}
	return catalogSpace{}, false
}

func publicSpace(entry catalogSpace, activeSpaceID string) Space {
	return Space{
		ID: entry.ID, Name: entry.Name, Active: entry.ID == activeSpaceID,
		Legacy: entry.Legacy, CreatedAt: entry.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func (r *Registry) existingDataDir(entry catalogSpace) (string, error) {
	if entry.Legacy {
		info, err := os.Stat(r.root)
		if err != nil || !info.IsDir() {
			return "", ErrUnavailable
		}
		return r.root, nil
	}
	return r.verifyNestedDataDir(entry.ID)
}

func (r *Registry) candidateDataDir(id string) (string, error) {
	if !spaceIDPattern.MatchString(id) {
		return "", ErrUnsafePath
	}
	candidate := filepath.Join(r.root, spacesDirectory, id)
	if !pathWithin(r.root, candidate) {
		return "", ErrUnsafePath
	}
	if err := rejectSymlinkOrFile(filepath.Join(r.root, spacesDirectory), false); err != nil {
		return "", err
	}
	if err := rejectSymlinkOrFile(candidate, false); err != nil {
		return "", err
	}
	return candidate, nil
}

func (r *Registry) verifyNestedDataDir(id string) (string, error) {
	candidate, err := r.candidateDataDir(id)
	if err != nil {
		return "", err
	}
	if err := rejectSymlinkOrFile(filepath.Join(r.root, spacesDirectory), true); err != nil {
		return "", err
	}
	if err := rejectSymlinkOrFile(candidate, true); err != nil {
		return "", err
	}
	return candidate, nil
}

func rejectSymlinkOrFile(path string, requireDirectory bool) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		if requireDirectory {
			return ErrUnavailable
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect storage space path: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return ErrUnsafePath
	}
	if !info.IsDir() {
		return ErrUnsafePath
	}
	return nil
}

func pathWithin(root string, candidate string) bool {
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(candidate))
	return err == nil && relative != "." && relative != ".." && !filepath.IsAbs(relative) && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
