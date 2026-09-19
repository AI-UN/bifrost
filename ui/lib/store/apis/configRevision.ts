import type {
	BaseQueryApi,
	BaseQueryFn,
	FetchArgs,
	FetchBaseQueryError,
	FetchBaseQueryMeta,
	QueryReturnValue,
} from "@reduxjs/toolkit/query/react";

const CONFIG_REVISION_HEADER = "X-Bifrost-Config-Revision";
const CONFIG_REVISION_ENDPOINT = "/config/revision";
const CONFIG_MUTATION_METHODS: Record<string, true> = {
	POST: true,
	PUT: true,
	PATCH: true,
	DELETE: true,
};

const CONFIG_MUTATION_PATHS = [
	/^\/config(?:\/|$)/,
	/^\/proxy-config(?:\/|$)/,
	/^\/providers(?:\/|$)/,
	/^\/models\/catalog(?:\/|$)/,
	/^\/governance(?:\/|$)/,
	/^\/plugins(?:\/|$)/,
	/^\/mcp\/(?:client|library)(?:\/|$)/,
	/^\/feature-flags(?:\/|$)/,
	/^\/prompt-repo\/(?:folders|prompts|versions)(?:\/|$)/,
	/^\/skills(?:\/|$)/,
	/^\/(?:api-keys|keys)(?:\/|$)/,
	/^\/prompt-repo\/sessions\/[^/]+\/commit(?:\/|$)/,
];

const DYNAMIC_MUTATION_PATHS = [
	/^\/config\/metadata(?:\/|$)/,
	/^\/skills\/files(?:\/|$)/,
	/^\/governance\/usage-reset(?:\/|$)/,
	/^\/mcp\/library\/force-sync(?:\/|$)/,
	/^\/mcp\/client\/[^/]+\/reconnect(?:\/|$)/,
	/^\/prompt-repo\/sessions(?!\/[^/]+\/commit(?:\/|$))(?:\/|$)/,
	/^\/prompt-repo\/prompts\/[^/]+\/sessions(?:\/|$)/,
];

export type ConfigRevisionBaseQuery = BaseQueryFn<
	string | FetchArgs,
	unknown,
	FetchBaseQueryError,
	Record<never, never>,
	FetchBaseQueryMeta
>;

type ConfigRevisionResult = QueryReturnValue<unknown, FetchBaseQueryError, FetchBaseQueryMeta>;

type RevisionResponse = {
	revision?: unknown;
};

function createRequestHeaders(headers: FetchArgs["headers"]): Headers {
	const requestHeaders = new Headers();
	if (!headers) {
		return requestHeaders;
	}
	if (headers instanceof Headers) {
		headers.forEach((value, name) => requestHeaders.set(name, value));
		return requestHeaders;
	}
	if (Array.isArray(headers)) {
		for (const [name, value] of headers) {
			if (typeof name === "string" && typeof value === "string") {
				requestHeaders.append(name, value);
			}
		}
		return requestHeaders;
	}
	for (const [name, value] of Object.entries(headers)) {
		if (value !== undefined) {
			requestHeaders.set(name, value);
		}
	}
	return requestHeaders;
}

function normalizeRevision(value: unknown): string | null {
	if (typeof value === "number") {
		return Number.isSafeInteger(value) && value >= 0 ? String(value) : null;
	}
	if (typeof value !== "string") {
		return null;
	}

	let revision = value.trim();
	if (revision.startsWith("W/")) {
		revision = revision.slice(2).trim();
	}
	if (revision.startsWith('"') && revision.endsWith('"')) {
		revision = revision.slice(1, -1).trim();
	}
	return /^\d+$/.test(revision) ? revision : null;
}

