import { GuardrailProvidersView } from "../views/providersView";

export default function GuardrailsProvidersPage() {
	return (
		<div className="no-padding-parent mx-auto flex h-[calc(var(--app-content-viewport)_-_var(--app-bottom-padding))] min-h-0 w-full flex-col overflow-hidden p-4">
			<GuardrailProvidersView />
		</div>
	);
}