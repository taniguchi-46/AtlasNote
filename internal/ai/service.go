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
	adapter          ProviderAdapter
	newCredentialRef func() (string, error)
	mu               sync.Mutex
	generationMu     sync.Mutex
	generating       bool
	shutdownCtx      context.Context
	shutdownCancel   context.CancelFunc
}

func NewService(repository *Repository, credentials *credential.Manager, checker ConnectionChecker) *Service {
	if checker == nil {
		checker = NewHTTPProviderAdapter()
	}
	adapter, ok := checker.(ProviderAdapter)
	if !ok {
		adapter = NewHTTPProviderAdapter()
	}
	return newService(repository, credentials, checker, adapter)
}

// NewServiceWithAdapter is the D-05 constructor used by the application. The
// legacy NewService constructor remains for D-02 compatibility tests.
func NewServiceWithAdapter(repository *Repository, credentials *credential.Manager, adapter ProviderAdapter) *Service {
	if adapter == nil {
		adapter = NewHTTPProviderAdapter()
	}
	return newService(repository, credentials, adapterConnectionChecker{adapter: adapter}, adapter)
}

type adapterConnectionChecker struct {
	adapter ProviderAdapter
}

func (c adapterConnectionChecker) Check(ctx context.Context, providerID ProviderID, apiKey string) error {
	return c.adapter.CheckConnection(ctx, providerID, apiKey)
}

func newService(repository *Repository, credentials *credential.Manager, checker ConnectionChecker, adapter ProviderAdapter) *Service {
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	return &Service{
		repository:       repository,
		credentials:      credentials,
		checker:          checker,
		adapter:          adapter,
		newCredentialRef: newCredentialReference,
		shutdownCtx:      shutdownCtx,
		shutdownCancel:   shutdownCancel,
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
	operationCtx, cancel := s.operationContext(ctx)
	defer cancel()
	if err := s.checker.Check(operationCtx, providerID, input.APIKey); err != nil {
		return ConnectionTestResult{}, toSafeError(err)
	}
	return ConnectionTestResult{Success: true}, nil
}

// ListModels verifies and uses a draft API key without persisting either the
// key or the returned model metadata.
func (s *Service) ListModels(ctx context.Context, input ListModelsInput) (ModelListResult, error) {
	providerID, err := normalizeProviderID(input.ProviderID)
	if err != nil {
		return ModelListResult{}, err
	}
	if err := validateAPIKey(input.APIKey); err != nil {
		return ModelListResult{}, err
	}
	operationCtx, cancel := s.operationContext(ctx)
	defer cancel()
	result, err := s.adapter.ListModels(operationCtx, providerID, input.APIKey)
	if err != nil {
		return ModelListResult{}, toSafeError(err)
	}
	return result, nil
}

// GenerateSummary resolves a saved credential internally and deliberately
// keeps the generated result outside Markdown, SQLite, and sync state.
func (s *Service) GenerateSummary(ctx context.Context, input GenerateSummaryInput) (SummaryResult, error) {
	normalized, err := normalizeSummaryInput(input)
	if err != nil {
		return SummaryResult{}, err
	}
	if !s.tryStartGeneration() {
		return SummaryResult{}, ErrBusy
	}
	defer s.finishGeneration()

	apiKey, err := s.credentialForSummary(ctx, normalized.ProviderID, normalized.ModelID)
	if err != nil {
		return SummaryResult{}, err
	}
	operationCtx, cancel := s.operationContext(ctx)
	defer cancel()
	result, err := s.adapter.GenerateSummary(operationCtx, normalized.ProviderID, apiKey, normalized)
	if err != nil {
		return SummaryResult{}, toSafeError(err)
	}
	return result, nil
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

func (s *Service) credentialForSummary(ctx context.Context, providerID ProviderID, modelID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, err := s.repository.get(ctx, providerID)
	if err != nil {
		return "", ErrConfigurationUnavailable
	}
	if record == nil {
		return "", ErrReauthenticationRequired
	}
	configuredModelID, err := normalizeSummaryModelID(providerID, record.ModelID)
	if err != nil || configuredModelID != modelID {
		return "", ErrModelUnavailable
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

func (s *Service) tryStartGeneration() bool {
	s.generationMu.Lock()
	defer s.generationMu.Unlock()
	if s.generating {
		return false
	}
	s.generating = true
	return true
}

func (s *Service) finishGeneration() {
	s.generationMu.Lock()
	s.generating = false
	s.generationMu.Unlock()
}

func (s *Service) operationContext(ctx context.Context) (context.Context, func()) {
	if ctx == nil {
		ctx = context.Background()
	}
	operationCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(s.shutdownCtx, cancel)
	return operationCtx, func() {
		stop()
		cancel()
	}
}

// Shutdown is called only during application shutdown. v1 has no
// user-initiated cancellation, but this stops any in-flight provider request.
func (s *Service) Shutdown() {
	if s.shutdownCancel != nil {
		s.shutdownCancel()
	}
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
