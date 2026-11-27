/**
 * Provider-specific fields of a guardrail profile. Each group edits the flat
 * profile draft the sheet submits, so no field is mirrored into separate state.
 */

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { MultiSelect } from "@/components/ui/multiSelect";
import { SecretVarInput } from "@/components/ui/secretVarInput";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { useGetGuardrailPresidioEntitiesMutation, useGetGuardrailSecretRulesQuery } from "@/lib/store/apis/guardrailsApi";
import {
	GUARDRAIL_ACTION_OPTIONS,
	GUARDRAIL_PII_TEMPLATE,
	GUARDRAIL_REDACTION_MODE,
	GuardrailProfile,
	GuardrailRegexPattern,
	REDACTION_STRATEGY_OPTIONS,
	splitList,
} from "@/lib/types/guardrails";
import { isHttpURL } from "@/lib/types/schemas";
import { AlertTriangle, Plus, Trash2, Wand2 } from "lucide-react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

type Draft = GuardrailProfile;
type Patch = (patch: Partial<Draft>) => void;

function ActionSelect({ value, onChange }: { value?: string; onChange: (value: string) => void }) {
	const { t } = useTranslation("governance");
	return (
		<Select value={value ?? "detect_only"} onValueChange={onChange}>
			<SelectTrigger data-testid="guardrail-profile-action-select">
				<SelectValue placeholder={t("guardrails.common.action")} />
			</SelectTrigger>
			<SelectContent>
				{GUARDRAIL_ACTION_OPTIONS.map((option) => (
					<SelectItem key={option.value} value={option.value}>
						{option.label}
					</SelectItem>
				))}
			</SelectContent>
		</Select>
	);
}

function StrategySelect({ value, onChange }: { value?: string; onChange: (value: string) => void }) {
	const { t } = useTranslation("governance");
	return (
		<Select value={value ?? "replace"} onValueChange={onChange}>
			<SelectTrigger data-testid="guardrail-profile-strategy-select">
				<SelectValue placeholder={t("guardrails.common.redactionStrategy")} />
			</SelectTrigger>
			<SelectContent>
				{REDACTION_STRATEGY_OPTIONS.map((option) => (
					<SelectItem key={option.value} value={option.value}>
						{option.label}
					</SelectItem>
				))}
			</SelectContent>
		</Select>
	);
}

/** Redaction mode is runtime in this build; shown, never editable. */
export function RedactionModeNote() {
	const { t } = useTranslation("governance");
	return (
		<div className="space-y-1">
			<Label>{t("guardrails.common.redactionMode")}</Label>
			<p className="text-muted-foreground text-sm" data-testid="guardrail-profile-redaction-mode">
				{t("guardrails.common.runtimeOnly")}
			</p>
		</div>
	);
}

