// Package siliconflowcn exposes the mainland China SiliconFlow provider
// entrypoint.
package siliconflowcn

import (
	"github.com/maximhq/bifrost/core/providers/siliconflow"
	"github.com/maximhq/bifrost/core/schemas"
)

// NewProvider constructs the China-region SiliconFlow provider. Wire conversion
// remains shared with the international service, while the constructor fixes
// the provider identity and regional default origin.
func NewProvider(config *schemas.ProviderConfig, logger schemas.Logger) (*siliconflow.SiliconFlowProvider, error) {
	return siliconflow.NewSiliconFlowCNProvider(config, logger)
}
