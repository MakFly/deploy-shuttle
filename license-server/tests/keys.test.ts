import { describe, expect, test } from "bun:test";
import { generateLicenseKey } from "../src/lib/keys";

describe("license key", () => {
  test("matches DS-XXXXXX-XXXXXX-XXXXXX format", () => {
    const k = generateLicenseKey();
    expect(k).toMatch(/^DS-[A-Z0-9]{6}-[A-Z0-9]{6}-[A-Z0-9]{6}$/);
  });

  test("does not produce duplicates over a large sample", () => {
    const seen = new Set<string>();
    for (let i = 0; i < 1000; i++) {
      const k = generateLicenseKey();
      expect(seen.has(k)).toBe(false);
      seen.add(k);
    }
  });
});
