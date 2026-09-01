package chatgpt_test

import (
	"context"

	credentialprovider "PandoraHelper/internal/provider/credential"
)

// RecordHealth keeps the provider test double aligned with credential.Provider.
// WebProvider tests do not persist credential health, so a no-op implementation
// is sufficient for this isolated protocol suite.
func (*testCredentialProvider) RecordHealth(context.Context, uint, credentialprovider.Health) error {
	return nil
}
