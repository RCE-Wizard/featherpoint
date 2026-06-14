//go:build windows

package collect

import (
	"log"

	"github.com/featherpoint/swinv/internal/proto"
	"golang.org/x/sys/windows/registry"
)

var uninstallKeys = []struct {
	root registry.Key
	path string
}{
	{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
	{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`},
	{registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
}

// InstalledSoftware reads the Windows registry Uninstall keys.
// Never uses Win32_Product — that triggers MSI self-repair.
func InstalledSoftware() []proto.SoftwareDelta {
	var out []proto.SoftwareDelta
	for _, entry := range uninstallKeys {
		out = append(out, readUninstallKey(entry.root, entry.path)...)
	}
	return out
}

func readUninstallKey(root registry.Key, path string) []proto.SoftwareDelta {
	k, err := registry.OpenKey(root, path, registry.READ)
	if err != nil {
		return nil
	}
	defer k.Close()

	subkeys, err := k.ReadSubKeyNames(-1)
	if err != nil {
		log.Printf("registry subkeys %s: %v", path, err)
		return nil
	}

	var out []proto.SoftwareDelta
	for _, sub := range subkeys {
		sk, err := registry.OpenKey(k, sub, registry.READ)
		if err != nil {
			continue
		}
		name, _, _ := sk.GetStringValue("DisplayName")
		ver, _, _ := sk.GetStringValue("DisplayVersion")
		publisher, _, _ := sk.GetStringValue("Publisher")
		location, _, _ := sk.GetStringValue("InstallLocation")
		sk.Close()

		if name == "" {
			continue
		}

		d := proto.SoftwareDelta{
			Op:      "upsert",
			Source:  "installed",
			Name:    name,
			Version: strPtr(ver),
		}
		if publisher != "" {
			d.Publisher = strPtr(publisher)
		}
		if location != "" {
			d.InstallLocation = strPtr(location)
		}
		out = append(out, d)
	}
	return out
}
