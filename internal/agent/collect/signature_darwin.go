//go:build darwin

package collect

import (
	"os/exec"
	"strings"
)

// signatureOf checks macOS code signing via codesign.
func signatureOf(path string) (bool, string) {
	out, err := exec.Command("codesign", "-dv", "--verbose=2", path).CombinedOutput()
	if err != nil {
		return false, ""
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Authority=") {
			signer := strings.TrimPrefix(line, "Authority=")
			return true, strings.TrimSpace(signer)
		}
	}
	return true, ""
}
