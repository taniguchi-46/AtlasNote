package appcleanup

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"atlasnote/internal/ai"
	"atlasnote/internal/config"
	"atlasnote/internal/credential"
	"atlasnote/internal/database"
	"atlasnote/internal/datalock"
	"atlasnote/internal/sync"
)

const (
	ErrorCodeUnsupported = "ATLAS_NOTE_CLEANUP_UNSUPPORTED"
	ErrorCodeIdentity    = "ATLAS_NOTE_CLEANUP_IDENTITY"
	ErrorCodeBusy        = "ATLAS_NOTE_CLEANUP_BUSY"
	ErrorCodePath        = "ATLAS_NOTE_CLEANUP_PATH"
	ErrorCodeRecovery    = "ATLAS_NOTE_CLEANUP_RECOVERY"
	ErrorCodeCredentials = "ATLAS_NOTE_CLEANUP_CREDENTIALS"
	ErrorCodeSettings    = "ATLAS_NOTE_CLEANUP_SETTINGS"
)

var (
	ErrMaintenanceUnsupported = errors.New("maintenance cleanup is supported only on Windows")
	ErrIdentityUnavailable    = errors.New("the cleanup target user could not be verified")
	ErrUnsafeCleanupPath      = errors.New("cleanup path is outside the allowlist")
	ErrApplicationRunning     = errors.New("Atlas Note is still running")
	ErrPendingRecovery        = errors.New("a migration or recovery operation is pending")
)

// UserIdentity is obtained from the current OS token. AppDataDir is the
// roaming profile directory used by Wails' default WebView data path.
type UserIdentity struct {
	SID        string
	Account    string
	ProfileDir string
	AppDataDir string
}

// Request contains only the two optional cleanup choices. Identity, paths,
// and stores are injectable for tests; the production maintenance command
// leaves them empty so they are resolved from the current Windows user.
type Request struct {
	DeleteDisplaySettings bool
	DeleteCredentials     bool
	ExpectedSID           string
	Identity              *UserIdentity
	AppDataDir            string
	DataRoots             []string
	ArchiveRoots          []string
	CredentialFactory     CredentialStoreFactory
}

type Result struct {
	DisplaySettingsDeleted bool
	CredentialsDeleted     bool
	CredentialCount        int
	Remaining              []string
	Error                  *Error
}

type Error struct {
	Code      string
	Message   string
	Retryable bool
}

type CredentialDeleter interface {
	Delete(string) error
}

type CredentialStoreFactory func(service string) (CredentialDeleter, error)

type Service struct {
	currentUser       func() (UserIdentity, error)
	openReadOnly      func(context.Context, string) (*sql.DB, error)
	credentialFactory CredentialStoreFactory
	acquireAppLock    func() (*ApplicationLock, error)
}

func NewService() *Service {
	return &Service{
		currentUser:       currentUserIdentity,
		openReadOnly:      database.OpenReadOnly,
		credentialFactory: defaultCredentialFactory,
		acquireAppLock:    AcquireMaintenanceLock,
	}
}

func defaultCredentialFactory(service string) (CredentialDeleter, error) {
	return credential.NewKeyringStore(service), nil
}

