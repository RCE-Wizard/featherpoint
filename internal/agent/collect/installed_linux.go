//go:build linux

package collect

import (
	"bufio"
	"log"
	"os"
	"strings"

	"github.com/featherpoint/swinv/internal/proto"
)

// InstalledSoftware enumerates installed packages on Linux.
// Reads dpkg status directly — does not shell out to dpkg -l.
func InstalledSoftware() []proto.SoftwareDelta {
	var out []proto.SoftwareDelta
	out = append(out, readDpkg()...)
	return out
}

func readDpkg() []proto.SoftwareDelta {
	f, err := os.Open("/var/lib/dpkg/status")
	if err != nil {
		return nil // not a Debian-based system
	}
	defer f.Close()

	var out []proto.SoftwareDelta
	var pkg, ver, maint string
	var installed bool

	flush := func() {
		if pkg != "" && installed {
			v := ver
			m := maint
			out = append(out, proto.SoftwareDelta{
				Op:        "upsert",
				Source:    "installed",
				Name:      pkg,
				Publisher: strPtr(m),
				Version:   strPtr(v),
				Signed:    true, // dpkg packages are signed at the repo level
			})
		}
		pkg, ver, maint = "", "", ""
		installed = false
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			flush()
			continue
		}
		switch {
		case strings.HasPrefix(line, "Package: "):
			pkg = strings.TrimPrefix(line, "Package: ")
		case strings.HasPrefix(line, "Version: "):
			ver = strings.TrimPrefix(line, "Version: ")
		case strings.HasPrefix(line, "Maintainer: "):
			maint = strings.TrimPrefix(line, "Maintainer: ")
		case strings.HasPrefix(line, "Status: "):
			installed = strings.Contains(line, "install ok installed")
		}
	}
	flush()

	if err := scanner.Err(); err != nil {
		log.Printf("dpkg scan: %v", err)
	}
	return out
}
