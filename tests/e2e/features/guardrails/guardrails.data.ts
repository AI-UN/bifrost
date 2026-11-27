import type { GuardrailProfileForm } from "./pages/guardrailsProviders.page";
import type { GuardrailRuleForm } from "./pages/guardrailsRules.page";

/** Entity type the test pattern reports, which names the placeholder a redaction writes. */
export const TEST_ENTITY_TYPE = "E2E_SECRET";

/** A stand-in secret the harness can look for: a fixed prefix and eight digits. */
export const TEST_SECRET_PATTERN = "BFGR-E2E-[0-9]{8}";

const uniqueSuffix = (): string => `${Date.now()}-${Math.floor(Math.random() * 1e6)}`;

export function createRegexProfileData(
  overrides: Partial<GuardrailProfileForm> = {}
): GuardrailProfileForm {
  return {
    name: `guardrails-regex-${uniqueSuffix()}`,
    ...overrides,
  };
}

export function createGuardrailRuleData(
  overrides: Partial<GuardrailRuleForm> = {}
): GuardrailRuleForm {
  return {
    name: `guardrails-rule-${uniqueSuffix()}`,
    description: "E2E guardrail rule",
    // "true" is what the sheet's "All requests" quick action writes.
    celExpression: "true",
    profileIds: [],
    ...overrides,
  };
}