export function SecretsProfileFields({ draft, onChange }: { draft: Draft; onChange: Patch }) {
	const { t } = useTranslation("governance");
	const { data: rulesData } = useGetGuardrailSecretRulesQuery();
	const ruleOptions = useMemo(
		() => (rulesData?.rules ?? []).map((rule) => ({ value: rule.id, label: rule.id, description: rule.description })),
		[rulesData],
	);
	const entropyEntries = Object.entries(draft.rule_entropy ?? {});
	const [pendingRule, setPendingRule] = useState("");

	const setEntropy = (ruleId: string, value: number | null) => {
		const next = { ...(draft.rule_entropy ?? {}) };
		if (value === null) delete next[ruleId];
		else next[ruleId] = value;
		onChange({ rule_entropy: next });
	};

	return (
		<div className="space-y-4">
			<div className="grid grid-cols-2 gap-4">
				<div className="space-y-2">
					<Label>{t("guardrails.common.action")}</Label>
					<ActionSelect value={draft.action} onChange={(value) => onChange({ action: value as Draft["action"] })} />
				</div>
				<div className="space-y-2">
					<Label>{t("guardrails.common.redactionStrategy")}</Label>
					<StrategySelect
						value={draft.redaction_strategy}
						onChange={(value) => onChange({ redaction_strategy: value as Draft["redaction_strategy"] })}
					/>
				</div>
			</div>

			<RedactionModeNote />

			<div className="space-y-2">
				<Label>{t("guardrails.providers.secrets.disabledRules")}</Label>
				<MultiSelect
					options={ruleOptions}
					defaultValue={draft.disabled_rules ?? []}
					onValueChange={(value) => onChange({ disabled_rules: value })}
					placeholder={t("guardrails.providers.secrets.disabledRulesPlaceholder")}
					searchable
					data-testid="guardrail-profile-disabled-rules"
				/>
				<p className="text-muted-foreground text-xs">{t("guardrails.providers.secrets.disabledRulesHelp")}</p>
			</div>

			<div className="space-y-2">
				<Label htmlFor="guardrail-profile-ignored-keywords">{t("guardrails.providers.secrets.ignoredKeywords")}</Label>
				<Input
					id="guardrail-profile-ignored-keywords"
					data-testid="guardrail-profile-ignored-keywords-input"
					placeholder={t("guardrails.providers.secrets.ignoredKeywordsPlaceholder")}
					defaultValue={(draft.ignored_secret_keywords ?? []).join(", ")}
					onChange={(event) => onChange({ ignored_secret_keywords: splitList(event.target.value) })}
				/>
				<p className="text-muted-foreground text-xs">{t("guardrails.providers.secrets.ignoredKeywordsHelp")}</p>
			</div>

			<div className="space-y-2">
				<Label>{t("guardrails.providers.secrets.entropyOverrides")}</Label>
				<p className="text-muted-foreground text-xs">{t("guardrails.providers.secrets.entropyOverridesHelp")}</p>
				{entropyEntries.map(([ruleId, entropy]) => (
					<div key={ruleId} className="flex items-center gap-2">
						<span className="flex-1 truncate font-mono text-xs" data-testid={`guardrail-profile-entropy-${ruleId}`}>
							{ruleId}
						</span>
						<Input
							type="number"
							min={0}
							max={6}
							step="0.1"
							className="w-20"
							value={entropy}
							onChange={(event) => setEntropy(ruleId, Number(event.target.value))}
						/>
						<Button
							type="button"
							variant="ghost"
							size="icon"
							aria-label={t("guardrails.providers.secrets.removeOverride")}
							onClick={() => setEntropy(ruleId, null)}
						>
							<Trash2 className="h-4 w-4" />
						</Button>
					</div>
				))}
				<div className="flex items-center gap-2">
					<Select value={pendingRule} onValueChange={setPendingRule}>
						<SelectTrigger className="flex-1" data-testid="guardrail-profile-entropy-rule-select">
							<SelectValue placeholder={t("guardrails.providers.secrets.pickRule")} />
						</SelectTrigger>
						<SelectContent>
							{ruleOptions
								.filter((option) => !(option.value in (draft.rule_entropy ?? {})))
								.map((option) => (
									<SelectItem key={option.value} value={option.value}>
										{option.value}
									</SelectItem>
								))}
						</SelectContent>
					</Select>
					<Button
						type="button"
						variant="outline"
						disabled={!pendingRule}
						data-testid="guardrail-profile-entropy-add-btn"
						onClick={() => {
							setEntropy(pendingRule, 3);
							setPendingRule("");
						}}
					>
						{t("guardrails.providers.secrets.addOverride")}
					</Button>
				</div>
			</div>
		</div>
	);
}

