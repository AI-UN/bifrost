/**
 * Guardrail rule sheet: create or edit the CEL condition and the ordered list of
 * detection profiles that inspect matching requests.
 */

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { getErrorMessage } from "@/lib/store";
import { useCreateGuardrailRuleMutation, useGetGuardrailsQuery, useUpdateGuardrailRuleMutation } from "@/lib/store/apis/guardrailsApi";
import { GuardrailProvider, GuardrailRule, GuardrailRuleRequest } from "@/lib/types/guardrails";
import { guardrailRuleFormSchema, GuardrailRuleFormValues } from "@/lib/types/schemas";
import { zodResolver } from "@hookform/resolvers/zod";
import { ArrowDown, ArrowUp } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

const DEFAULT_FORM: GuardrailRuleFormValues = {
	name: "",
	description: "",
	enabled: true,
	celExpression: "true",
	samplingRate: 100,
	timeout: 60,
	profiles: [],
};

interface ProfileOption {
	value: string;
	name: string;
	provider: GuardrailProvider;
	disabled: boolean;
}

/** Ordered profile picker: checking a profile appends it to the execution order. */
function ProfileOrderPicker({
	options,
	value,
	onChange,
}: {
	options: ProfileOption[];
	value: string[];
	onChange: (next: string[]) => void;
}) {
	const { t } = useTranslation("governance");
	const byValue = useMemo(() => new Map(options.map((option) => [option.value, option])), [options]);
	const move = (index: number, delta: number) => {
		const next = [...value];
		const target = index + delta;
		if (target < 0 || target >= next.length) return;
		[next[index], next[target]] = [next[target], next[index]];
		onChange(next);
	};

	return (
		<div className="space-y-3">
			{value.length > 0 && (
				<div className="space-y-1" data-testid="guardrail-rule-order">
					<Label className="text-xs">{t("guardrails.rules.order")}</Label>
					{value.map((profile, index) => (
						<div key={profile} className="flex items-center gap-2 rounded-sm border px-2 py-1 text-sm">
							<span className="text-muted-foreground w-4 text-xs">{index + 1}</span>
							<span className="flex-1 truncate" data-testid={`guardrail-rule-order-${index}`}>
								{byValue.get(profile)?.name ?? profile}
							</span>
							<Button
								type="button"
								variant="ghost"
								size="icon"
								className="h-6 w-6"
								disabled={index === 0}
								aria-label={t("guardrails.rules.moveUp")}
								onClick={() => move(index, -1)}
							>
								<ArrowUp className="h-3 w-3" />
							</Button>
							<Button
								type="button"
								variant="ghost"
								size="icon"
								className="h-6 w-6"
								disabled={index === value.length - 1}
								aria-label={t("guardrails.rules.moveDown")}
								onClick={() => move(index, 1)}
							>
								<ArrowDown className="h-3 w-3" />
							</Button>
						</div>
					))}
				</div>
			)}

			{options.length === 0 ? (
				<p className="text-muted-foreground text-xs">{t("guardrails.rules.noProfiles")}</p>
			) : (
				<div className="max-h-56 space-y-2 overflow-y-auto rounded-sm border p-3">
					{options.map((option) => (
						<label key={option.value} htmlFor={`guardrail-profile-${option.value}`} className="flex cursor-pointer items-start gap-2">
							<Checkbox
								id={`guardrail-profile-${option.value}`}
								data-testid={`guardrail-rule-profile-${option.value}-checkbox`}
								checked={value.includes(option.value)}
								onCheckedChange={(checked) =>
									onChange(checked === true ? [...value, option.value] : value.filter((entry) => entry !== option.value))
								}
							/>
							<span className="flex-1 text-sm">
								{option.name}
								<span className="text-muted-foreground ml-2 text-xs">
									{t(`guardrails.providers.sections.${option.provider}`)}
									{!option.disabled && ` · ${t("guardrails.common.disabled")}`}
								</span>
							</span>
						</label>
					))}
				</div>
			)}
		</div>
	);
}

