package siliconflow

import "github.com/maximhq/bifrost/core/schemas"

const (
	// defaultInternationalBaseURL is the origin of the international service,
	// whose keys are issued at cloud.siliconflow.com.
	defaultInternationalBaseURL = "https://api.siliconflow.com"
	// defaultChinaBaseURL is the origin of the mainland China service
	// whose keys are issued at cloud.siliconflow.cn.
	defaultChinaBaseURL = "https://api.siliconflow.cn"
)

// serviceProfile pins the immutable regional identity of a provider instance.
// SiliconFlow runs two independent services with separate control planes,
// accounts, API keys and model catalogues; a key issued for one is not
// documented to work against the other. The wire protocol is shared, so the
// profile only fixes provider identity and the default origin.
//
// Regional identity comes from the profile, never from network_config.base_url:
// base_url stays free for proxies, private gateways and compatible deployments,
// and must not silently reclassify which service a provider represents.
type serviceProfile struct {
	provider       schemas.ModelProvider
	defaultBaseURL string
}

var (
	internationalProfile = serviceProfile{
		provider:       schemas.SiliconFlow,
		defaultBaseURL: defaultInternationalBaseURL,
	}
	chinaProfile = serviceProfile{
		provider:       schemas.SiliconFlowCN,
		defaultBaseURL: defaultChinaBaseURL,
	}
)