export function RegexProfileFields({ draft, onChange }: { draft: Draft; onChange: Patch }) {
	const { t } = useTranslation("governance");
	const patterns = draft.patterns ?? [];
	// Rows hold uncontrolled inputs (pattern and allowed values are free text), so
	// adding or removing a row must remount the rest for their defaults to follow
	// the array. Bumping the generation on those edits only, not on typing, keeps
	// focus while the row is being typed in.
	const [rowGeneration, setRowGeneration] = useState(0);

	const updatePattern = (index: number, patch: Partial<GuardrailRegexPattern>) =>
		onChange({ patterns: patterns.map((pattern, i) => (i === index ? { ...pattern, ...patch } : pattern)) });

	return (
		<div className="space-y-3">
			<div className="flex items-center justify-between gap-2">
				<Label>{t("guardrails.providers.regex.patterns")}</Label>
				<div className="flex gap-2">
					<Button
						type="button"
						variant="outline"
						size="sm"
						data-testid="guardrail-profile-pii-template-btn"
						onClick={() => {
							onChange({ patterns: [...patterns, ...GUARDRAIL_PII_TEMPLATE.map((pattern) => ({ ...pattern }))] });
							setRowGeneration((generation) => generation + 1);
						}}
					>
						<Wand2 className="h-4 w-4" />
						{t("guardrails.providers.regex.insertTemplate")}
					</Button>
					<Button
						type="button"
						variant="outline"
						size="sm"
						data-testid="guardrail-profile-add-pattern-btn"
						onClick={() => {
							onChange({
								patterns: [
									...patterns,
									{
										pattern: "",
										description: "",
										entity_type: "",
										action: "redact",
										redaction_strategy: "replace",
										redaction_mode: GUARDRAIL_REDACTION_MODE,
									},
								],
							});
							setRowGeneration((generation) => generation + 1);
						}}
					>
						<Plus className="h-4 w-4" />
						{t("guardrails.providers.regex.addPattern")}
					</Button>
				</div>
			</div>

			{patterns.length === 0 && <p className="text-muted-foreground text-sm">{t("guardrails.providers.regex.noPatterns")}</p>}

			{patterns.map((pattern, index) => (
				<div
					key={`${rowGeneration}-${index}`}
					className="space-y-3 rounded-sm border p-3"
					data-testid={`guardrail-profile-pattern-${index}`}
				>
					<div className="space-y-1">
						<Label htmlFor={`guardrail-pattern-${index}`} className="text-xs">
							{t("guardrails.providers.regex.pattern")}
						</Label>
						<Input
							id={`guardrail-pattern-${index}`}
							className="font-mono text-xs"
							placeholder={t("guardrails.providers.regex.patternPlaceholder")}
							value={pattern.pattern}
							onChange={(event) => updatePattern(index, { pattern: event.target.value })}
						/>
					</div>
					<div className="grid grid-cols-2 gap-3">
						<div className="space-y-1">
							<Label className="text-xs">{t("guardrails.common.description")}</Label>
							<Input
								value={pattern.description}
								placeholder={t("guardrails.providers.regex.descriptionPlaceholder")}
								onChange={(event) => updatePattern(index, { description: event.target.value })}
							/>
						</div>
						<div className="space-y-1">
							<Label className="text-xs">{t("guardrails.providers.regex.entityType")}</Label>
							<Input
								value={pattern.entity_type}
								placeholder={t("guardrails.providers.regex.entityTypePlaceholder")}
								onChange={(event) => updatePattern(index, { entity_type: event.target.value })}
							/>
						</div>
						<div className="space-y-1">
							<Label className="text-xs">{t("guardrails.providers.regex.flags")}</Label>
							<Input
								value={pattern.flags ?? ""}
								placeholder="i"
								onChange={(event) => updatePattern(index, { flags: event.target.value })}
							/>
						</div>
						<div className="space-y-1">
							<Label className="text-xs">{t("guardrails.common.action")}</Label>
							<ActionSelect value={pattern.action} onChange={(value) => updatePattern(index, { action: value as Draft["action"] })} />
						</div>
						<div className="space-y-1">
							<Label className="text-xs">{t("guardrails.common.redactionStrategy")}</Label>
							<StrategySelect
								value={pattern.redaction_strategy}
								onChange={(value) => updatePattern(index, { redaction_strategy: value as Draft["redaction_strategy"] })}
							/>
						</div>
						<div className="space-y-1">
							<Label className="text-xs">{t("guardrails.providers.regex.allowedValues")}</Label>
							<Input
								defaultValue={(pattern.allowed_values ?? []).join(", ")}
								placeholder={t("guardrails.providers.regex.allowedValuesPlaceholder")}
								onChange={(event) => updatePattern(index, { allowed_values: splitList(event.target.value) })}
							/>
						</div>
					</div>
					<div className="flex justify-end">
						<Button
							type="button"
							variant="ghost"
							size="sm"
							aria-label={t("guardrails.providers.regex.removePattern")}
							onClick={() => {
								onChange({ patterns: patterns.filter((_, i) => i !== index) });
								setRowGeneration((generation) => generation + 1);
							}}
						>
							<Trash2 className="h-4 w-4" />
						</Button>
					</div>
				</div>
			))}
			<p className="text-muted-foreground text-xs">{t("guardrails.providers.regex.flagsHelp")}</p>
		</div>
	);
}

