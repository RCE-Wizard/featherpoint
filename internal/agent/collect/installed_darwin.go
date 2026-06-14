//go:build darwin

package collect

import (
	"encoding/xml"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/featherpoint/swinv/internal/proto"
)

// InstalledSoftware enumerates macOS .app bundles via Info.plist.
func InstalledSoftware() []proto.SoftwareDelta {
	dirs := []string{"/Applications"}
	if home := os.Getenv("HOME"); home != "" {
		dirs = append(dirs, filepath.Join(home, "Applications"))
	}

	var out []proto.SoftwareDelta
	for _, dir := range dirs {
		apps, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, app := range apps {
			if filepath.Ext(app.Name()) != ".app" {
				continue
			}
			plistPath := filepath.Join(dir, app.Name(), "Contents", "Info.plist")
			d := parsePlist(plistPath)
			if d != nil {
				out = append(out, *d)
			}
		}
	}
	return out
}

// parsePlist reads an Apple plist XML file and extracts bundle name + version.
func parsePlist(path string) *proto.SoftwareDelta {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	kv, err := plistTopLevelKV(f)
	if err != nil {
		log.Printf("plist %s: %v", path, err)
		return nil
	}

	name := kv["CFBundleName"]
	if name == "" {
		name = kv["CFBundleDisplayName"]
	}
	if name == "" {
		return nil
	}
	ver := kv["CFBundleShortVersionString"]

	return &proto.SoftwareDelta{
		Op:      "upsert",
		Source:  "installed",
		Name:    name,
		Version: strPtr(ver),
		Signed:  true,
	}
}

// plistTopLevelKV walks a plist XML token stream and collects top-level key→string pairs.
func plistTopLevelKV(r io.Reader) (map[string]string, error) {
	dec := xml.NewDecoder(r)
	kv := map[string]string{}
	var pendingKey string
	depth := 0

	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			switch {
			case t.Name.Local == "key" && depth == 3:
				var s string
				if err := dec.DecodeElement(&s, &t); err == nil {
					pendingKey = strings.TrimSpace(s)
				}
				depth--
			case t.Name.Local == "string" && depth == 3 && pendingKey != "":
				var s string
				if err := dec.DecodeElement(&s, &t); err == nil {
					kv[pendingKey] = s
				}
				pendingKey = ""
				depth--
			}
		case xml.EndElement:
			depth--
		}
	}
	return kv, nil
}
