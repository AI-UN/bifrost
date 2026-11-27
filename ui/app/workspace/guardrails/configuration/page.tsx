import { GuardrailRulesView } from "../views/rulesView";

export default function GuardrailsConfigurationPage() {
	return (
		<div className="no-padding-parent mx-auto flex h-[calc(var(--app-content-viewport)_-_var(--app-bottom-padding))] min-h-0 w-full flex-col overflow-hidden p-4">
			<GuardrailRulesView />
		</div>
	);
}