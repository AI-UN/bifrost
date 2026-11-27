import { Locator, Page, expect } from "@playwright/test";
import { BasePage } from "../../../core/pages/base.page";
import { waitForNetworkIdle } from "../../../core/utils/test-helpers";

/**
 * The regex profile form. Only the fields a custom-regex profile needs are modelled: the
 * name, and whether the PII template fills the pattern list.
 */
export interface GuardrailProfileForm {
  name: string;
}

/**
 * Page object for /workspace/guardrails/providers, where each detection provider
 * (secrets, regex, presidio) lists its profiles.
 */
export class GuardrailsProvidersPage extends BasePage {
  readonly regexSection: Locator;
  readonly addRegexProfileBtn: Locator;

  readonly sheet: Locator;
  readonly nameInput: Locator;
  readonly piiTemplateBtn: Locator;
  readonly saveBtn: Locator;
  readonly deleteConfirmBtn: Locator;

  constructor(page: Page) {
    super(page);

    this.regexSection = page.getByTestId("guardrail-provider-regex");
    this.addRegexProfileBtn = page.getByTestId("guardrail-profile-add-regex-btn");

    this.sheet = page.getByTestId("guardrail-profile-sheet");
    this.nameInput = page.getByTestId("guardrail-profile-name-input");
    this.piiTemplateBtn = page.getByTestId("guardrail-profile-pii-template-btn");
    this.saveBtn = page.getByTestId("guardrail-profile-save-btn");
    this.deleteConfirmBtn = page.getByTestId("guardrail-profile-delete-confirm-btn");
  }

  async goto(): Promise<void> {
    await this.page.goto("/workspace/guardrails/providers");
    await waitForNetworkIdle(this.page);
    await expect(this.regexSection).toBeVisible({ timeout: 15000 });
  }

  /** Profile rows carry the server-assigned id, so they are found by their name. */
  getProfileRow(name: string): Locator {
    return this.regexSection.getByRole("row").filter({ hasText: name }).first();
  }

  async profileExists(name: string): Promise<boolean> {
    return (await this.getProfileRow(name).count()) > 0;
  }

  /** Pattern rows carry no per-input testid, so their containers are what the count reads. */
  get patternRows(): Locator {
    return this.sheet.locator('[data-testid^="guardrail-profile-pattern-"]');
  }

  async openCreate(): Promise<void> {
    await this.addRegexProfileBtn.click();
    await expect(this.sheet).toBeVisible({ timeout: 5000 });
    await this.waitForSheetAnimation();
  }

  async openEdit(name: string): Promise<void> {
    await this.getProfileRow(name)
      .locator('[data-testid^="guardrail-profile-edit-regex-"]')
      .click();
    await expect(this.sheet).toBeVisible({ timeout: 5000 });
    await this.waitForSheetAnimation();
  }

  async saveProfile(): Promise<void> {
    await this.saveBtn.click();
    await this.waitForSuccessToast();
    await expect(this.sheet).not.toBeVisible({ timeout: 10000 });
    await this.dismissToasts();
  }

  /** Opens the profile, reads how many patterns were stored, and closes it. */
  async storedPatternCount(name: string): Promise<number> {
    await this.openEdit(name);
    const count = await this.patternRows.count();
    await this.closeSheet();
    return count;
  }

  async renameProfile(name: string, newName: string): Promise<void> {
    await this.openEdit(name);
    await this.nameInput.fill(newName);
    await this.saveProfile();
    await expect(this.getProfileRow(newName)).toBeVisible({ timeout: 10000 });
  }

  async deleteProfile(name: string): Promise<void> {
    await this.getProfileRow(name)
      .locator('[data-testid^="guardrail-profile-delete-regex-"]')
      .click();
    await expect(this.deleteConfirmBtn).toBeVisible({ timeout: 5000 });
    await this.deleteConfirmBtn.click();
    await this.waitForSuccessToast();
    await expect(this.getProfileRow(name)).toHaveCount(0, { timeout: 10000 });
    await this.dismissToasts();
  }

  async closeSheet(): Promise<void> {
    if ((await this.sheet.count()) === 0) return;
    await this.page.keyboard.press("Escape");
    await expect(this.sheet).not.toBeVisible({ timeout: 5000 });
  }
}
