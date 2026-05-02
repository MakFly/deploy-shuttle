import { describe, expect, test } from "bun:test";
import { signToken, verifyToken } from "../src/lib/jwt";

async function generateKeyPairB64(): Promise<{ pub: string; priv: string }> {
  const pair = (await crypto.subtle.generateKey(
    { name: "Ed25519" },
    true,
    ["sign", "verify"],
  )) as unknown as CryptoKeyPair;
  const { publicKey, privateKey } = pair;
  const pubRaw = new Uint8Array(await crypto.subtle.exportKey("raw", publicKey));
  const privPkcs8 = new Uint8Array(await crypto.subtle.exportKey("pkcs8", privateKey));
  const privRaw = privPkcs8.slice(privPkcs8.length - 32);
  const toB64 = (b: Uint8Array) => {
    let s = "";
    for (const byte of b) s += String.fromCharCode(byte);
    return btoa(s);
  };
  return { pub: toB64(pubRaw), priv: toB64(privRaw) };
}

describe("Ed25519 JWT", () => {
  test("sign + verify round trip", async () => {
    const { pub, priv } = await generateKeyPairB64();
    const claims = {
      key: "DS-AAAAAA-BBBBBB-CCCCCC",
      tier: "pro" as const,
      fp: "abc",
      iat: 1_000_000_000,
      exp: 1_000_000_900,
    };
    const tok = await signToken(claims, priv);
    expect(tok.split(".")).toHaveLength(3);
    const out = await verifyToken(tok, pub);
    expect(out).toEqual(claims);
  });

  test("verify rejects forged signature", async () => {
    const { pub, priv } = await generateKeyPairB64();
    const claims = { key: "K", tier: "pro" as const, fp: "f", iat: 0, exp: 1 };
    const tok = await signToken(claims, priv);
    // Flip a byte in the signature segment.
    const parts = tok.split(".");
    const sig = parts[2]!;
    const tampered = parts[0] + "." + parts[1] + "." + flipFirstChar(sig);
    expect(await verifyToken(tampered, pub)).toBeNull();
  });

  test("verify rejects malformed token", async () => {
    const { pub } = await generateKeyPairB64();
    expect(await verifyToken("", pub)).toBeNull();
    expect(await verifyToken("only.two", pub)).toBeNull();
  });
});

function flipFirstChar(s: string): string {
  if (s.length === 0) return "A";
  const c = s.charAt(0);
  const replacement = c === "A" ? "B" : "A";
  return replacement + s.slice(1);
}
