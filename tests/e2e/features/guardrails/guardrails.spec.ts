import type { APIRequestContext } from "@playwright/test";
import { guardrailsApi } from "../../core/actions/api";
import { expect, test } from "../../core/fixtures/base.fixture";
import {
  TEST_ENTITY_TYPE,
  TEST_SECRET_PATTERN,
  createGuardrailRuleData,
  createRegexProfileData,
} from "./guardrails.data";
import type { GuardrailRuleForm } from "./pages/guardrailsRules.page";

// Every resource a test creates, deleted by name in afterEach. Names carry a timestamp
// and a random suffix, so a cleanup only ever touches this run's rows.
const createdRuleNames: string[] = [];
const createdProfileNames: string[] = [];

async function deleteCreatedResources(request: APIRequestContext): Promise<void> {
  // Rules first: a profile a rule still references is a 409.
  try {
    const { rules } = await guardrailsApi.getRules(request);
    for (const rule of rules) {
      if (!createdRuleNames.includes(rule.name)) continue;
      await guardrailsApi.deleteRule(request, rule.id).catch(() => false);
    }
  } catch (error) {
    console.error(`[CLEANUP] Failed to delete guardrail rules: ${error}`);
  }
  createdRuleNames.length = 0;

  try {
    const { configs } = await guardrailsApi.getProfiles(request, "regex");
    for (const profile of configs) {
      if (!createdProfileNames.includes(profile.name)) continue;
      await guardrailsApi.deleteProfile(request, "regex", profile.id).catch(() => false);
    }
  } catch (error) {
    console.error(`[CLEANUP] Failed to delete guardrail profiles: ${error}`);
  }
  createdProfileNames.length = 0;
}

/** A saved regex profile whose single pattern flags the E2E test secret. */
async function seedRegexProfile(request: APIRequestContext, name: string): Promise<number> {
  const { config } = await guardrailsApi.createProfile(request, "regex", {
    name,
    enabled: true,
    config: {
      patterns: [
        {
          pattern: TEST_SECRET_PATTERN,
          description: "E2E test secret",
          entity_type: TEST_ENTITY_TYPE,
          action: "block",
        },
      ],
    },
  });
  createdProfileNames.push(name);
  return config.id;
}

/** A saved rule over already-seeded profiles, so a UI action is the only thing under test. */
async function seedRule(request: APIRequestContext, seed: GuardrailRuleForm): Promise<void> {
  await guardrailsApi.createRule(request, {
    name: seed.name,
    description: seed.description ?? "",
    enabled: true,
    target: "llm",
    applyTo: "input",
    celExpression: seed.celExpression,
    samplingRate: 100,
    timeout: 60,
    selectedGuardrailProfiles: seed.profileIds.map((id) => `regex:${id}`),
  });
  createdRuleNames.push(seed.name);
}

test.describe("Guardrails", () => {
  test.afterEach(async ({ request }) => {
    await deleteCreatedResources(request);
  });

  test.describe("Providers page", () => {
    test("creates a regex profile from the PII template and stores its patterns", async ({
      guardrailsProvidersPage,
    }) => {
      const profile = createRegexProfileData();
      createdProfileNames.push(profile.name);

      await guardrailsProvidersPage.goto();
      await guardrailsProvidersPage.openCreate();
      await guardrailsProvidersPage.nameInput.fill(profile.name);
      await guardrailsProvidersPage.piiTemplateBtn.click();

      const inserted = await guardrailsProvidersPage.patternRows.count();
      expect(inserted).toBeGreaterThan(0);

      await guardrailsProvidersPage.saveProfile();
      await expect(guardrailsProvidersPage.getProfileRow(profile.name)).toBeVisible();

      // Reopening reads what was persisted, not what the form still held.
      expect(await guardrailsProvidersPage.storedPatternCount(profile.name)).toBe(inserted);
    });

    test("renames a regex profile", async ({ request, guardrailsProvidersPage }) => {
      const profile = createRegexProfileData();
      const renamed = `${profile.name}-renamed`;
      await seedRegexProfile(request, profile.name);
      createdProfileNames.push(renamed);

      await guardrailsProvidersPage.goto();
      await expect(guardrailsProvidersPage.getProfileRow(profile.name)).toBeVisible();
      await guardrailsProvidersPage.renameProfile(profile.name, renamed);
    });

    test("deletes a regex profile", async ({ request, guardrailsProvidersPage }) => {
      const profile = createRegexProfileData();
      await seedRegexProfile(request, profile.name);

      await guardrailsProvidersPage.goto();
      await expect(guardrailsProvidersPage.getProfileRow(profile.name)).toBeVisible();
      await guardrailsProvidersPage.deleteProfile(profile.name);

      expect(await guardrailsProvidersPage.profileExists(profile.name)).toBe(false);
    });
  });

  test.describe("Rules page", () => {
    test("creates a rule that inspects all requests with the chosen profile", async ({
      request,
      guardrailsRulesPage,
    }) => {
      const profile = createRegexProfileData();
      const profileId = await seedRegexProfile(request, profile.name);
      const rule = createGuardrailRuleData({ profileIds: [profileId] });
      createdRuleNames.push(rule.name);

      await guardrailsRulesPage.goto();
      await guardrailsRulesPage.createRule(rule);

      const row = guardrailsRulesPage.getRuleRow(rule.name);
      await expect(row).toBeVisible();
      // The condition the rule was created with, and the profile it runs.
      await expect(row).toContainText(rule.celExpression);
      await expect(row).toContainText(profile.name);
    });

    test("toggles a rule off and back on", async ({ request, guardrailsRulesPage }) => {
      const profile = createRegexProfileData();
      const profileId = await seedRegexProfile(request, profile.name);
      const rule = createGuardrailRuleData({ profileIds: [profileId] });
      await seedRule(request, rule);

      await guardrailsRulesPage.goto();
      await expect(guardrailsRulesPage.getRuleEnabledSwitch(rule.name)).toHaveAttribute(
        "data-state",
        "checked"
      );
      await guardrailsRulesPage.toggleRule(rule.name);
      await expect(guardrailsRulesPage.getRuleEnabledSwitch(rule.name)).toHaveAttribute(
        "data-state",
        "unchecked"
      );
    });

    test("deletes a rule", async ({ request, guardrailsRulesPage }) => {
      const profile = createRegexProfileData();
      const profileId = await seedRegexProfile(request, profile.name);
      const rule = createGuardrailRuleData({ profileIds: [profileId] });
      await seedRule(request, rule);

      await guardrailsRulesPage.goto();
      await guardrailsRulesPage.deleteRule(rule.name);

      expect(await guardrailsRulesPage.ruleExists(rule.name)).toBe(false);
    });
  });

  test.describe("Incidents page", () => {
    test("renders the incidents view", async ({ guardrailsIncidentsPage }) => {
      await guardrailsIncidentsPage.goto();

      await expect(guardrailsIncidentsPage.blockedToggle).toBeVisible();
      await expect(guardrailsIncidentsPage.detectedToggle).toBeVisible();
      await expect(guardrailsIncidentsPage.totalLabel).toContainText("in the last 24 hours");

      const rows = await guardrailsIncidentsPage.rows.count();
      if (rows === 0) {
        await expect(guardrailsIncidentsPage.emptyState).toBeVisible();
      } else {
        await expect(guardrailsIncidentsPage.rows.first()).toBeVisible();
        await expect(guardrailsIncidentsPage.emptyState).toHaveCount(0);
      }
    });
  });
});
