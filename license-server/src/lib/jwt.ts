// Ed25519 JWT-compatible signer that matches the Go verifier in
// go-cli/internal/license/token.go. Uses Web Crypto + base64url.

const HEADER = { alg: "EdDSA", typ: "JWT" } as const;

export type Claims = {
  key: string;
  tier: "pro";
  fp: string;
  iat: number; // unix seconds
  exp: number; // unix seconds
};

function b64urlEncode(bytes: Uint8Array): string {
  let bin = "";
  for (const byte of bytes) bin += String.fromCharCode(byte);
  return btoa(bin).replaceAll("+", "-").replaceAll("/", "_").replaceAll("=", "");
}

function b64Decode(b64: string): Uint8Array {
  // Accept either standard or URL-safe base64.
  const normalized = b64.replaceAll("-", "+").replaceAll("_", "/");
  const padded = normalized + "=".repeat((4 - (normalized.length % 4)) % 4);
  const bin = atob(padded);
  const bytes = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
  return bytes;
}

async function importPrivKey(b64: string): Promise<CryptoKey> {
  const raw = b64Decode(b64);
  if (raw.length !== 32) {
    throw new Error(`Ed25519 private key must be 32 bytes, got ${raw.length}`);
  }
  // Wrap raw 32-byte seed into PKCS#8 for Web Crypto.
  // PKCS#8 prefix for Ed25519 (RFC 8410): 30 2e 02 01 00 30 05 06 03 2b 65 70 04 22 04 20
  const pkcs8Prefix = new Uint8Array([
    0x30, 0x2e, 0x02, 0x01, 0x00, 0x30, 0x05, 0x06, 0x03, 0x2b, 0x65, 0x70, 0x04,
    0x22, 0x04, 0x20,
  ]);
  const pkcs8 = new Uint8Array(pkcs8Prefix.length + raw.length);
  pkcs8.set(pkcs8Prefix, 0);
  pkcs8.set(raw, pkcs8Prefix.length);
  return await crypto.subtle.importKey("pkcs8", pkcs8.buffer as ArrayBuffer, { name: "Ed25519" }, false, ["sign"]);
}

export async function signToken(claims: Claims, privateKeyB64: string): Promise<string> {
  const key = await importPrivKey(privateKeyB64);
  const headerJson = JSON.stringify(HEADER);
  const payloadJson = JSON.stringify(claims);
  const headerEnc = b64urlEncode(new TextEncoder().encode(headerJson));
  const payloadEnc = b64urlEncode(new TextEncoder().encode(payloadJson));
  const signing = new TextEncoder().encode(`${headerEnc}.${payloadEnc}`);
  const sig = new Uint8Array(await crypto.subtle.sign("Ed25519", key, signing.buffer as ArrayBuffer));
  return `${headerEnc}.${payloadEnc}.${b64urlEncode(sig)}`;
}

async function importPubKey(b64: string): Promise<CryptoKey> {
  const raw = b64Decode(b64);
  if (raw.length !== 32) {
    throw new Error(`Ed25519 public key must be 32 bytes, got ${raw.length}`);
  }
  return await crypto.subtle.importKey("raw", raw.buffer as ArrayBuffer, { name: "Ed25519" }, false, ["verify"]);
}

export async function verifyToken(
  token: string,
  publicKeyB64: string,
): Promise<Claims | null> {
  const parts = token.split(".");
  if (parts.length !== 3) return null;
  const [h, p, s] = parts as [string, string, string];
  let payload: Claims;
  try {
    payload = JSON.parse(new TextDecoder().decode(b64Decode(p)));
  } catch {
    return null;
  }
  const key = await importPubKey(publicKeyB64);
  const sigBytes = b64Decode(s);
  const signed = new TextEncoder().encode(`${h}.${p}`);
  const ok = await crypto.subtle.verify(
    "Ed25519",
    key,
    sigBytes.buffer as ArrayBuffer,
    signed.buffer as ArrayBuffer,
  );
  if (!ok) return null;
  return payload;
}