export function PresidioProfileFields({ draft, onChange, profileId }: { draft: Draft; onChange: Patch; profileId?: number }) {
	const { t } = useTranslation("governance");
	const [fetchEntities, { isLoading: isLoadingEntities }] = useGetGuardrailPresidioEntitiesMutation();
	const [loadedEntities, setLoadedEntities] = useState<string[]>([]);
	const [newEntity, setNewEntity] = useState("");
	const entities = draft.entities ?? [];
	const options = useMemo(
		() => [...new Set([...loadedEntities, ...entities])].map((entity) => ({ value: entity, label: entity })),
		[loadedEntities, entities],
	);

	const loadEntities = async () => {
		if (!isHttpURL(draft.analyzer_url ?? "")) {
			toast.error(t("guardrails.providers.presidio.urlInvalid"));
			return;
		}
		try {
			const result = await fetchEntities({
				id: profileId,
				config: { analyzer_url: draft.analyzer_url ?? "", api_key: draft.api_key, language: draft.language || "en" },
			}).unwrap();
			setLoadedEntities(result.entities);
			toast.success(t("guardrails.providers.presidio.entitiesLoaded", { count: result.entities.length }));
		} catch {
			toast.error(t("guardrails.providers.presidio.entitiesFailed"));
		}
	};

	return (
		<div className="space-y-4">
			<div className="space-y-2">
				<Label htmlFor="guardrail-presidio-url">{t("guardrails.providers.presidio.analyzerUrl")}</Label>
				<Input
					id="guardrail-presidio-url"
					data-testid="guardrail-profile-analyzer-url-input"
					placeholder={t("guardrails.providers.presidio.analyzerUrlPlaceholder")}
					value={draft.analyzer_url ?? ""}
					onChange={(event) => onChange({ analyzer_url: event.target.value })}
				/>
				<p className="text-muted-foreground text-xs">{t("guardrails.providers.presidio.analyzerUrlHelp")}</p>
			</div>

			<div className="space-y-2">
				<Label htmlFor="guardrail-presidio-key">{t("guardrails.providers.presidio.apiKey")}</Label>
				<SecretVarInput
					id="guardrail-presidio-key"
					data-testid="guardrail-profile-api-key-input"
					value={draft.api_key}
					onChange={(value) => onChange({ api_key: value })}
					redactNonEnvValue
				/>
				<p className="text-muted-foreground text-xs">{t("guardrails.providers.presidio.apiKeyHelp")}</p>
			</div>

			<div className="grid grid-cols-2 gap-4">
				<div className="space-y-2">
					<Label htmlFor="guardrail-presidio-language">{t("guardrails.providers.presidio.language")}</Label>
					<Input
						id="guardrail-presidio-language"
						data-testid="guardrail-profile-language-input"
						value={draft.language ?? "en"}
						onChange={(event) => onChange({ language: event.target.value })}
					/>
				</div>
				<div className="space-y-2">
					<Label htmlFor="guardrail-presidio-score">{t("guardrails.providers.presidio.scoreThreshold")}</Label>
					<Input
						id="guardrail-presidio-score"
						data-testid="guardrail-profile-score-input"
						type="number"
						min={0}
						max={1}
						step="0.05"
						value={draft.score_threshold ?? 0.5}
						onChange={(event) => onChange({ score_threshold: Number(event.target.value) })}
					/>
					<p className="text-muted-foreground text-xs">{t("guardrails.providers.presidio.scoreThresholdHelp")}</p>
				</div>
			</div>

			<div className="space-y-2">
				<div className="flex items-center justify-between gap-2">
					<Label>{t("guardrails.providers.presidio.entities")}</Label>
					<Button
						type="button"
						variant="outline"
						size="sm"
						disabled={isLoadingEntities}
						data-testid="guardrail-profile-load-entities-btn"
						onClick={() => void loadEntities()}
					>
						{t("guardrails.providers.presidio.loadEntities")}
					</Button>
				</div>
				<MultiSelect
					options={options}
					defaultValue={entities}
					onValueChange={(value) => onChange({ entities: value })}
					placeholder={t("guardrails.providers.presidio.entitiesPlaceholder")}
					searchable
					data-testid="guardrail-profile-entities"
				/>
				<div className="flex items-center gap-2">
					<Input
						data-testid="guardrail-profile-new-entity-input"
						placeholder={t("guardrails.providers.presidio.entityPlaceholder")}
						value={newEntity}
						onChange={(event) => setNewEntity(event.target.value)}
					/>
					<Button
						type="button"
						variant="outline"
						disabled={!newEntity.trim()}
						data-testid="guardrail-profile-add-entity-btn"
						onClick={() => {
							onChange({ entities: [...entities, newEntity.trim()] });
							setNewEntity("");
						}}
					>
						{t("guardrails.providers.presidio.addEntity")}
					</Button>
				</div>
				<p className="text-muted-foreground text-xs">{t("guardrails.providers.presidio.entitiesHelp")}</p>
			</div>

			<div className="grid grid-cols-2 gap-4">
				<div className="space-y-2">
					<Label>{t("guardrails.common.action")}</Label>
					<ActionSelect value={draft.action} onChange={(value) => onChange({ action: value as Draft["action"] })} />
				</div>
				<div className="space-y-2">
					<Label>{t("guardrails.common.redactionStrategy")}</Label>
					<StrategySelect
						value={draft.redaction_strategy}
						onChange={(value) => onChange({ redaction_strategy: value as Draft["redaction_strategy"] })}
					/>
				</div>
			</div>

			<RedactionModeNote />

			<div className="space-y-2 rounded-md border border-amber-500/40 bg-amber-500/5 p-3">
				<div className="flex items-center justify-between gap-4">
					<Label htmlFor="guardrail-presidio-fail-open" className="text-sm font-normal">
						{t("guardrails.providers.presidio.failOpen")}
					</Label>
					<Switch
						id="guardrail-presidio-fail-open"
						data-testid="guardrail-profile-fail-open-switch"
						checked={draft.fail_open ?? false}
						onCheckedChange={(checked) => onChange({ fail_open: checked })}
					/>
				</div>
				<p className="text-xs text-amber-600 dark:text-amber-400">
					<AlertTriangle className="mr-1 inline h-3 w-3" />
					{t("guardrails.providers.presidio.failOpenHelp")}
				</p>
			</div>
		</div>
	);
}