/**
 * Guardrails providers view: the detection profiles attached to rules, grouped
 * by the three providers this build ships.
 */

import { LoadErrorView } from "@/components/loadErrorView";
import PageTitle from "@/components/pageTitle";
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
import { Switch } from "@/components/ui/switch";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { getErrorMessage } from "@/lib/store";
import {
	useDeleteGuardrailProfileMutation,
	useGetGuardrailsQuery,
	useUpdateGuardrailProfileMutation,
	useVerifyGuardrailProfileMutation,
} from "@/lib/store/apis/guardrailsApi";
import {
	GUARDRAIL_ACTION_OPTIONS,
	GUARDRAIL_PROVIDERS,
	GuardrailProfile,
	GuardrailProvider,
	DEFAULT_GUARDRAIL_TIMEOUT,
} from "@/lib/types/guardrails";
import { showLoadError } from "@/lib/utils/loadError";
import { RbacOperation, RbacResource, useRbac } from "@enterprise/lib";
import { PencilIcon, Plug, Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { buildConfig, GuardrailProfileSheet } from "./profileSheet";

export function GuardrailProvidersView() {
	const { t } = useTranslation("governance");
	const { t: tc } = useTranslation("common");
	const canCreate = useRbac(RbacResource.GuardrailsProviders, RbacOperation.Create);
	const canUpdate = useRbac(RbacResource.GuardrailsProviders, RbacOperation.Update);
	const canDelete = useRbac(RbacResource.GuardrailsProviders, RbacOperation.Delete);

	const { data, error, refetch } = useGetGuardrailsQuery();
	const groups = data ?? [];
	const [updateProfile] = useUpdateGuardrailProfileMutation();
	const [deleteProfile, { isLoading: isDeleting }] = useDeleteGuardrailProfileMutation();
	const [verifyProfile, { isLoading: isVerifying }] = useVerifyGuardrailProfileMutation();

	const [sheet, setSheet] = useState<{ provider: GuardrailProvider; profile: GuardrailProfile | null } | null>(null);
	const [deleteTarget, setDeleteTarget] = useState<{ provider: GuardrailProvider; profile: GuardrailProfile } | null>(null);

	const handleToggle = async (provider: GuardrailProvider, profile: GuardrailProfile, enabled: boolean) => {
		try {
			await updateProfile({ provider, data: { id: profile.id, enabled } }).unwrap();
			toast.success(enabled ? t("guardrails.common.enabledSuccess") : t("guardrails.common.disabledSuccess"));
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	};

	const handleVerify = async (provider: GuardrailProvider, profile: GuardrailProfile) => {
		try {
			await verifyProfile({
				provider,
				data: { id: profile.id, name: profile.name, config: buildConfig(profile, profile.timeout ?? DEFAULT_GUARDRAIL_TIMEOUT, provider) },
			}).unwrap();
			toast.success(t("guardrails.providers.verified"));
		} catch (err) {
			toast.error(t("guardrails.providers.verifyFailed"), { description: getErrorMessage(err) });
		}
	};

	const handleDelete = async () => {
		if (!deleteTarget) return;
		try {
			await deleteProfile({ provider: deleteTarget.provider, id: deleteTarget.profile.id }).unwrap();
			toast.success(t("guardrails.providers.deleted"));
			setDeleteTarget(null);
		} catch (err) {
			toast.error(getErrorMessage(err));
		}
	};

	if (showLoadError(error, data)) {
		return (
			<LoadErrorView entity="guardrail providers" error={error} onRetry={() => void refetch()} testId="guardrails-providers-load-error" />
		);
	}

	return (
		<div className="flex min-h-0 flex-1 flex-col gap-6 overflow-y-auto">
			<PageTitle>{t("guardrails.description.providers")}</PageTitle>

			{GUARDRAIL_PROVIDERS.map((provider) => {
				const configs = groups.find((group) => group.name === provider)?.configs ?? [];
				return (
					<section key={provider} className="space-y-3" data-testid={`guardrail-provider-${provider}`}>
						<div className="flex flex-wrap items-start justify-between gap-3">
							<div>
								<h3 className="text-base font-medium">{t(`guardrails.providers.sections.${provider}`)}</h3>
								<p className="text-muted-foreground text-sm">{t(`guardrails.providers.sections.${provider}Desc`)}</p>
							</div>
							{canCreate && (
								<Button
									variant="outline"
									size="sm"
									className="gap-2"
									data-testid={`guardrail-profile-add-${provider}-btn`}
									onClick={() => setSheet({ provider, profile: null })}
								>
									<Plus className="h-4 w-4" />
									{t("guardrails.providers.addTitle")}
								</Button>
							)}
						</div>

						{configs.length === 0 ? (
							<p
								className="text-muted-foreground rounded-sm border px-4 py-6 text-center text-sm"
								data-testid={`guardrail-provider-${provider}-empty`}
							>
								{t("guardrails.providers.empty")}
							</p>
						) : (
							<div className="overflow-hidden rounded-sm border">
								<Table>
									<TableHeader className="bg-muted/50">
										<TableRow>
											<TableHead className="font-semibold">{t("guardrails.common.name")}</TableHead>
											<TableHead className="font-semibold">{t("guardrails.common.action")}</TableHead>
											<TableHead className="text-right font-semibold">{t("guardrails.common.timeout")}</TableHead>
											<TableHead className="font-semibold">{t("guardrails.common.status")}</TableHead>
											<TableHead className="text-right font-semibold">{tc("actions")}</TableHead>
										</TableRow>
									</TableHeader>
									<TableBody>
										{configs.map((profile) => (
											<TableRow key={profile.id} data-testid={`guardrail-profile-row-${provider}-${profile.id}`}>
												<TableCell className="font-medium">
													<span className="max-w-xs truncate">{profile.name}</span>
												</TableCell>
												<TableCell>
													<Badge variant="secondary">
														{GUARDRAIL_ACTION_OPTIONS.find((option) => option.value === (profile.action ?? "detect_only"))?.label}
													</Badge>
												</TableCell>
												<TableCell className="text-right">{profile.timeout ?? 30}s</TableCell>
												<TableCell>
													<Switch
														data-testid={`guardrail-profile-enabled-${provider}-${profile.id}-switch`}
														checked={profile.enabled}
														disabled={!canUpdate}
														onAsyncCheckedChange={(checked) => handleToggle(provider, profile, checked)}
													/>
												</TableCell>
												<TableCell className="text-right">
													<div className="flex items-center justify-end gap-1">
														<Button
															variant="ghost"
															size="icon"
															className="h-8 w-8"
															aria-label={t("guardrails.providers.verify")}
															disabled={isVerifying}
															data-testid={`guardrail-profile-verify-${provider}-${profile.id}-btn`}
															onClick={() => void handleVerify(provider, profile)}
														>
															<Plug className="h-4 w-4" />
														</Button>
														<Button
															variant="ghost"
															size="icon"
															className="h-8 w-8"
															aria-label={tc("edit")}
															disabled={!canUpdate}
															data-testid={`guardrail-profile-edit-${provider}-${profile.id}-btn`}
															onClick={() => setSheet({ provider, profile })}
														>
															<PencilIcon className="h-4 w-4" />
														</Button>
														<Button
															variant="ghost"
															size="icon"
															className="text-destructive h-8 w-8"
															aria-label={tc("delete")}
															disabled={!canDelete}
															data-testid={`guardrail-profile-delete-${provider}-${profile.id}-btn`}
															onClick={() => setDeleteTarget({ provider, profile })}
														>
															<Trash2 className="h-4 w-4" />
														</Button>
													</div>
												</TableCell>
											</TableRow>
										))}
									</TableBody>
								</Table>
							</div>
						)}
					</section>
				);
			})}

			{sheet && (
				<GuardrailProfileSheet
					open={!!sheet}
					onOpenChange={(open) => !open && setSheet(null)}
					provider={sheet.provider}
					profile={sheet.profile}
				/>
			)}

			<AlertDialog open={!!deleteTarget} onOpenChange={(open) => !open && setDeleteTarget(null)}>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>{t("guardrails.providers.deleteTitle")}</AlertDialogTitle>
						<AlertDialogDescription>
							{t("guardrails.providers.deleteDescription", { name: deleteTarget?.profile.name ?? "" })}
						</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel disabled={isDeleting}>{tc("cancel")}</AlertDialogCancel>
						<AlertDialogAction
							disabled={isDeleting}
							data-testid="guardrail-profile-delete-confirm-btn"
							onClick={(event) => {
								event.preventDefault();
								void handleDelete();
							}}
						>
							{tc("delete")}
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</div>
	);
}