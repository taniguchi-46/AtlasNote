package ai

import "context"

func (s *Service) generateText(ctx context.Context, providerID ProviderID, modelID string, input TextGenerationInput) (TextGenerationResult, error) {
	if !s.tryStartGeneration() {
		return TextGenerationResult{}, ErrBusy
	}
	defer s.finishGeneration()

	apiKey, err := s.credentialForSummary(ctx, providerID, modelID)
	if err != nil {
		return TextGenerationResult{}, err
	}
	adapter, ok := s.adapter.(TextGenerationProviderAdapter)
	if !ok {
		return TextGenerationResult{}, ErrProviderUnavailable
	}
	operationCtx, cancel := s.operationContext(ctx)
	defer cancel()
	result, err := adapter.GenerateText(operationCtx, providerID, apiKey, input)
	if err != nil {
		return TextGenerationResult{}, toSafeError(err)
	}
	return result, nil
}
