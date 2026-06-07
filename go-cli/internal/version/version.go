// Package version exposes build-time metadata. Values are injected via
// `-ldflags "-X github.com/MakFly/deploy-shuttle/go-cli/internal/version.Version=..."` at build time.
package version

// Version is the CLI release. Defaults to "dev" when not injected.
var Version = "dev"

// LicensePubKeyB64 is the Ed25519 public key (base64-encoded raw 32 bytes)
// used to verify license tokens offline. Empty in dev builds.
var LicensePubKeyB64 = ""

// LicenseServer is the default license server endpoint. Can be overridden at
// runtime via --server or SHUTTLE_LICENSE_SERVER.
var LicenseServer = "https://license.deployshuttle.io"