function getRevisionFromResult(result: ConfigRevisionResult): string | null {
	const response = result.meta?.response;
	const headerRevision = normalizeRevision(response?.headers.get(CONFIG_REVISION_HEADER));
	if (headerRevision !== null) {
		return headerRevision;
	}

	const etagRevision = normalizeRevision(response?.headers.get("ETag"));
	if (etagRevision !== null) {
		return etagRevision;
	}

	if (result.data && typeof result.data === "object" && "revision" in result.data) {
		return normalizeRevision((result.data as RevisionResponse).revision);
	}
	return null;
}

function normalizePath(url: string): string {
	let path = url;
	if (url.startsWith("http://") || url.startsWith("https://")) {
		try {
			path = new URL(url).pathname;
		} catch {
			return "";
		}
	} else {
		path = path.split(/[?#]/, 1)[0];
	}

	if (!path.startsWith("/")) {
		path = `/${path}`;
	}
	if (path === "/api") {
		return "/";
	}
	return path.startsWith("/api/") ? path.slice(4) : path;
}

export function isConfigControlPlaneMutation(args: string | FetchArgs): args is FetchArgs {
	if (typeof args === "string") {
		return false;
	}

	const method = args.method?.toUpperCase() ?? "GET";
	if (!CONFIG_MUTATION_METHODS[method]) {
		return false;
	}

	const path = normalizePath(args.url);
	if (DYNAMIC_MUTATION_PATHS.some((pattern) => pattern.test(path))) {
		return false;
	}
	return CONFIG_MUTATION_PATHS.some((pattern) => pattern.test(path));
}

export function createConfigRevisionBaseQuery(baseQuery: ConfigRevisionBaseQuery): ConfigRevisionBaseQuery {
	let revision: string | null = null;
	let revisionEndpointSupported: boolean | undefined;
	let revisionRequest: Promise<void> | null = null;

	const refreshRevision = async (api: BaseQueryApi, extraOptions: Record<never, never>, force: boolean): Promise<void> => {
		if (!force && (revision !== null || revisionEndpointSupported === false)) {
			return;
		}
		if (revisionRequest) {
			await revisionRequest;
			return;
		}

		revisionRequest = (async () => {
			const result = await baseQuery({ url: CONFIG_REVISION_ENDPOINT, method: "GET" }, api, extraOptions);
			if (result.error) {
				const status = result.error.status === "PARSING_ERROR" ? result.error.originalStatus : result.error.status;
				if (status === 404 || status === 405) {
					revision = null;
					revisionEndpointSupported = false;
				}
				return;
			}

			const nextRevision = getRevisionFromResult(result);
			if (nextRevision === null) {
				revision = null;
				revisionEndpointSupported = false;
				return;
			}
			revision = nextRevision;
			revisionEndpointSupported = true;
		})().finally(() => {
			revisionRequest = null;
		});

		await revisionRequest;
	};

	return async (args, api, extraOptions) => {
		if (!isConfigControlPlaneMutation(args)) {
			return baseQuery(args, api, extraOptions);
		}

		await refreshRevision(api, extraOptions, false);
		const requestArgs: FetchArgs = { ...args };
		if (revision !== null) {
			const headers = createRequestHeaders(args.headers);
			headers.set("If-Match", revision);
			requestArgs.headers = headers;
		}

		const result = await baseQuery(requestArgs, api, extraOptions);
		const responseRevision = getRevisionFromResult(result);
		if (responseRevision !== null) {
			revision = responseRevision;
			revisionEndpointSupported = true;
		}

		if (result.error?.status === 428) {
			await refreshRevision(api, extraOptions, true);
			if (revision !== null) {
				const retryHeaders = createRequestHeaders(args.headers);
				retryHeaders.set("If-Match", revision);
				const retryResult = await baseQuery({ ...args, headers: retryHeaders }, api, extraOptions);
				const retryRevision = getRevisionFromResult(retryResult);
				if (retryRevision !== null) {
					revision = retryRevision;
					revisionEndpointSupported = true;
				}
				return retryResult;
			}
		}

		if (result.error?.status === 409) {
			await refreshRevision(api, extraOptions, true);
		}
		return result;
	};
}