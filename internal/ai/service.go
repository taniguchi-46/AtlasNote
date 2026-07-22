package ai

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"

	"atlasnote/internal/credential"
)

type Service struct {
	repository       *Repository
	credentials      *credential.Manager
	checker          ConnectionChecker
	newCredentialRef func() (string, error)
	mu               sync.Mutex
}

func NewService(repository *Repository, credentials *credential.Manager, checker ConnectionChecker) *Service {
	if checker == nil {
		checker = NewHTTPConnectionChecker()
	}
	return &Service{
		repository:       repository,
		credentials:      credentials,
		checker:          checker,
		newCredentialRef: newCredentialReference,
	}
}

func newCredentialReference() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func (s *Service) GetSettings(ctx context.Context) ([]ProviderSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listSettings(ctx)
}

func (s *Service) listSettings(ctx context.Context) ([]ProviderSettings, error) {
	records, err := s.repository.list(ctx)
	if err != nil {
		return nil, ErrConfigurationUnavailable
	}
	byProvider := make(map[ProviderID]providerRecord, len(records))
	for _, record := range records {
		byProvider[record.ProviderID] = record
	}

	settings := make([]ProviderSettings, 0, len(supportedProviders))
	for _, providerID := range supportedProviders {
		setting := ProviderSettings{ProviderID: providerID, CredentialStatus: CredentialStatusNotConfigured}
		record, ok := byProvider[providerID]
		if !ok {
			settings = append(settings, setting)
			continue
		}
		setting.ModelID = record.ModelID
		available, availabilityErr := s.credentials.Has(record.CredentialRef)
		if availabilityErr != nil || !available {
			setting.CredentialStatus = CredentialStatusReauthenticationRequired
		} else if record.CredentialStorage == credentialStorageSessionOnly {
			setting.CredentialStatus = CredentialStatusSessionOnly
		} else {
			setting.CredentialStatus = CredentialStatusPersistent
		}
		settings = append(settings, setting)
	}
	return settings, nil
}

// Configure stores only a generated credential reference and non-secret model
// setting in SQLite. The API key itself is written to the OS store or the
// process-local fallback and is never returned from this method.
func (s *Service) Configure(ctx context.Context, input ConfigureProviderInput) ([]ProviderSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	providerID, err := normalizeProviderID(input.ProviderID)
	if err != nil {
		return nil, err
	}
	if err := validateAPIKey(input.APIKey); err != nil {
		return nil, err
	}
	modelID, err := normalizeModelID(input.ModelID)
	if err != nil {
		return nil, err
	}
	existing, err := s.repository.get(ctx, providerID)
	if err != nil {
		return nil, ErrConfigurationUnavailable
	}
	if modelID == "" && existing != nil {
		modelID = existing.ModelID
	}

	credentialRef, err := s.newCredentialRef()
	if err != nil {
		return nil, ErrCredentialUnavailable
	}
	persisted, err := s.credentials.Save(credentialRef, input.APIKey, true)
	if err != nil {
		return nil, ErrCredentialUnavailable
	}
	storage := credentialStorageSessionOnly
	if persisted {
		storage = credentialStoragePersistent
	}
	if err := s.repository.save(ctx, providerRecord{
		ProviderID:        providerID,
		ModelID:           modelID,
		CredentialRef:     credentialRef,
		CredentialStorage: storage,
	}); err != nil {
		_ = s.credentials.Delete(credentialRef)
		return nil, ErrConfigurationUnavailable
	}

	settings, settingsErr := s.listSettings(ctx)
	if settingsErr != nil {
		return nil, settingsErr
	}
	if existing != nil && existing.CredentialRef != credentialRef {
		if err := s.deleteCredential(*existing); err != nil {
			return settings, ErrCredentialCleanup
		}
	}
	return settings, nil
}

// TestConnection verifies a draft API key without touching credential storage
// or SQLite settings. Provider failures are converted to safe, stable errors.
func (s *Service) TestConnection(ctx context.Context, input TestConnectionInput) (ConnectionTestResult, error) {
	providerID, err := normalizeProviderID(input.ProviderID)
	if err != nil {
		return ConnectionTestResult{}, err
	}
	if err := validateAPIKey(input.APIKey); err != nil {
		return ConnectionTestResult{}, err
	}
	if err := s.checker.Check(ctx, providerID, input.APIKey); err != nil {
		return ConnectionTestResult{}, toSafeError(err)
	}
	return ConnectionTestResult{Success: true}, nil
}

func (s *Service) GetCredential(ctx context.Context, providerID ProviderID) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	providerID, err := normalizeProviderID(providerID)
	if err != nil {
		return "", err
	}
	record, err := s.repository.get(ctx, providerID)
	if err != nil {
		return "", ErrConfigurationUnavailable
	}
	if record == nil {
		return "", ErrReauthenticationRequired
	}
	apiKey, err := s.credentials.Get(record.CredentialRef)
	if err != nil {
		return "", ErrReauthenticationRequired
	}
	return apiKey, nil
}

func (s *Service) DeleteProvider(ctx context.Context, providerID ProviderID) ([]ProviderSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	providerID, err := normalizeProviderID(providerID)
	if err != nil {
		return nil, err
	}
	record, err := s.repository.get(ctx, providerID)
	if err != nil {
		return nil, ErrConfigurationUnavailable
	}
	if record == nil {
		return s.listSettings(ctx)
	}
	if err := s.deleteRecord(ctx, *record); err != nil {
		return nil, err
	}
	return s.listSettings(ctx)
}

func (s *Service) DeleteAll(ctx context.Context) ([]ProviderSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	records, err := s.repository.list(ctx)
	if err != nil {
		return nil, ErrConfigurationUnavailable
	}
	for _, record := range records {
		if err := s.deleteRecord(ctx, record); err != nil {
			return nil, err
		}
	}
	return s.listSettings(ctx)
}

func (s *Service) deleteRecord(ctx context.Context, record providerRecord) error {
	if err := s.deleteCredential(record); err != nil {
		return ErrCredentialUnavailable
	}
	if err := s.repository.delete(ctx, record.ProviderID); err != nil {
		return ErrConfigurationUnavailable
	}
	return nil
}

func (s *Service) deleteCredential(record providerRecord) error {
	if err := s.credentials.Delete(record.CredentialRef); err != nil {
		// A session-only credential is intentionally gone after restart. If the
		// OS store is still unavailable, there is no stored value left to remove.
		if record.CredentialStorage == credentialStorageSessionOnly && errors.Is(err, credential.ErrStoreUnavailable) {
			return nil
		}
		return err
	}
	return nil
}

func toSafeError(err error) error {
	if err == nil {
		return nil
	}
	var safeError *SafeError
	if errors.As(err, &safeError) {
		return safeError
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrTimeout
	}
	return ErrProviderUnavailable
}
