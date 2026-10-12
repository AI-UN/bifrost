import { describe, expect, it } from "vitest";
import { compareVersions } from "./version";

describe("compareVersions", () => {
	it("treats a downstream -oss build as equal to the upstream release it is built from", () => {
		expect(compareVersions("v2.0.0", "v2.0.0-oss")).toBe(0);
		expect(compareVersions("v2.0.0-oss", "v2.0.0")).toBe(0);
	});

	it("still reports a newer upstream release than an -oss build", () => {
		expect(compareVersions("v2.1.0", "v2.0.0-oss")).toBe(1);
		expect(compareVersions("v2.0.1", "v2.0.0-oss")).toBe(1);
	});

	it("does not report an older upstream release as newer", () => {
		expect(compareVersions("v1.9.9", "v2.0.0-oss")).toBe(-1);
	});

	it("compares plain versions", () => {
		expect(compareVersions("1.5.10", "1.5.9")).toBe(1);
		expect(compareVersions("v1.5.9", "v1.5.9")).toBe(0);
		expect(compareVersions("v1.5.0", "v1.6.0")).toBe(-1);
	});

	it("ranks a release above its own prerelease", () => {
		expect(compareVersions("v2.0.0", "v2.0.0-prerelease1")).toBe(1);
		expect(compareVersions("v2.0.0-prerelease1", "v2.0.0")).toBe(-1);
		expect(compareVersions("v2.0.0-prerelease2", "v2.0.0-prerelease1")).toBe(1);
	});
});