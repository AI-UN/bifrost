/**
 * Guardrail incidents: requests the guardrails blocked or flagged. Reads the
 * log store's guardrail_outcomes filter and links each row to its log detail.
 */

import { LoadErrorView } from "@/components/loadErrorView";
import PageTitle from "@/components/pageTitle";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { useGetLogsQuery } from "@/lib/store/apis/logsApi";
import { getLocale } from "@/lib/i18n";
import type { GuardrailOutcome, LogEntry } from "@/lib/types/logs";
import { showLoadError } from "@/lib/utils/loadError";
import { Link } from "@tanstack/react-router";
import { ChevronLeft, ChevronRight, ShieldAlert } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

const PAGE_SIZE = 25;
const OUTCOMES: GuardrailOutcome[] = ["blocked", "detected"];

const isBlocked = (log: LogEntry): boolean => (log.guardrail_debug?.interventions?.length ?? 0) > 0;

const matchedRules = (log: LogEntry): string[] =>
	[...(log.guardrail_debug?.interventions ?? []), ...(log.guardrail_debug?.detections ?? [])]
		.map((match) => match.rule_name)
		.filter((name): name is string => !!name);

export function GuardrailIncidentsView() {
	const { t } = useTranslation("governance");
	const { t: tc } = useTranslation("common");
	const [offset, setOffset] = useState(0);
	const [outcomes, setOutcomes] = useState<GuardrailOutcome[]>(OUTCOMES);

	const { data, isLoading, error, refetch } = useGetLogsQuery(
		{
			filters: { guardrail_outcomes: outcomes, period: "24h" },
			pagination: { limit: PAGE_SIZE, offset, sort_by: "timestamp", order: "desc" },
		},
		// An empty outcome list would drop the filter and list every request.
		{ skip: outcomes.length === 0 },
	);

	const logs = data?.logs ?? [];
	const total = data?.stats?.total_requests ?? 0;

	const toggleOutcome = (outcome: GuardrailOutcome) => {
		setOffset(0);
		setOutcomes((prev) => (prev.includes(outcome) ? prev.filter((entry) => entry !== outcome) : [...prev, outcome]));
	};

	if (showLoadError(error, data)) {
		return (
			<LoadErrorView entity="guardrail incidents" error={error} onRetry={() => void refetch()} testId="guardrails-incidents-load-error" />
		);
	}

	return (
		<div className="flex min-h-0 flex-1 flex-col">
			<PageTitle>{t("guardrails.description.incidents")}</PageTitle>

			<div className="mb-4 flex flex-wrap items-center gap-2">
				{OUTCOMES.map((outcome) => (
					<Button
						key={outcome}
						variant={outcomes.includes(outcome) ? "default" : "outline"}
						size="sm"
						data-testid={`guardrail-incidents-${outcome}-toggle`}
						onClick={() => toggleOutcome(outcome)}
					>
						{t(`guardrails.incidents.${outcome}`)}
					</Button>
				))}
				<span className="text-muted-foreground ml-auto text-sm" data-testid="guardrail-incidents-total">
					{t("guardrails.incidents.total", { count: total })}
				</span>
			</div>

			{outcomes.length === 0 ? (
				<p className="text-muted-foreground rounded-sm border px-4 py-10 text-center text-sm">
					{t("guardrails.incidents.noOutcomeSelected")}
				</p>
			) : !isLoading && logs.length === 0 ? (
				<div
					className="flex flex-col items-center justify-center rounded-sm border py-16 text-center"
					data-testid="guardrail-incidents-empty"
				>
					<ShieldAlert className="text-muted-foreground mb-3 h-10 w-10" strokeWidth={1.5} />
					<h3 className="mb-1 text-base font-medium">{t("guardrails.incidents.emptyTitle")}</h3>
					<p className="text-muted-foreground max-w-md text-sm">{t("guardrails.incidents.emptyDescription")}</p>
				</div>
			) : (
				<div className="mb-2 overflow-hidden rounded-sm border">
					<Table containerClassName="h-full overflow-auto">
						<TableHeader className="bg-muted sticky top-0 z-10">
							<TableRow className="bg-muted/50">
								<TableHead className="font-semibold">{t("guardrails.incidents.time")}</TableHead>
								<TableHead className="font-semibold">{t("guardrails.incidents.request")}</TableHead>
								<TableHead className="font-semibold">{t("guardrails.incidents.model")}</TableHead>
								<TableHead className="font-semibold">{t("guardrails.incidents.outcome")}</TableHead>
								<TableHead className="font-semibold">{t("guardrails.incidents.rules")}</TableHead>
							</TableRow>
						</TableHeader>
						<TableBody>
							{isLoading
								? [...Array(4)].map((_, index) => (
										<TableRow key={index}>
											<TableCell colSpan={5} className="h-10">
												<div className="bg-muted h-2 w-32 animate-pulse rounded" />
											</TableCell>
										</TableRow>
									))
								: logs.map((log) => (
										<TableRow key={log.id} data-testid={`guardrail-incidents-row-${log.id}`}>
											<TableCell className="text-muted-foreground text-xs whitespace-nowrap">
												{new Date(log.timestamp).toLocaleString(getLocale())}
											</TableCell>
											<TableCell className="font-mono text-xs">
												<Link
													to="/workspace/logs"
													search={{ selected_log: log.id }}
													className="hover:text-primary max-w-[12rem] truncate underline-offset-2 hover:underline"
												>
													{log.id}
												</Link>
											</TableCell>
											<TableCell className="text-xs">
												{log.provider} / {log.model}
											</TableCell>
											<TableCell>
												<Badge variant={isBlocked(log) ? "destructive" : "warning"}>
													{t(isBlocked(log) ? "guardrails.incidents.blocked" : "guardrails.incidents.detected")}
												</Badge>
											</TableCell>
											<TableCell>
												<div className="flex max-w-xs flex-wrap gap-1">
													{matchedRules(log).map((rule) => (
														<Badge key={rule} variant="secondary">
															{rule}
														</Badge>
													))}
												</div>
											</TableCell>
										</TableRow>
									))}
						</TableBody>
					</Table>
				</div>
			)}

			{total > PAGE_SIZE && (
				<div className="flex shrink-0 items-center justify-between text-xs" data-testid="guardrail-incidents-pagination">
					<span className="text-muted-foreground">
						{offset + 1}–{Math.min(offset + PAGE_SIZE, total)} / {total}
					</span>
					<div className="flex items-center gap-1">
						<Button
							variant="ghost"
							size="sm"
							disabled={offset === 0}
							aria-label={tc("prev")}
							onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
						>
							<ChevronLeft className="h-4 w-4" />
						</Button>
						<Button
							variant="ghost"
							size="sm"
							disabled={offset + PAGE_SIZE >= total}
							aria-label={tc("next")}
							onClick={() => setOffset(offset + PAGE_SIZE)}
						>
							<ChevronRight className="h-4 w-4" />
						</Button>
					</div>
				</div>
			)}
		</div>
	);
}