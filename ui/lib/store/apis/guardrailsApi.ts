/**
 * Guardrails RTK Query API.
 * Covers detection profiles per provider and the rules that attach them.
 */

import {
	GetGuardrailRulesParams,
	GetGuardrailRulesResponse,
	GuardrailPresidioEntitiesRequest,
	GuardrailProfile,
	GuardrailProfileRequest,
	GuardrailProvider,
	GuardrailProviderGroup,
	GuardrailRule,
	GuardrailRuleRequest,
	GuardrailVerifyRequest,
	SecretRuleInfo,
	UpdateGuardrailProfileRequest,
} from "@/lib/types/guardrails";
import { baseApi } from "./baseApi";

export const guardrailsApi = baseApi.injectEndpoints({
	endpoints: (builder) => ({
		// All three provider groups in one call; the API always returns the fixed three.
		getGuardrails: builder.query<GuardrailProviderGroup[], void>({
			query: () => ({ url: "/guardrails" }),
			providesTags: ["Guardrails"],
		}),

		getGuardrailProvider: builder.query<GuardrailProviderGroup, GuardrailProvider>({
			query: (provider) => ({ url: `/guardrails/${provider}` }),
			providesTags: ["Guardrails"],
		}),

		createGuardrailProfile: builder.mutation<GuardrailProfile, { provider: GuardrailProvider; data: GuardrailProfileRequest }>({
			query: ({ provider, data }) => ({ url: `/guardrails/${provider}`, method: "POST", body: data }),
			transformResponse: (response: { config: GuardrailProfile }) => response.config,
			invalidatesTags: ["Guardrails"],
		}),

		updateGuardrailProfile: builder.mutation<GuardrailProfile, { provider: GuardrailProvider; data: UpdateGuardrailProfileRequest }>({
			query: ({ provider, data }) => ({ url: `/guardrails/${provider}`, method: "PUT", body: data }),
			transformResponse: (response: { config: GuardrailProfile }) => response.config,
			invalidatesTags: ["Guardrails"],
		}),

		deleteGuardrailProfile: builder.mutation<void, { provider: GuardrailProvider; id: number }>({
			query: ({ provider, id }) => ({ url: `/guardrails/${provider}`, method: "DELETE", body: { id } }),
			invalidatesTags: ["Guardrails"],
		}),

		verifyGuardrailProfile: builder.mutation<{ ok: boolean }, { provider: GuardrailProvider; data: GuardrailVerifyRequest }>({
			query: ({ provider, data }) => ({ url: `/guardrails/${provider}/verify`, method: "POST", body: data }),
		}),

		// Gitleaks rule catalog, for the secrets profile's disabled-rules and entropy editors.
		getGuardrailSecretRules: builder.query<{ rules: SecretRuleInfo[] }, void>({
			query: () => ({ url: "/guardrails/secrets/rules" }),
		}),

		// Fork extension: the server fetches the analyzer's entity list, since a
		// self-hosted analyzer is often unreachable from the browser.
		getGuardrailPresidioEntities: builder.mutation<{ entities: string[] }, GuardrailPresidioEntitiesRequest>({
			query: (data) => ({ url: "/guardrails/presidio/supported-entities", method: "POST", body: data }),
		}),

		getGuardrailRules: builder.query<GetGuardrailRulesResponse, GetGuardrailRulesParams | void>({
			query: (params) => ({
				url: "/guardrails/rules",
				params: {
					...(params?.limit !== undefined && { limit: params.limit }),
					...(params?.offset !== undefined && { offset: params.offset }),
				},
			}),
			providesTags: ["GuardrailRules"],
		}),

		createGuardrailRule: builder.mutation<GuardrailRule, GuardrailRuleRequest>({
			query: (data) => ({ url: "/guardrails/rules", method: "POST", body: data }),
			transformResponse: (response: { rule: GuardrailRule }) => response.rule,
			invalidatesTags: ["GuardrailRules"],
		}),

		updateGuardrailRule: builder.mutation<GuardrailRule, { id: number; data: GuardrailRuleRequest }>({
			query: ({ id, data }) => ({ url: `/guardrails/rules/${id}`, method: "PUT", body: data }),
			transformResponse: (response: { rule: GuardrailRule }) => response.rule,
			invalidatesTags: ["GuardrailRules"],
		}),

		deleteGuardrailRule: builder.mutation<void, number>({
			query: (id) => ({ url: `/guardrails/rules/${id}`, method: "DELETE" }),
			invalidatesTags: ["GuardrailRules"],
		}),

		toggleGuardrailRule: builder.mutation<GuardrailRule, { id: number; enabled: boolean }>({
			query: ({ id, enabled }) => ({ url: `/guardrails/rules/${id}/toggle`, method: "PATCH", body: { enabled } }),
			transformResponse: (response: { rule: GuardrailRule }) => response.rule,
			invalidatesTags: ["GuardrailRules"],
		}),
	}),
});

export const {
	useGetGuardrailsQuery,
	useGetGuardrailProviderQuery,
	useCreateGuardrailProfileMutation,
	useUpdateGuardrailProfileMutation,
	useDeleteGuardrailProfileMutation,
	useVerifyGuardrailProfileMutation,
	useGetGuardrailSecretRulesQuery,
	useGetGuardrailPresidioEntitiesMutation,
	useGetGuardrailRulesQuery,
	useCreateGuardrailRuleMutation,
	useUpdateGuardrailRuleMutation,
	useDeleteGuardrailRuleMutation,
	useToggleGuardrailRuleMutation,
} = guardrailsApi;