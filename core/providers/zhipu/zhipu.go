// Package zhipu exposes the mainland China Zhipu AI provider entrypoint.
package zhipu

import (
	"github.com/maximhq/bifrost/core/providers/zai"
	"github.com/maximhq/bifrost/core/schemas"
)

// NewProvider constructs the China-region Zhipu AI provider. Wire conversion
// remains shared with zai, while the constructor fixes the provider identity and
// regional capability profile.
func NewProvider(config *schemas.ProviderConfig, logger schemas.Logger) (*zai.ZAIProvider, error) {
	return zai.NewZhipuProvider(config, logger)
}
