//go:build linux

package collect

// signatureOf returns (signed, signer) for a Linux executable.
// Linux doesn't have a universal signature model; package managers provide
// package-level signing. We mark as unsigned for individual binaries.
func signatureOf(path string) (bool, string) {
	return false, ""
}
