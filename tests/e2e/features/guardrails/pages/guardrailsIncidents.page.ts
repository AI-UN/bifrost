import { Locator, Page, expect } from "@playwright/test";
import { BasePage } from "../../../core/pages/base.page";
import { waitForNetworkIdle } from "../../../core/utils/test-helpers";

/**
 * Page object for /workspace/guardrails/incidents, which lists the requests the
 * guardrails blocked or flagged over the last 24 hours from the log store.
 */
export class GuardrailsIncidentsPage extends BasePage {
  readonly totalLabel: Locator;
  readonly blockedToggle: Locator;
  readonly detectedToggle: Locator;
  readonly emptyState: Locator;
  readonly rows: Locator;

  constructor(page: Page) {
    super(page);

    this.totalLabel = page.getByTestId("guardrail-incidents-total");
    this.blockedToggle = page.getByTestId("guardrail-incidents-blocked-toggle");
    this.detectedToggle = page.getByTestId("guardrail-incidents-detected-toggle");
    this.emptyState = page.getByTestId("guardrail-incidents-empty");
    this.rows = page.locator('[data-testid^="guardrail-incidents-row-"]');
  }

  async goto(): Promise<void> {
    await this.page.goto("/workspace/guardrails/incidents");
    await waitForNetworkIdle(this.page);
    await expect(this.totalLabel).toBeVisible({ timeout: 15000 });
    // The log query renders skeletons until it settles; the list or the empty state follows.
    await this.emptyState
      .or(this.rows.first())
      .first()
      .waitFor({ state: "visible", timeout: 20000 });
  }
}
