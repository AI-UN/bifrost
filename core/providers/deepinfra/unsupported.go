package deepinfra

import (
	providerUtils "github.com/maximhq/bifrost/core/providers/utils"
	"github.com/maximhq/bifrost/core/schemas"
)

// ModelRetrieve is not supported by DeepInfra's model-listing API.
func (provider *DeepInfraProvider) ModelRetrieve(_ *schemas.BifrostContext, _ schemas.Key, _ *schemas.BifrostModelRetrieveRequest) (*schemas.BifrostModelRetrieveResponse, *schemas.BifrostError) {
	return nil, providerUtils.NewUnsupportedOperationError(schemas.ModelRetrieveRequest, provider.GetProviderKey())
}
