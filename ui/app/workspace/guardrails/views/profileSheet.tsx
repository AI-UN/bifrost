/**
 * Guardrail profile sheet: one form for the three detection providers, with the
 * provider's own fields rendered by profileFields.
 */

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { Switch } from "@/components/ui/switch";
import { getErrorMessage } from "@/lib/store";
import {
	useCreateGuardrailProfileMutation,
	useUpdateGuardrailProfileMutation,
	useVerifyGuardrailProfileMutation,
} from "@/lib/store/apis/guardrailsApi";
import {
	DEFAULT_GUARDRAIL_TIMEOUT,
	DEFAULT_PRESIDIO_CONFIG,
	DEFAULT_REGEX_CONFIG,
	DEFAULT_SECRETS_CONFIG,
	GUARDRAIL_REDACTION_MODE,
	GuardrailProfile,
	GuardrailProvider,
} from "@/lib/types/guardrails";
import {
	guardrailPresidioConfigSchema,
	guardrailProfileBasicsSchema,
	guardrailRegexConfigSchema,
	GuardrailProfileBasicsValues,
} from "@/lib/types/schemas";
import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { PresidioProfileFields, RegexProfileFields, SecretsProfileFields } from "./profileFields";

function emptyDraft(provider: GuardrailProvider): GuardrailProfile {
	const base = { id: 0, name: "", enabled: true, timeout: DEFAULT_GUARDRAIL_TIMEOUT };
	if (provider === "secrets") return { ...base, ...DEFAULT_SECRETS_CONFIG };
	if (provider === "regex") return { ...base, ...DEFAULT_REGEX_CONFIG };
	return { ...base, ...DEFAULT_PRESIDIO_CONFIG, redaction_mode: GUARDRAIL_REDACTION_MODE };
}

// Only the fields the provider reads are sent, and empty lists/maps are omitted
// so the stored config stays minimal. `redaction_mode` is a top-level setting of the
// secrets and presidio providers only; a regex profile carries it per pattern, and
// sending it at the top level is rejected as an unknown key.
export function buildConfig(draft: GuardrailProfile, timeout: number, provider: GuardrailProvider): Record<string, unknown> {
	const config: Record<string, unknown> = { timeout };
	if (draft.ignored_secret_keywords?.length) config.ignored_secret_keywords = draft.ignored_secret_keywords;
	if (draft.disabled_rules?.length) config.disabled_rules = draft.disabled_rules;
	if (draft.rule_entropy && Object.keys(draft.rule_entropy).length) config.rule_entropy = draft.rule_entropy;
	if (draft.patterns?.length) config.patterns = draft.patterns;
	if (draft.analyzer_url) config.analyzer_url = draft.analyzer_url;
	if (draft.api_key?.value || draft.api_key?.ref) config.api_key = draft.api_key;
	if (draft.language) config.language = draft.language;
	if (draft.score_threshold !== undefined) config.score_threshold = draft.score_threshold;
	if (draft.entities?.length) config.entities = draft.entities;
	if (draft.action) config.action = draft.action;
	if (draft.redaction_strategy) config.redaction_strategy = draft.redaction_strategy;
	if (provider !== "regex") config.redaction_mode = GUARDRAIL_REDACTION_MODE;
	if (draft.fail_open !== undefined) config.fail_open = draft.fail_open;
	return config;
}