// Run performs optional cleanup in a standalone process. It never opens the
// normal application services, never migrates a database, and never deletes a
// storage root, notes, backups, recovery workspace, or location configuration.
func (s *Service) Run(ctx context.Context, request Request) Result {
	result := Result{Remaining: make([]string, 0)}
	if !request.DeleteDisplaySettings && !request.DeleteCredentials {
		return result
	}
	if err := ctx.Err(); err != nil {
		return cleanupError(ErrorCodeBusy, "クリーンアップを完了できませんでした。Atlas Noteを終了してから再試行してください。", true, err)
	}
	if s == nil {
		return cleanupError(ErrorCodeUnsupported, "クリーンアップ機能を利用できません。", false, ErrMaintenanceUnsupported)
	}

	identity, err := s.resolveIdentity(request)
	if err != nil {
		return cleanupError(ErrorCodeIdentity, "対象ユーザーを確認できないため、追加の削除は実行しませんでした。正しい利用者でアンインストールを再実行してください。", true, err)
	}

	appDataDir, err := resolveAppDataDir(identity, request.AppDataDir)
	if err != nil {
		return cleanupError(ErrorCodeIdentity, "表示設定・キャッシュの保存先を確認できないため、追加の削除は実行しませんでした。", true, err)
	}

	appLock, err := s.acquireAppLock()
	if err != nil {
		return cleanupError(ErrorCodeBusy, "Atlas Noteが実行中のため、追加の削除を実行できません。Atlas Noteを終了して再試行してください。", true, err)
	}
	defer appLock.Release()

	dataRoots, archiveRoots, err := s.resolveCleanupRoots(identity, request.DataRoots)
	if err != nil {
		return cleanupError(ErrorCodePath, "保存空間の場所を安全に確認できないため、追加の削除は実行しませんでした。保存空間とバックアップは保持されています。", true, err)
	}
	archiveRoots = append(archiveRoots, request.ArchiveRoots...)
	if err := validateUnlinkedPaths(append(append([]string{}, dataRoots...), archiveRoots...)); err != nil {
		return cleanupError(ErrorCodePath, "保存場所の祖先パスを安全に確認できないため、追加削除は実行しませんでした。", true, err)
	}
	var deletion *preparedDeletion
	if request.DeleteDisplaySettings {
		webviewPath, pathErr := knownWebViewUserDataPath(appDataDir)
		protected := append(append([]string{}, dataRoots...), archiveRoots...)
		protected = append(protected, filepath.Join(identity.AppDataDir, "AtlasNote"))
		if pathErr == nil {
			pathErr = validateCleanupSeparation(webviewPath, protected)
		}
		if pathErr == nil {
			deletion, pathErr = prepareCacheDeletion(webviewPath)
		}
		if pathErr != nil {
			return cleanupError(ErrorCodeSettings, "削除先に保存データ、未知の項目、リンクまたは使用中の項目があるため、追加削除は実行しませんでした。", true, pathErr)
		}
		defer deletion.Close()
	}
	if err := ensureNoPendingOperations(identity, dataRoots, archiveRoots); err != nil {
		if errors.Is(err, ErrPendingRecovery) {
			return cleanupError(ErrorCodeRecovery, "保留中の移行または復旧処理があるため、追加の削除は実行しませんでした。Atlas Noteを起動して復旧を完了してから再試行してください。ノート、バックアップ、保存空間、復旧情報は保持されています。", true, err)
		}
		return cleanupError(ErrorCodePath, "移行・復旧情報を安全に確認できないため、追加の削除は実行しませんでした。保存空間とバックアップは保持されています。", true, err)
	}
	databasePaths, err := collectDatabasePaths(dataRoots)
	if err != nil {
		return cleanupError(ErrorCodePath, "保存空間を安全に確認できないため、追加の削除は実行しませんでした。保存空間とバックアップは保持されています。", true, err)
	}
	locks, err := acquireDataLocks(databasePaths)
	if err != nil {
		return cleanupError(ErrorCodeBusy, "保存空間が使用中のため、追加の削除を実行できません。Atlas Noteを終了して再試行してください。", true, err)
	}
	defer releaseDataLocks(locks)

	var refs credentialReferences
	if request.DeleteCredentials {
		refs, err = s.collectCredentialReferences(ctx, databasePaths)
		if err != nil {
			return cleanupError(ErrorCodeCredentials, "この利用者のAtlas Note用認証情報を確認できないため、認証情報は削除しませんでした。正しい利用者で再試行してください。", true, err)
		}
	}

	if request.DeleteCredentials {
		if err := s.deleteCredentials(refs); err != nil {
			return cleanupError(ErrorCodeCredentials, "一部の認証情報を削除できませんでした。Atlas Noteを終了し、資格情報ストアを利用できる状態で再試行してください。", true, err)
		}
		result.CredentialsDeleted = true
		result.CredentialCount = refs.count()
	}

	if request.DeleteDisplaySettings {
		if err := deletion.Remove(); err != nil {
			return cleanupError(ErrorCodeSettings, "表示設定・キャッシュを完全に削除できませんでした。残った項目を確認してから再試行してください。ノート、バックアップ、保存空間、復旧情報は削除していません。", true, err)
		}
		result.DisplaySettingsDeleted = true
	}

	return result
}

