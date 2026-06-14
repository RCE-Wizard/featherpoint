package collect

import (
	"net"
	"os"
	"runtime"
	"sort"
	"strings"

	"github.com/featherpoint/swinv/internal/proto"
	gopshost "github.com/shirou/gopsutil/v3/host"
)

// HostFacts gathers machine identity information for enrollment.
func HostFacts() proto.HostFacts {
	info, _ := gopshost.Info()

	hostname, _ := os.Hostname()
	fqdn := resolveFQDN(hostname)
	macs := sortedMACs()
	ip := primaryIP()

	f := proto.HostFacts{
		Hostname:     hostname,
		FQDN:         fqdn,
		OS:           runtime.GOOS,
		MACAddresses: macs,
	}
	if ip != "" {
		f.PrimaryIP = &ip
	}
	if info != nil {
		f.OSVersion = info.PlatformVersion
		if info.HostID != "" {
			serial := info.HostID
			f.SerialNumber = &serial
		}
	}
	return f
}

func resolveFQDN(hostname string) string {
	addrs, err := net.LookupHost(hostname)
	if err != nil || len(addrs) == 0 {
		return hostname
	}
	names, err := net.LookupAddr(addrs[0])
	if err != nil || len(names) == 0 {
		return hostname
	}
	return strings.TrimSuffix(names[0], ".")
}

func sortedMACs() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, iface := range ifaces {
		mac := iface.HardwareAddr.String()
		if mac == "" || mac == "00:00:00:00:00:00" || seen[mac] {
			continue
		}
		seen[mac] = true
		out = append(out, mac)
	}
	sort.Strings(out)
	return out
}

func primaryIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	addr := conn.LocalAddr().(*net.UDPAddr)
	return addr.IP.String()
}
