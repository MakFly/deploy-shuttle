// Generate an Ed25519 keypair for the license server.
// Outputs base64 strings: copy LICENSE_PRIVATE_KEY_B64 to the server env,
// LICENSE_PUBLIC_KEY_B64 to both the server env and the CLI build env.

export {};

const pair = (await crypto.subtle.generateKey(
  { name: "Ed25519" },
  true,
  ["sign", "verify"],
)) as unknown as CryptoKeyPair;
const { publicKey, privateKey } = pair;

const pubRaw = new Uint8Array(await crypto.subtle.exportKey("raw", publicKey));
const privPkcs8 = new Uint8Array(await crypto.subtle.exportKey("pkcs8", privateKey));

// Strip the PKCS#8 wrapper to get the raw 32-byte seed (last 32 bytes).
const privRaw = privPkcs8.slice(privPkcs8.length - 32);

function toB64(bytes: Uint8Array): string {
  let bin = "";
  for (const byte of bytes) bin += String.fromCharCode(byte);
  return btoa(bin);
}

console.log("LICENSE_PRIVATE_KEY_B64=" + toB64(privRaw));
console.log("LICENSE_PUBLIC_KEY_B64=" + toB64(pubRaw));