func (s *Service) resolveIdentity(request Request) (UserIdentity, error) {
	identity := UserIdentity{}
	if request.Identity != nil {
		identity = *request.Identity
	} else {
		if s.currentUser == nil {
			return identity, ErrIdentityUnavailable
		}
		var err error
		identity, err = s.currentUser()
		if err != nil {
			return UserIdentity{}, err
		}
	}
	if strings.TrimSpace(identity.SID) == "" || strings.TrimSpace(identity.Account) == "" || strings.TrimSpace(identity.ProfileDir) == "" || strings.TrimSpace(identity.AppDataDir) == "" {
		return UserIdentity{}, ErrIdentityUnavailable
	}
	if request.Identity == nil && strings.TrimSpace(request.ExpectedSID) == "" {
		return UserIdentity{}, ErrIdentityUnavailable
	}
	if expected := strings.TrimSpace(request.ExpectedSID); expected != "" && expected != identity.SID {
		return UserIdentity{}, fmt.Errorf("cleanup user mismatch: expected %q", expected)
	}
	return identity, nil
}

func resolveAppDataDir(identity UserIdentity, requested string) (string, error) {
	appDataDir := identity.AppDataDir
	if strings.TrimSpace(requested) != "" {
		appDataDir = requested
	}
	absolute, err := filepath.Abs(filepath.Clean(appDataDir))
	if err != nil || !filepath.IsAbs(absolute) || !samePath(absolute, identity.AppDataDir) {
		return "", ErrUnsafeCleanupPath
	}
	return absolute, nil
}

func (s *Service) resolveCleanupRoots(identity UserIdentity, requested []string) ([]string, []string, error) {
	if requested != nil {
		dataRoots, err := normalizeRoots(requested)
		return dataRoots, nil, err
	}
	// Nonstandard launch overrides cannot safely be rediscovered by an elevated
	// uninstaller. Keep everything rather than guessing which bootstrap to use.
	for _, name := range []string{"ATLAS_NOTE_DATA_DIR", "ATLAS_NOTE_STORAGE_LOCATIONS_FILE", "ATLAS_NOTE_DEFAULT_DATA_ROOT"} {
		if strings.TrimSpace(os.Getenv(name)) != "" {
			return nil, nil, ErrUnsafeCleanupPath
		}
	}

	dataRoots := make([]string, 0, 3)
	archiveRoots := make([]string, 0, 3)
	configPath := filepath.Join(identity.AppDataDir, "AtlasNote", "storage-locations.json")
	if err := validateUnlinkedPaths([]string{configPath}); err != nil {
		return nil, nil, err
	}
	locations, err := config.LoadStorageLocationsForRecoveryFrom(configPath)
	if err == nil {
		dataRoots = append(dataRoots, locations.DataRoot)
		archiveRoot := locations.BackupRoot
		if strings.TrimSpace(archiveRoot) == "" {
			archiveRoot = locations.DataRoot
		}
		archiveRoots = append(archiveRoots, archiveRoot)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, nil, err
	}
	if defaultRoot, defaultErr := config.DefaultDataRoot(); defaultErr == nil {
		dataRoots = append(dataRoots, defaultRoot)
		archiveRoots = append(archiveRoots, defaultRoot)
	} else {
		return nil, nil, defaultErr
	}
	// Before storage locations were introduced, the legacy data root was the
	// user-config directory. It is read only as a candidate DB location; the
	// directory itself is never deleted.
	legacyRoot := filepath.Join(identity.AppDataDir, "AtlasNote")
	dataRoots = append(dataRoots, legacyRoot)
	archiveRoots = append(archiveRoots, legacyRoot)
	normalizedDataRoots, err := normalizeRoots(dataRoots)
	if err != nil {
		return nil, nil, err
	}
	normalizedArchiveRoots, err := normalizeRoots(archiveRoots)
	if err != nil {
		return nil, nil, err
	}
	return normalizedDataRoots, normalizedArchiveRoots, nil
}

