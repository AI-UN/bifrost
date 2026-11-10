// Build suffixes that identify a downstream build of an upstream release
// rather than a semver prerelease of it. `vX.Y.Z-oss` is built from upstream
// `vX.Y.Z`, so the two must compare as equal — otherwise every dashboard load
// reports the upstream release the build already contains as "now available".
const BUILD_SUFFIXES = ["-oss"] as const;

/** Strip the leading `v` and any known downstream build suffix. */
function normalize(version: string): string {
	let normalized = version.startsWith("v") ? version.slice(1) : version;
	for (const suffix of BUILD_SUFFIXES) {
		if (normalized.endsWith(suffix)) {
			normalized = normalized.slice(0, -suffix.length);
			break;
		}
	}
	return normalized;
}

/**
 * Compare two semantic versions.
 * Returns 1 when `v1` is newer, -1 when `v2` is newer, and 0 when equivalent.
 */
export function compareVersions(v1: string, v2: string): number {
	// Split into main version and prerelease
	const [mainV1, prereleaseV1] = normalize(v1).split("-");
	const [mainV2, prereleaseV2] = normalize(v2).split("-");

	// Compare main version numbers (major.minor.patch)
	const partsV1 = mainV1.split(".").map(Number);
	const partsV2 = mainV2.split(".").map(Number);

	for (let i = 0; i < Math.max(partsV1.length, partsV2.length); i++) {
		const num1 = partsV1[i] || 0;
		const num2 = partsV2[i] || 0;

		if (num1 > num2) return 1;
		if (num1 < num2) return -1;
	}

	// If main versions are equal, check prerelease
	// Version without prerelease is higher than version with prerelease
	if (!prereleaseV1 && prereleaseV2) return 1;
	if (prereleaseV1 && !prereleaseV2) return -1;

	// Both have prereleases, compare them
	if (prereleaseV1 && prereleaseV2) {
		// Extract prerelease number (e.g., "prerelease1" -> 1)
		const prereleaseNum1 = parseInt(prereleaseV1.replace(/\D/g, "")) || 0;
		const prereleaseNum2 = parseInt(prereleaseV2.replace(/\D/g, "")) || 0;

		if (prereleaseNum1 > prereleaseNum2) return 1;
		if (prereleaseNum1 < prereleaseNum2) return -1;
	}
	return 0;
}