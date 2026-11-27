/**
 * Guardrails rules view: which requests are inspected and by which profiles.
 */

import { LoadErrorView } from "@/components/loadErrorView";
import PageTitle from "@/components/pageTitle";
import { PIN_SHADOW_RIGHT } from "@/components/table/columnPinning";
import {
	AlertDialog,
	AlertDialogAction,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
} from "@/components/ui/alertDialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdownMenu";
import { Switch } from "@/components/ui/switch";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { getErrorMessage } from "@/lib/store";
import {
	useDeleteGuardrailRuleMutation,
	useGetGuardrailRulesQuery,
	useGetGuardrailsQuery,
	useToggleGuardrailRuleMutation,
} from "@/lib/store/apis/guardrailsApi";
import { GuardrailRule } from "@/lib/types/guardrails";
import { showLoadError } from "@/lib/utils/loadError";
import { RbacOperation, RbacResource, useRbac } from "@enterprise/lib";
import { ChevronLeft, ChevronRight, MoreHorizontal, PencilIcon, Plus, ShieldCheck, Trash2 } from "lucide-react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { GuardrailRuleSheet } from "./ruleSheet";

const PAGE_SIZE = 25;

export function GuardrailRulesView() {
	const { t } = useTranslation("governance");
	const { t: tc } = useTranslation("common");
	const canCreate = useRbac(RbacResource.GuardrailsConfig, RbacOperation.Create);
	const canUpdate = useRbac(RbacResource.GuardrailsConfig, RbacOperation.Update);
	const canDelete = useRbac(RbacResource.GuardrailsConfig, RbacOperation.Delete);

	const [offset, setOffset] = useState(0);
	const [sheetOpen, setSheetOpen] = useState(false);
	const [editingRule, setEditingRule] = useState<GuardrailRule | null>(null);
	const [deleteTarget, setDeleteTarget] = useState<GuardrailRule | null>(null);

	const { data, isLoading, error, refetch } = useGetGuardrailRulesQuery({ limit: PAGE_SIZE, offset });
	const { data: groups = [] } = useGetGuardrailsQuery();
	const [deleteRule, { isLoading: isDeleting }] = useDeleteGuardrailRuleMutation();
	const [toggleRule] = useToggleGuardrailRuleMutation();

	const rules = data?.rules ?? [];
	const totalCount = data?.count ?? 0;

	// Profile references resolve to their policy name for display; a deleted
	// profile keeps its raw reference rather than disappearing from the row.
	const profileNames = useMemo(
		() =>
			new Map<string, string>(
				groups.flatMap((group) => group.configs.map((config) => [`${group.name}:${config.id}`, config.name] as [string, string])),
			),
		[groups],
	);

	const openCreate = () => {
		setEditingRule(null);
		setSheetOpen(true);
	};

	const openEdit = (rule: GuardrailRule) => {
		setEditingRule(rule);
		setSheetOpen(true);
	};

	const handleDelete = async () => {
		if (!deleteTarget) return;
		try {
			await deleteRule(deleteTarget.id).unwrap();
			toast.success(t("guardrails.rules.deleted"));
			// A delete that empties the last page must not leave the offset past the end.
			if (rules.length === 1 && offset > 0) setOffset(Math.max(0, offset - PAGE_SIZE));
			setDeleteTarget(null);
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	};

	if (showLoadError(error, data)) {
		return <LoadErrorView entity="guardrail rules" error={error} onRetry={() => void refetch()} testId="guardrails-rules-load-error" />;
	}

	return (
		<div className="flex min-h-0 flex-1 flex-col">
			<PageTitle>{t("guardrails.description.rules")}</PageTitle>

			<div className="mb-4 flex flex-wrap items-center gap-3">
				<span className="text-muted-foreground text-sm" data-testid="guardrail-rules-count">
					{t("guardrails.rules.total", { count: totalCount })}
				</span>
				{canCreate && (
					<Button onClick={openCreate} className="ml-auto gap-2" data-testid="guardrail-rule-create-btn">
						<Plus className="h-4 w-4" />
						{t("guardrails.rules.add")}
					</Button>
				)}
			</div>

			{!isLoading && totalCount === 0 ? (
				<div className="flex flex-col items-center justify-center rounded-sm border py-16 text-center" data-testid="guardrail-rules-empty">
					<ShieldCheck className="text-muted-foreground mb-3 h-10 w-10" strokeWidth={1.5} />
					<h3 className="mb-1 text-base font-medium">{t("guardrails.rules.emptyTitle")}</h3>
					<p className="text-muted-foreground mb-4 max-w-md text-sm">{t("guardrails.rules.emptyDescription")}</p>
					{canCreate && (
						<Button onClick={openCreate} data-testid="guardrail-rule-empty-create-btn">
							{t("guardrails.rules.add")}
						</Button>
					)}
				</div>
			) : (
				<div className="mb-2 overflow-hidden rounded-sm border">
					<Table containerClassName="h-full overflow-auto">
						<TableHeader className="bg-muted sticky top-0 z-10">
							<TableRow className="bg-muted/50">
								<TableHead className="font-semibold">{t("guardrails.common.name")}</TableHead>
								<TableHead className="font-semibold">{t("guardrails.rules.profiles")}</TableHead>
								<TableHead className="font-semibold">{t("guardrails.rules.condition")}</TableHead>
								<TableHead className="text-right font-semibold">{t("guardrails.rules.samplingRate")}</TableHead>
								<TableHead className="font-semibold">{t("guardrails.common.status")}</TableHead>
								<TableHead className={`bg-muted sticky right-0 z-30 w-[50px] text-right font-semibold ${PIN_SHADOW_RIGHT}`}>
									{tc("actions")}
								</TableHead>
							</TableRow>
						</TableHeader>
						<TableBody>
							{isLoading ? (
								[...Array(4)].map((_, index) => (
									<TableRow key={index}>
										<TableCell colSpan={6} className="h-10">
											<div className="bg-muted h-2 w-32 animate-pulse rounded" />
										</TableCell>
									</TableRow>
								))
							) : rules.length === 0 ? (
								<TableRow>
									<TableCell colSpan={6} className="h-24 text-center">
										<span className="text-muted-foreground text-sm">{t("guardrails.rules.emptyTitle")}</span>
									</TableCell>
								</TableRow>
							) : (
								rules.map((rule) => (
									<TableRow key={rule.id} className="hover:bg-muted/50 transition-colors" data-testid={`guardrail-rule-row-${rule.id}`}>
										<TableCell className="font-medium">
											<div className="flex flex-col gap-1">
												<span className="max-w-xs truncate">{rule.name}</span>
												{rule.description && <span className="text-muted-foreground max-w-xs truncate text-xs">{rule.description}</span>}
											</div>
										</TableCell>
										<TableCell>
											<div className="flex max-w-xs flex-wrap gap-1">
												{rule.selectedGuardrailProfiles.length === 0 ? (
													<span className="text-muted-foreground text-xs">{t("guardrails.rules.noProfiles")}</span>
												) : (
													rule.selectedGuardrailProfiles.map((profile) => (
														<Badge key={profile} variant="secondary">
															{profileNames.get(profile) ?? profile}
														</Badge>
													))
												)}
											</div>
										</TableCell>
										<TableCell>
											<span className="text-muted-foreground block max-w-xs truncate font-mono text-xs" title={rule.celExpression}>
												{rule.celExpression}
											</span>
										</TableCell>
										<TableCell className="text-right">{rule.samplingRate}%</TableCell>
										<TableCell>
											<Switch
												data-testid={`guardrail-rule-enabled-${rule.id}-switch`}
												checked={rule.enabled}
												disabled={!canUpdate}
												onAsyncCheckedChange={async (checked) => {
													await toggleRule({ id: rule.id, enabled: checked })
														.unwrap()
														.then(() =>
															toast.success(checked ? t("guardrails.common.enabledSuccess") : t("guardrails.common.disabledSuccess")),
														)
														.catch((err) => toast.error(getErrorMessage(err)));
												}}
											/>
										</TableCell>
										<TableCell
											className={`group-hover:bg-muted dark:bg-card dark:group-hover:bg-muted sticky right-0 z-20 bg-white text-right ${PIN_SHADOW_RIGHT}`}
										>
											<DropdownMenu>
												<DropdownMenuTrigger asChild>
													<Button
														variant="ghost"
														size="icon"
														className="h-8 w-8"
														aria-label={t("guardrails.rules.actionsAria", { name: rule.name })}
														data-testid={`guardrail-rule-actions-${rule.id}-btn`}
													>
														<MoreHorizontal className="h-4 w-4" />
													</Button>
												</DropdownMenuTrigger>
												<DropdownMenuContent align="end">
													<DropdownMenuItem
														className="cursor-pointer"
														disabled={!canUpdate}
														data-testid={`guardrail-rule-edit-${rule.id}-btn`}
														onSelect={() => openEdit(rule)}
													>
														<PencilIcon className="h-4 w-4" />
														{tc("edit")}
													</DropdownMenuItem>
													<DropdownMenuItem
														variant="destructive"
														className="cursor-pointer"
														disabled={!canDelete}
														data-testid={`guardrail-rule-delete-${rule.id}-btn`}
														onSelect={() => setDeleteTarget(rule)}
													>
														<Trash2 className="h-4 w-4" />
														{tc("delete")}
													</DropdownMenuItem>
												</DropdownMenuContent>
											</DropdownMenu>
										</TableCell>
									</TableRow>
								))
							)}
						</TableBody>
					</Table>
				</div>
			)}

			{totalCount > PAGE_SIZE && (
				<div className="flex shrink-0 items-center justify-between text-xs" data-testid="guardrail-rules-pagination">
					<span className="text-muted-foreground">
						{offset + 1}–{Math.min(offset + PAGE_SIZE, totalCount)} / {totalCount}
					</span>
					<div className="flex items-center gap-1">
						<Button
							variant="ghost"
							size="sm"
							disabled={offset === 0}
							onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
							aria-label={tc("prev")}
						>
							<ChevronLeft className="h-4 w-4" />
						</Button>
						<Button
							variant="ghost"
							size="sm"
							disabled={offset + PAGE_SIZE >= totalCount}
							onClick={() => setOffset(offset + PAGE_SIZE)}
							aria-label={tc("next")}
						>
							<ChevronRight className="h-4 w-4" />
						</Button>
					</div>
				</div>
			)}

			<GuardrailRuleSheet open={sheetOpen} onOpenChange={setSheetOpen} rule={editingRule} />

			<AlertDialog open={!!deleteTarget} onOpenChange={(open) => !open && setDeleteTarget(null)}>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>{t("guardrails.rules.deleteTitle")}</AlertDialogTitle>
						<AlertDialogDescription>{t("guardrails.rules.deleteDescription", { name: deleteTarget?.name ?? "" })}</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel disabled={isDeleting}>{tc("cancel")}</AlertDialogCancel>
						<AlertDialogAction
							disabled={isDeleting}
							data-testid="guardrail-rule-delete-confirm-btn"
							onClick={(event) => {
								event.preventDefault();
								void handleDelete();
							}}
						>
							{isDeleting ? t("guardrails.rules.deleting") : tc("delete")}
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</div>
	);
}