func (s *Service) resolveDataRoots(identity UserIdentity, requested []string) ([]string, error) {
	dataRoots, _, err := s.resolveCleanupRoots(identity, requested)
	return dataRoots, err
}

func normalizeRoots(roots []string) ([]string, error) {
	seen := make(map[string]struct{}, len(roots))
	result := make([]string, 0, len(roots))
	for _, root := range roots {
		if strings.TrimSpace(root) == "" {
			continue
		}
		absolute, err := filepath.Abs(filepath.Clean(root))
		if err != nil || !filepath.IsAbs(absolute) {
			return nil, ErrUnsafeCleanupPath
		}
		key := strings.ToLower(absolute)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, absolute)
	}
	return result, nil
}

func collectDatabasePaths(roots []string) ([]string, error) {
	paths := make([]string, 0)
	for _, root := range roots {
		info, err := os.Lstat(root)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !info.IsDir() || isUnsafeFileInfo(root, info) {
			return nil, ErrUnsafeCleanupPath
		}
		appendDatabasePath := func(path string) error {
			info, err := os.Lstat(path)
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			if err != nil {
				return err
			}
			if isUnsafeFileInfo(path, info) || !info.Mode().IsRegular() {
				return ErrUnsafeCleanupPath
			}
			paths = append(paths, path)
			return nil
		}
		if err := appendDatabasePath(filepath.Join(root, "atlasnote.db")); err != nil {
			return nil, err
		}

		spacesPath := filepath.Join(root, "spaces")
		spacesInfo, err := os.Lstat(spacesPath)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if isUnsafeFileInfo(spacesPath, spacesInfo) || !spacesInfo.IsDir() {
			return nil, ErrUnsafeCleanupPath
		}
		entries, err := os.ReadDir(spacesPath)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if !spaceIDPattern.MatchString(entry.Name()) {
				return nil, ErrUnsafeCleanupPath
			}
			spacePath := filepath.Join(spacesPath, entry.Name())
			spaceInfo, err := os.Lstat(spacePath)
			if err != nil {
				return nil, err
			}
			if isUnsafeFileInfo(spacePath, spaceInfo) || !spaceInfo.IsDir() {
				return nil, ErrUnsafeCleanupPath
			}
			if err := appendDatabasePath(filepath.Join(spacePath, "atlasnote.db")); err != nil {
				return nil, err
			}
		}
	}
	sort.Strings(paths)
	return uniqueStrings(paths), nil
}