export function GuardrailRuleSheet({
	open,
	onOpenChange,
	rule,
}: {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	rule: GuardrailRule | null;
}) {
	const { t } = useTranslation("governance");
	const { t: tc } = useTranslation("common");
	const isEditing = !!rule;
	const { data: groups = [] } = useGetGuardrailsQuery();
	const [createRule, { isLoading: isCreating }] = useCreateGuardrailRuleMutation();
	const [updateRule, { isLoading: isUpdating }] = useUpdateGuardrailRuleMutation();
	const isSaving = isCreating || isUpdating;

	const options = useMemo<ProfileOption[]>(
		() =>
			groups.flatMap((group) =>
				group.configs.map((config) => ({
					value: `${group.name}:${config.id}`,
					name: config.name,
					provider: group.name,
					disabled: config.enabled,
				})),
			),
		[groups],
	);

	const {
		register,
		handleSubmit,
		setValue,
		watch,
		reset,
		formState: { errors },
	} = useForm<GuardrailRuleFormValues>({
		resolver: zodResolver(guardrailRuleFormSchema),
		defaultValues: DEFAULT_FORM,
	});

	const enabled = watch("enabled");
	const celExpression = watch("celExpression");
	const profiles = watch("profiles");

	useEffect(() => {
		if (!open) return;
		reset(
			rule
				? {
						name: rule.name,
						description: rule.description ?? "",
						enabled: rule.enabled,
						celExpression: rule.celExpression || "true",
						samplingRate: rule.samplingRate,
						timeout: rule.timeout || DEFAULT_FORM.timeout,
						profiles: rule.selectedGuardrailProfiles ?? [],
					}
				: DEFAULT_FORM,
		);
	}, [open, rule, reset]);

	const onSubmit = async (values: GuardrailRuleFormValues) => {
		const body: GuardrailRuleRequest = {
			name: values.name,
			description: values.description,
			enabled: values.enabled,
			target: "llm",
			applyTo: "input",
			celExpression: values.celExpression,
			samplingRate: values.samplingRate,
			timeout: values.timeout,
			selectedGuardrailProfiles: values.profiles,
		};
		try {
			if (rule) {
				await updateRule({ id: rule.id, data: body }).unwrap();
				toast.success(t("guardrails.rules.updated"));
			} else {
				await createRule(body).unwrap();
				toast.success(t("guardrails.rules.created"));
			}
			onOpenChange(false);
		} catch (error) {
			toast.error(getErrorMessage(error));
		}
	};

	return (
		<Sheet open={open} onOpenChange={onOpenChange}>
			<SheetContent className="flex w-full flex-col overflow-x-hidden px-0" data-testid="guardrail-rule-sheet">
				<SheetHeader className="flex flex-col items-start pt-8" headerClassName="px-4 md:px-6">
					<SheetTitle>{isEditing ? t("guardrails.rules.editTitle") : t("guardrails.rules.createTitle")}</SheetTitle>
					<SheetDescription>{isEditing ? t("guardrails.rules.editDescription") : t("guardrails.rules.createDescription")}</SheetDescription>
				</SheetHeader>

				<form onSubmit={handleSubmit(onSubmit)} className="flex min-h-0 flex-1 flex-col">
					<div className="flex-1 space-y-4 overflow-y-auto px-4 md:px-8">
						<div className="space-y-2">
							<Label htmlFor="guardrail-rule-name">{t("guardrails.common.name")}</Label>
							<Input
								id="guardrail-rule-name"
								data-testid="guardrail-rule-name-input"
								placeholder={t("guardrails.rules.namePlaceholder")}
								{...register("name")}
								className={errors.name ? "border-destructive" : ""}
							/>
							{errors.name && <p className="text-destructive text-sm">{errors.name.message}</p>}
						</div>

						<div className="space-y-2">
							<Label htmlFor="guardrail-rule-description">{t("guardrails.common.description")}</Label>
							<Input
								id="guardrail-rule-description"
								data-testid="guardrail-rule-description-input"
								placeholder={t("guardrails.common.descriptionPlaceholder")}
								{...register("description")}
							/>
						</div>

						<div className="flex items-center justify-between gap-4 rounded-md border px-3 py-2">
							<Label htmlFor="guardrail-rule-enabled" className="text-sm font-normal">
								{t("guardrails.common.enabled")}
							</Label>
							<Switch
								id="guardrail-rule-enabled"
								data-testid="guardrail-rule-enabled-switch"
								checked={enabled}
								onCheckedChange={(checked) => setValue("enabled", checked, { shouldDirty: true })}
							/>
						</div>

						<Separator />

						<div className="space-y-2">
							<div className="flex items-center justify-between gap-2">
								<Label htmlFor="guardrail-rule-cel">{t("guardrails.rules.condition")}</Label>
								<Button
									type="button"
									variant="outline"
									size="sm"
									data-testid="guardrail-rule-all-requests-btn"
									onClick={() => setValue("celExpression", "true", { shouldValidate: true, shouldDirty: true })}
								>
									{t("guardrails.rules.allRequests")}
								</Button>
							</div>
							<Textarea
								id="guardrail-rule-cel"
								data-testid="guardrail-rule-cel-input"
								className="min-h-24 font-mono text-xs"
								placeholder={t("guardrails.rules.celPlaceholder")}
								value={celExpression}
								onChange={(event) => setValue("celExpression", event.target.value, { shouldValidate: true, shouldDirty: true })}
							/>
							<p className="text-muted-foreground text-xs">{t("guardrails.rules.conditionHelp")}</p>
							<p className="text-muted-foreground text-xs">{t("guardrails.rules.allRequestsHelp")}</p>
							{errors.celExpression && <p className="text-destructive text-sm">{errors.celExpression.message}</p>}
						</div>

						<div className="space-y-2">
							<Label>{t("guardrails.rules.profiles")}</Label>
							<ProfileOrderPicker
								options={options}
								value={profiles}
								onChange={(next) => setValue("profiles", next, { shouldDirty: true })}
							/>
							<p className="text-muted-foreground text-xs">{t("guardrails.rules.profilesHelp")}</p>
						</div>

						<Separator />

						<div className="grid grid-cols-2 gap-4">
							<div className="space-y-2">
								<Label htmlFor="guardrail-rule-sampling">{t("guardrails.rules.samplingRate")}</Label>
								<Input
									id="guardrail-rule-sampling"
									data-testid="guardrail-rule-sampling-input"
									type="number"
									min={0}
									max={100}
									{...register("samplingRate", { valueAsNumber: true })}
									className={errors.samplingRate ? "border-destructive" : ""}
								/>
								<p className="text-muted-foreground text-xs">{t("guardrails.rules.samplingRateHelp")}</p>
								{errors.samplingRate && <p className="text-destructive text-sm">{errors.samplingRate.message}</p>}
							</div>
							<div className="space-y-2">
								<Label htmlFor="guardrail-rule-timeout">{t("guardrails.common.timeout")}</Label>
								<Input
									id="guardrail-rule-timeout"
									data-testid="guardrail-rule-timeout-input"
									type="number"
									min={1}
									{...register("timeout", { valueAsNumber: true })}
									className={errors.timeout ? "border-destructive" : ""}
								/>
								<p className="text-muted-foreground text-xs">{t("guardrails.common.timeoutHelp")}</p>
								{errors.timeout && <p className="text-destructive text-sm">{errors.timeout.message}</p>}
							</div>
						</div>

						<Separator />

						<div className="space-y-1 text-sm">
							<div className="flex items-center justify-between gap-4">
								<span className="text-muted-foreground">{t("guardrails.rules.target")}</span>
								<span data-testid="guardrail-rule-target">{t("guardrails.rules.targetLLM")}</span>
							</div>
							<div className="flex items-center justify-between gap-4">
								<span className="text-muted-foreground">{t("guardrails.rules.applyTo")}</span>
								<span data-testid="guardrail-rule-apply-to">{t("guardrails.rules.applyToInput")}</span>
							</div>
							<p className="text-muted-foreground pt-1 text-xs">{t("guardrails.rules.fixedNote")}</p>
						</div>
					</div>

					<div className="dark:bg-card border-border border-t bg-white px-4 py-4 md:px-8">
						<div className="flex justify-end gap-2">
							<Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={isSaving}>
								{tc("cancel")}
							</Button>
							<Button type="submit" disabled={isSaving} data-testid="guardrail-rule-save-btn">
								{isSaving ? t("guardrails.common.saving") : isEditing ? t("guardrails.rules.update") : t("guardrails.rules.create")}
							</Button>
						</div>
					</div>
				</form>
			</SheetContent>
		</Sheet>
	);
}