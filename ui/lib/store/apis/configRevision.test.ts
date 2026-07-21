import type { BaseQueryApi, FetchArgs, FetchBaseQueryMeta } from "@reduxjs/toolkit/query/react";
import { describe, expect, test } from "vitest";
import { createConfigRevisionBaseQuery, isConfigControlPlaneMutation, type ConfigRevisionBaseQuery } from "./configRevision";

const queryApi = {} as BaseQueryApi;

function responseMeta(headers: HeadersInit = {}, status = 200): FetchBaseQueryMeta {
	return {
		request: new Request("http://localhost/api/test"),
		response: new Response(null, { headers, status }),
	};
}

function requestArgs(args: string | FetchArgs): FetchArgs {
	return typeof args === "string" ? { url: args } : args;
}

function headerValue(headers: FetchArgs["headers"], name: string): string | null {
	if (!headers) {
		return null;
	}
	if (headers instanceof Headers) {
		return headers.get(name);
	}
	if (Array.isArray(headers)) {
		return headers.find(([headerName]) => headerName.toLowerCase() === name.toLowerCase())?.[1] ?? null;
	}
	const entry = Object.entries(headers).find(([headerName]) => headerName.toLowerCase() === name.toLowerCase());
	return entry?.[1] ?? null;
}

describe("isConfigControlPlaneMutation", () => {
	test.each([
		["/config", "PUT"],
		["/proxy-config", "PUT"],
		["/providers", "POST"],
		["/providers/openai/keys/key-1", "DELETE"],
		["/models/catalog", "PUT"],
		["/governance/virtual-keys", "POST"],
		["/plugins/cache", "DELETE"],
		["/mcp/client/client-1", "PUT"],
		["/mcp/library/42", "DELETE"],
		["/feature-flags/dark-mode", "PUT"],
		["/prompt-repo/folders", "POST"],
		["/prompt-repo/prompts/prompt-1/versions", "POST"],
		["/prompt-repo/sessions/1/commit", "POST"],
		["/mcp/client/client-1/complete-oauth", "POST"],
		["/skills/skill-1", "PUT"],
		["/api-keys/key-1", "DELETE"],
	])("classifies %s as configuration state", (url, method) => {
		expect(isConfigControlPlaneMutation({ url, method })).toBe(true);
	});

	test.each([
		["/config/metadata", "POST"],
		["/governance/usage-reset", "POST"],
		["/prompt-repo/prompts/prompt-1/sessions", "POST"],
		["/mcp/client/client-1/reconnect", "POST"],
		["/mcp/library/force-sync", "POST"],
		["/skills/files/upload", "POST"],
		["/session/login", "POST"],
		["/inference", "POST"],
	])("does not classify dynamic request %s as configuration state", (url, method) => {
		expect(isConfigControlPlaneMutation({ url, method })).toBe(false);
	});
});

describe("createConfigRevisionBaseQuery", () => {
	test("loads and caches the revision before configuration mutations", async () => {
		const requests: FetchArgs[] = [];
		const baseQuery: ConfigRevisionBaseQuery = async (args) => {
			const request = requestArgs(args);
			requests.push(request);
			if (request.url === "/config/revision") {
				return { data: { revision: 7 }, meta: responseMeta({ "X-Bifrost-Config-Revision": "7" }) };
			}
			if (request.url === "/providers") {
				expect(headerValue(request.headers, "If-Match")).toBe("7");
				return { data: {}, meta: responseMeta({ ETag: '"8"' }, 202) };
			}
			expect(headerValue(request.headers, "If-Match")).toBe("8");
			return { data: {}, meta: responseMeta({ "X-Bifrost-Config-Revision": "9" }) };
		};
		const query = createConfigRevisionBaseQuery(baseQuery);

		await query({ url: "/providers", method: "POST", body: {} }, queryApi, {});
		await query({ url: "/plugins/cache", method: "DELETE" }, queryApi, {});

		expect(requests.map(({ url }) => url)).toEqual(["/config/revision", "/providers", "/plugins/cache"]);
	});

	test("keeps mutations compatible when the revision endpoint is unavailable", async () => {
		const requests: FetchArgs[] = [];
		const baseQuery: ConfigRevisionBaseQuery = async (args) => {
			const request = requestArgs(args);
			requests.push(request);
			if (request.url === "/config/revision") {
				return { error: { status: "PARSING_ERROR", originalStatus: 404, data: "Not Found", error: "Unexpected token" } };
			}
			expect(headerValue(request.headers, "If-Match")).toBeNull();
			return { data: {} };
		};
		const query = createConfigRevisionBaseQuery(baseQuery);

		await query({ url: "/config", method: "PUT", body: {} }, queryApi, {});
		await query({ url: "/skills/skill-1", method: "DELETE" }, queryApi, {});

		expect(requests.filter(({ url }) => url === "/config/revision")).toHaveLength(1);
	});

	test("refreshes after a conflict and returns it without replaying the mutation", async () => {
		const requests: FetchArgs[] = [];
		let revisionReads = 0;
		let mutationCalls = 0;
		const baseQuery: ConfigRevisionBaseQuery = async (args) => {
			const request = requestArgs(args);
			requests.push(request);
			if (request.url === "/config/revision") {
				revisionReads += 1;
				const revision = revisionReads === 1 ? "4" : "6";
				return { data: { revision }, meta: responseMeta({ "X-Bifrost-Config-Revision": revision }) };
			}

			mutationCalls += 1;
			if (mutationCalls === 1) {
				expect(headerValue(request.headers, "If-Match")).toBe("4");
				return {
					error: { status: 409, data: { error: "configuration revision conflict", current_revision: 5 } },
					meta: responseMeta({ "X-Bifrost-Config-Revision": "5" }, 409),
				};
			}
			expect(headerValue(request.headers, "If-Match")).toBe("6");
			return { data: {}, meta: responseMeta({ "X-Bifrost-Config-Revision": "7" }) };
		};
		const query = createConfigRevisionBaseQuery(baseQuery);

		const conflict = await query({ url: "/feature-flags/test", method: "PUT", body: {} }, queryApi, {});
		expect(conflict.error?.status).toBe(409);
		expect(mutationCalls).toBe(1);
		expect(revisionReads).toBe(2);

		await query({ url: "/feature-flags/test", method: "PUT", body: {} }, queryApi, {});
		expect(mutationCalls).toBe(2);
		expect(requests.map(({ url }) => url)).toEqual(["/config/revision", "/feature-flags/test", "/config/revision", "/feature-flags/test"]);
	});

	test("refreshes revision and safely retries once after 428", async () => {
		const requests: FetchArgs[] = [];
		const baseQuery: ConfigRevisionBaseQuery = async (args) => {
			const request = requestArgs(args);
			requests.push(request);
			switch (requests.length) {
				case 1:
					return { error: { status: 500, data: {} } };
				case 2:
					return { error: { status: 428, data: {} } };
				case 3:
					return { data: { revision: 8 }, meta: responseMeta({ "X-Bifrost-Config-Revision": "8" }) };
				default:
					return { data: {}, meta: responseMeta({ "X-Bifrost-Config-Revision": "9" }) };
			}
		};
		const query = createConfigRevisionBaseQuery(baseQuery);

		const result = await query({ url: "/providers", method: "POST", body: {} }, queryApi, {});

		expect(result.error).toBeUndefined();
		expect(requests).toHaveLength(4);
		expect(headerValue(requests[1].headers, "If-Match")).toBeNull();
		expect(headerValue(requests[3].headers, "If-Match")).toBe("8");
	});
});