func acquireDataLocks(databasePaths []string) ([]*datalock.Lock, error) {
	locks := make([]*datalock.Lock, 0, len(databasePaths))
	for _, databasePath := range databasePaths {
		lockPath := filepath.Join(filepath.Dir(databasePath), "atlasnote.lock")
		if info, err := os.Lstat(lockPath); err == nil {
			if isUnsafeFileInfo(lockPath, info) || !info.Mode().IsRegular() {
				releaseDataLocks(locks)
				return nil, ErrUnsafeCleanupPath
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			releaseDataLocks(locks)
			return nil, err
		}
		lock, err := datalock.Acquire(lockPath)
		if err != nil {
			releaseDataLocks(locks)
			return nil, errors.Join(ErrApplicationRunning, err)
		}
		locks = append(locks, lock)
	}
	return locks, nil
}

func releaseDataLocks(locks []*datalock.Lock) {
	for index := len(locks) - 1; index >= 0; index-- {
		_ = locks[index].Release()
	}
}

type credentialReferences struct {
	byService map[string]map[string]struct{}
}

func (refs credentialReferences) count() int {
	count := 0
	for _, values := range refs.byService {
		count += len(values)
	}
	return count
}

func (s *Service) collectCredentialReferences(ctx context.Context, databasePaths []string) (credentialReferences, error) {
	refs := credentialReferences{byService: map[string]map[string]struct{}{
		ai.CredentialStoreServiceName: {},
		sync.ServiceName:              {},
	}}
	for _, path := range databasePaths {
		db, err := s.openReadOnly(ctx, path)
		if err != nil {
			return credentialReferences{}, err
		}
		if err := readCredentialReferences(ctx, db, refs.byService); err != nil {
			_ = db.Close()
			return credentialReferences{}, err
		}
		if err := db.Close(); err != nil {
			return credentialReferences{}, err
		}
	}
	return refs, nil
}

func readCredentialReferences(ctx context.Context, db *sql.DB, byService map[string]map[string]struct{}) error {
	rows, err := db.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type = 'table' AND name IN ('ai_provider_settings', 'sync_connections')")
	if err != nil {
		return err
	}
	tableNames := make([]string, 0, 2)
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			_ = rows.Close()
			return err
		}
		tableNames = append(tableNames, tableName)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	for _, tableName := range tableNames {
		serviceName := ""
		switch tableName {
		case "ai_provider_settings":
			serviceName = ai.CredentialStoreServiceName
		case "sync_connections":
			serviceName = sync.ServiceName
		}
		if serviceName == "" {
			continue
		}
		credentialRows, err := db.QueryContext(ctx, "SELECT credential_ref FROM "+tableName+" WHERE credential_ref <> ''")
		if err != nil {
			return err
		}
		for credentialRows.Next() {
			var ref string
			if err := credentialRows.Scan(&ref); err != nil {
				_ = credentialRows.Close()
				return err
			}
			if !credentialReferencePattern.MatchString(ref) {
				_ = credentialRows.Close()
				return fmt.Errorf("unsafe credential reference")
			}
			byService[serviceName][ref] = struct{}{}
		}
		if err := credentialRows.Err(); err != nil {
			_ = credentialRows.Close()
			return err
		}
		if err := credentialRows.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) deleteCredentials(refs credentialReferences) error {
	factory := s.credentialFactory
	if factory == nil {
		factory = defaultCredentialFactory
	}
	services := make([]string, 0, len(refs.byService))
	for serviceName := range refs.byService {
		services = append(services, serviceName)
	}
	sort.Strings(services)
	for _, serviceName := range services {
		if len(refs.byService[serviceName]) == 0 {
			continue
		}
		store, err := factory(serviceName)
		if err != nil {
			return err
		}
		refsForService := make([]string, 0, len(refs.byService[serviceName]))
		for ref := range refs.byService[serviceName] {
			refsForService = append(refsForService, ref)
		}
		sort.Strings(refsForService)
		for _, ref := range refsForService {
			if err := store.Delete(ref); err != nil && !errors.Is(err, credential.ErrNotFound) {
				return err
			}
		}
	}
	return nil
}

func cleanupError(code string, message string, retryable bool, _ error) Result {
	return Result{Remaining: make([]string, 0), Error: &Error{Code: code, Message: message, Retryable: retryable}}
}

var (
	spaceIDPattern             = regexp.MustCompile(`^[a-f0-9]{32}$`)
	credentialReferencePattern = regexp.MustCompile(`^[a-f0-9]{32}$`)
)

func samePath(left string, right string) bool {
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}

func uniqueStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	result := values[:1]
	for _, value := range values[1:] {
		if !samePath(result[len(result)-1], value) {
			result = append(result, value)
		}
	}
	return result
}