export function GuardrailProfileSheet({
	open,
	onOpenChange,
	provider,
	profile,
}: {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	provider: GuardrailProvider;
	profile: GuardrailProfile | null;
}) {
	const { t } = useTranslation("governance");
	const { t: tc } = useTranslation("common");
	const isEditing = !!profile;
	const [draft, setDraft] = useState<GuardrailProfile>(() => emptyDraft(provider));
	const [configError, setConfigError] = useState<string | null>(null);
	const [createProfile, { isLoading: isCreating }] = useCreateGuardrailProfileMutation();
	const [updateProfile, { isLoading: isUpdating }] = useUpdateGuardrailProfileMutation();
	const [verifyProfile, { isLoading: isVerifying }] = useVerifyGuardrailProfileMutation();
	const isSaving = isCreating || isUpdating;

	const {
		register,
		handleSubmit,
		setValue,
		watch,
		reset,
		formState: { errors },
	} = useForm<GuardrailProfileBasicsValues>({
		resolver: zodResolver(guardrailProfileBasicsSchema),
		defaultValues: { name: "", enabled: true, timeout: DEFAULT_GUARDRAIL_TIMEOUT },
	});

	const enabled = watch("enabled");

	useEffect(() => {
		if (!open) return;
		setDraft(profile ? { ...profile } : emptyDraft(provider));
		setConfigError(null);
		reset({
			name: profile?.name ?? "",
			enabled: profile?.enabled ?? true,
			timeout: profile?.timeout || DEFAULT_GUARDRAIL_TIMEOUT,
		});
	}, [open, profile, provider, reset]);

	// The server compiles regex patterns and validates rule IDs, so a config
	// error comes back from verify or save; the local schema only catches what
	// would always fail.
	const validateConfig = (): string | null => {
		if (provider === "regex") {
			const parsed = guardrailRegexConfigSchema.safeParse({ patterns: draft.patterns ?? [] });
			if (!parsed.success) return parsed.error.issues[0]?.message ?? null;
		}
		if (provider === "presidio") {
			const parsed = guardrailPresidioConfigSchema.safeParse({
				analyzer_url: draft.analyzer_url ?? "",
				score_threshold: draft.score_threshold ?? 0.5,
			});
			if (!parsed.success) return parsed.error.issues[0]?.message ?? null;
		}
		return null;
	};

	const onSubmit = async (values: GuardrailProfileBasicsValues) => {
		const error = validateConfig();
		if (error) {
			setConfigError(error);
			return;
		}
		setConfigError(null);
		const config = buildConfig(draft, values.timeout, provider);
		try {
			if (profile) {
				await updateProfile({ provider, data: { id: profile.id, name: values.name, enabled: values.enabled, config } }).unwrap();
				toast.success(t("guardrails.providers.updated"));
			} else {
				await createProfile({ provider, data: { name: values.name, enabled: values.enabled, config } }).unwrap();
				toast.success(t("guardrails.providers.created"));
			}
			onOpenChange(false);
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	};

	const handleVerify = async () => {
		const error = validateConfig();
		if (error) {
			setConfigError(error);
			return;
		}
		setConfigError(null);
		try {
			await verifyProfile({
				provider,
				data: {
					id: profile?.id,
					name: watch("name") || profile?.name || "",
					config: buildConfig(draft, watch("timeout"), provider),
				},
			}).unwrap();
			toast.success(t("guardrails.providers.verified"));
		} catch (err) {
			toast.error(t("guardrails.providers.verifyFailed"), { description: getErrorMessage(err) });
		}
	};

	return (
		<Sheet open={open} onOpenChange={onOpenChange}>
			<SheetContent className="flex w-full flex-col overflow-x-hidden px-0" data-testid="guardrail-profile-sheet">
				<SheetHeader className="flex flex-col items-start pt-8" headerClassName="px-4 md:px-6">
					<SheetTitle>
						{isEditing ? t("guardrails.providers.editTitle") : t("guardrails.providers.addTitle")} ·{" "}
						{t(`guardrails.providers.sections.${provider}`)}
					</SheetTitle>
					<SheetDescription>{t(`guardrails.providers.sections.${provider}Desc`)}</SheetDescription>
				</SheetHeader>

				<form onSubmit={handleSubmit(onSubmit)} className="flex min-h-0 flex-1 flex-col">
					<div className="flex-1 space-y-4 overflow-y-auto px-4 md:px-8">
						<div className="space-y-2">
							<Label htmlFor="guardrail-profile-name">{t("guardrails.common.name")}</Label>
							<Input
								id="guardrail-profile-name"
								data-testid="guardrail-profile-name-input"
								placeholder={t("guardrails.common.namePlaceholder")}
								{...register("name")}
								className={errors.name ? "border-destructive" : ""}
							/>
							{errors.name && <p className="text-destructive text-sm">{errors.name.message}</p>}
						</div>

						<div className="grid grid-cols-2 gap-4">
							<div className="flex items-center justify-between gap-4 rounded-md border px-3 py-2">
								<Label htmlFor="guardrail-profile-enabled" className="text-sm font-normal">
									{t("guardrails.common.enabled")}
								</Label>
								<Switch
									id="guardrail-profile-enabled"
									data-testid="guardrail-profile-enabled-switch"
									checked={enabled}
									onCheckedChange={(checked) => setValue("enabled", checked, { shouldDirty: true })}
								/>
							</div>
							<div className="space-y-2">
								<Label htmlFor="guardrail-profile-timeout">{t("guardrails.common.timeout")}</Label>
								<Input
									id="guardrail-profile-timeout"
									data-testid="guardrail-profile-timeout-input"
									type="number"
									min={1}
									{...register("timeout", { valueAsNumber: true })}
									className={errors.timeout ? "border-destructive" : ""}
								/>
								{errors.timeout && <p className="text-destructive text-sm">{errors.timeout.message}</p>}
							</div>
						</div>

						<Separator />

						{configError && (
							<p className="text-destructive text-sm" data-testid="guardrail-profile-config-error">
								{configError}
							</p>
						)}

						{provider === "secrets" && (
							<SecretsProfileFields draft={draft} onChange={(patch) => setDraft((prev) => ({ ...prev, ...patch }))} />
						)}
						{provider === "regex" && <RegexProfileFields draft={draft} onChange={(patch) => setDraft((prev) => ({ ...prev, ...patch }))} />}
						{provider === "presidio" && (
							<PresidioProfileFields
								draft={draft}
								profileId={profile?.id}
								onChange={(patch) => setDraft((prev) => ({ ...prev, ...patch }))}
							/>
						)}
					</div>

					<div className="dark:bg-card border-border border-t bg-white px-4 py-4 md:px-8">
						<div className="flex justify-end gap-2">
							<Button
								type="button"
								variant="outline"
								disabled={isVerifying}
								data-testid="guardrail-profile-verify-btn"
								onClick={() => void handleVerify()}
							>
								{isVerifying ? t("guardrails.providers.verifying") : t("guardrails.providers.verify")}
							</Button>
							<Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={isSaving}>
								{tc("cancel")}
							</Button>
							<Button type="submit" disabled={isSaving} data-testid="guardrail-profile-save-btn">
								{isSaving ? t("guardrails.common.saving") : isEditing ? tc("save") : t("guardrails.providers.addTitle")}
							</Button>
						</div>
					</div>
				</form>
			</SheetContent>
		</Sheet>
	);
}