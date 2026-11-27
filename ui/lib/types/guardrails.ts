/**
 * Guardrails type definitions.
 * Mirrors the management API shapes in the OSS guardrails contract: profiles are
 * returned with their provider config flattened onto the profile view.
 */

import i18n from "@/lib/i18n";
import type { ParseKeys } from "i18next";
import type { SecretVar } from "./schemas";

export const GUARDRAIL_PROVIDERS = ["secrets", "regex", "presidio"] as const;
export type GuardrailProvider = (typeof GUARDRAIL_PROVIDERS)[number];

export type GuardrailAction = "detect_only" | "block" | "redact";
export type GuardrailRedactionStrategy = "replace" | "mask" | "hash";
// v1 supports runtime redaction only; logs_only and reversible modes are rejected by the API.
export type GuardrailRedactionMode = "runtime";
export const GUARDRAIL_REDACTION_MODE: GuardrailRedactionMode = "runtime";

export type GuardrailTarget = "llm";
export type GuardrailApplyTo = "input";

function option<T extends string>(value: T, labelKey: ParseKeys<"governance">) {
	return {
		value,
		get label(): string {
			return i18n.t(labelKey, { ns: "governance" });
		},
	};
}

export const GUARDRAIL_ACTION_OPTIONS = [
	option<GuardrailAction>("detect_only", "guardrails.common.actionDetectOnly"),
	option<GuardrailAction>("block", "guardrails.common.actionBlock"),
	option<GuardrailAction>("redact", "guardrails.common.actionRedact"),
];

export const REDACTION_STRATEGY_OPTIONS = [
	option<GuardrailRedactionStrategy>("replace", "guardrails.common.strategyReplace"),
	option<GuardrailRedactionStrategy>("mask", "guardrails.common.strategyMask"),
	option<GuardrailRedactionStrategy>("hash", "guardrails.common.strategyHash"),
];

export interface GuardrailRegexPattern {
	pattern: string;
	description: string;
	entity_type: string;
	/** RE2 inline flags, e.g. "i" or "im". */
	flags?: string;
	action?: GuardrailAction;
	redaction_strategy?: GuardrailRedactionStrategy;
	redaction_mode?: GuardrailRedactionMode;
	allowed_values?: string[];
}

export interface SecretRuleInfo {
	id: string;
	description: string;
	entropy: number;
}

// One provider configuration. The API flattens the provider config onto the
// profile view, so every config field is optional here and each form reads the
// subset its provider owns.
export interface GuardrailProfile {
	id: number;
	name: string;
	enabled: boolean;
	timeout?: number;
	// secrets
	action?: GuardrailAction;
	redaction_strategy?: GuardrailRedactionStrategy;
	redaction_mode?: GuardrailRedactionMode;
	ignored_secret_keywords?: string[];
	disabled_rules?: string[];
	rule_entropy?: Record<string, number>;
	// regex
	patterns?: GuardrailRegexPattern[];
	// presidio
	analyzer_url?: string;
	api_key?: SecretVar;
	language?: string;
	score_threshold?: number;
	entities?: string[];
	fail_open?: boolean;
}

export interface GuardrailProviderGroup {
	name: GuardrailProvider;
	configs: GuardrailProfile[];
}

export interface GuardrailProfileRequest {
	name: string;
	enabled: boolean;
	config: Record<string, unknown>;
}

export interface UpdateGuardrailProfileRequest {
	id: number;
	name?: string;
	enabled?: boolean;
	config?: Record<string, unknown>;
}

export interface GuardrailVerifyRequest {
	id?: number;
	name: string;
	config: Record<string, unknown>;
}

export interface GuardrailPresidioEntitiesRequest {
	id?: number;
	config: { analyzer_url: string; api_key?: SecretVar; language?: string };
}

export interface GuardrailRule {
	id: number;
	name: string;
	description: string;
	enabled: boolean;
	target: GuardrailTarget;
	celExpression: string;
	applyTo: GuardrailApplyTo;
	samplingRate: number;
	timeout: number;
	/** Profile references as "<provider>:<id>", in execution order. */
	selectedGuardrailProfiles: string[];
	query?: unknown;
	createdAt?: string;
	updatedAt?: string;
}

export type GuardrailRuleRequest = Omit<GuardrailRule, "id" | "createdAt" | "updatedAt" | "query">;

export interface GetGuardrailRulesParams {
	limit?: number;
	offset?: number;
}

export interface GetGuardrailRulesResponse {
	count: number;
	limit: number;
	offset: number;
	rules: GuardrailRule[];
}

// PII Detection template from the custom-regex documentation. v1 redacts at
// runtime only, so the template uses redaction_mode "runtime".
export const GUARDRAIL_PII_TEMPLATE: GuardrailRegexPattern[] = [
	{
		pattern: "\\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\\.[A-Z]{2,}\\b",
		description: "Email address",
		entity_type: "EMAIL",
		flags: "i",
		action: "redact",
		redaction_strategy: "replace",
		redaction_mode: "runtime",
	},
	{
		pattern: "\\b(?:\\+?1[-.\\s]?)?(?:\\(?\\d{3}\\)?[-.\\s]?)\\d{3}[-.\\s]?\\d{4}\\b",
		description: "US phone number",
		entity_type: "PHONE_NUMBER",
		action: "redact",
		redaction_strategy: "replace",
		redaction_mode: "runtime",
	},
	{
		pattern: "\\b\\d{3}-\\d{2}-\\d{4}\\b",
		description: "US Social Security Number",
		entity_type: "US_SSN",
		action: "redact",
		redaction_strategy: "replace",
		redaction_mode: "runtime",
	},
	{
		pattern: "\\b(?:\\d[ -]?){13,19}\\b",
		description: "Credit card-like number",
		entity_type: "CREDIT_CARD",
		action: "redact",
		redaction_strategy: "replace",
		redaction_mode: "runtime",
	},
	{
		pattern: "\\b(?:25[0-5]|2[0-4]\\d|1\\d\\d|[1-9]?\\d)(?:\\.(?:25[0-5]|2[0-4]\\d|1\\d\\d|[1-9]?\\d)){3}\\b",
		description: "IPv4 address",
		entity_type: "IP_ADDRESS",
		action: "redact",
		redaction_strategy: "replace",
		redaction_mode: "runtime",
	},
];

export const DEFAULT_SECRETS_CONFIG = {
	action: "block" as GuardrailAction,
	redaction_strategy: "replace" as GuardrailRedactionStrategy,
	redaction_mode: GUARDRAIL_REDACTION_MODE,
	ignored_secret_keywords: [] as string[],
	disabled_rules: [] as string[],
	rule_entropy: {} as Record<string, number>,
};

export const DEFAULT_REGEX_CONFIG = {
	patterns: [] as GuardrailRegexPattern[],
};

export const DEFAULT_PRESIDIO_CONFIG = {
	analyzer_url: "",
	language: "en",
	score_threshold: 0.5,
	entities: [] as string[],
	action: "detect_only" as GuardrailAction,
	redaction_strategy: "replace" as GuardrailRedactionStrategy,
	redaction_mode: GUARDRAIL_REDACTION_MODE,
	fail_open: false,
};

export const DEFAULT_GUARDRAIL_TIMEOUT = 30;

/** Splits a comma-separated input into trimmed, non-empty values. */
export const splitList = (value: string): string[] =>
	value
		.split(",")
		.map((entry) => entry.trim())
		.filter(Boolean);