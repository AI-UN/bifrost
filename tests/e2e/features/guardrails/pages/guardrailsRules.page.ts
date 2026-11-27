import { Locator, Page, expect } from "@playwright/test";
import { BasePage } from "../../../core/pages/base.page";
import { waitForNetworkIdle } from "../../../core/utils/test-helpers";

/**
 * The rule form. `celExpression` is the condition: "true" (the "All requests" quick
 * action) inspects every request. `profileIds` are the custom-regex profile ids the rule
 * runs, in order.
 */
export interface GuardrailRuleForm {
  name: string;
  description?: string;
  celExpression: string;
  profileIds: number[];
}

/**
 * Page object for /workspace/guardrails, the rules that decide which requests the
 * detection profiles inspect.
 */
export class GuardrailsRulesPage extends BasePage {
  readonly countLabel: Locator;
  readonly createBtn: Locator;
  readonly emptyCreateBtn: Locator;

  readonly sheet: Locator;
  readonly nameInput: Locator;
  readonly descriptionInput: Locator;
  readonly enabledSwitch: Locator;
  readonly allRequestsBtn: Locator;
  readonly celInput: Locator;
  readonly saveBtn: Locator;
  readonly deleteConfirmBtn: Locator;

  constructor(page: Page) {
    super(page);

    this.countLabel = page.getByTestId("guardrail-rules-count");
    this.createBtn = page.getByTestId("guardrail-rule-create-btn");
    this.emptyCreateBtn = page.getByTestId("guardrail-rule-empty-create-btn");

    this.sheet = page.getByTestId("guardrail-rule-sheet");
    this.nameInput = page.getByTestId("guardrail-rule-name-input");
    this.descriptionInput = page.getByTestId("guardrail-rule-description-input");
    this.enabledSwitch = page.getByTestId("guardrail-rule-enabled-switch");
    this.allRequestsBtn = page.getByTestId("guardrail-rule-all-requests-btn");
    this.celInput = page.getByTestId("guardrail-rule-cel-input");
    this.saveBtn = page.getByTestId("guardrail-rule-save-btn");
    this.deleteConfirmBtn = page.getByTestId("guardrail-rule-delete-confirm-btn");
  }

  async goto(): Promise<void> {
    await this.page.goto("/workspace/guardrails");
    await waitForNetworkIdle(this.page);
    await expect(this.countLabel).toBeVisible({ timeout: 15000 });
  }

  /** Rule rows carry the server-assigned id, so they are found by their name. */
  getRuleRow(name: string): Locator {
    return this.page
      .locator('[data-testid^="guardrail-rule-row-"]')
      .filter({ hasText: name })
      .first();
  }

  async ruleExists(name: string): Promise<boolean> {
    return (await this.getRuleRow(name).count()) > 0;
  }

  /** The create button lives in the empty state until a rule exists. */
  async openCreate(): Promise<void> {
    if ((await this.emptyCreateBtn.count()) > 0) {
      await this.emptyCreateBtn.click();
    } else {
      await this.createBtn.click();
    }
    await expect(this.sheet).toBeVisible({ timeout: 5000 });
    await this.waitForSheetAnimation();
  }

  async createRule(data: GuardrailRuleForm): Promise<void> {
    await this.openCreate();
    await this.nameInput.fill(data.name);
    if (data.description) await this.descriptionInput.fill(data.description);

    // "All requests" writes the CEL the rule is created with; any other condition is typed.
    if (data.celExpression === "true") {
      await this.allRequestsBtn.click();
    } else {
      await this.celInput.fill(data.celExpression);
    }
    await expect(this.celInput).toHaveValue(data.celExpression);

    for (const id of data.profileIds) {
      const checkbox = this.page.getByTestId(`guardrail-rule-profile-regex-${id}-checkbox`);
      await checkbox.click();
      await expect(checkbox).toHaveAttribute("data-state", "checked");
    }

    await this.saveBtn.click();
    await this.waitForSuccessToast();
    await expect(this.sheet).not.toBeVisible({ timeout: 10000 });
    await this.dismissToasts();
    await expect(this.getRuleRow(data.name)).toBeVisible({ timeout: 10000 });
  }

  getRuleEnabledSwitch(name: string): Locator {
    return this.getRuleRow(name).locator('[data-testid^="guardrail-rule-enabled-"]');
  }

  async toggleRule(name: string): Promise<void> {
    const toggle = this.getRuleEnabledSwitch(name);
    const before = await toggle.getAttribute("data-state");
    await toggle.click();
    await this.waitForSuccessToast();
    await expect(toggle).toHaveAttribute("data-state", before === "checked" ? "unchecked" : "checked");
    await this.dismissToasts();
  }

  async deleteRule(name: string): Promise<void> {
    await this.getRuleRow(name)
      .locator('[data-testid^="guardrail-rule-actions-"]')
      .click();
    // The menu is portalled, so the destructive item is outside the row; the confirm
    // button of the dialog shares the prefix, hence the menuitem role.
    await this.page.locator('[role="menuitem"][data-testid^="guardrail-rule-delete-"]').first().click();
    await expect(this.deleteConfirmBtn).toBeVisible({ timeout: 5000 });
    await this.deleteConfirmBtn.click();
    await this.waitForSuccessToast();
    await expect(this.getRuleRow(name)).toHaveCount(0, { timeout: 10000 });
    await this.dismissToasts();
  }

  async closeSheet(): Promise<void> {
    if ((await this.sheet.count()) === 0) return;
    await this.page.keyboard.press("Escape");
    await expect(this.sheet).not.toBeVisible({ timeout: 5000 });
  }
}
