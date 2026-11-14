// Package audit reads the network connections that Windows records in the Security log.
//
// It turns on the audit subcategory "Filtering Platform Connection" (event 5156) for a
// short time. Windows then records each allowed connection with the program path and the
// remote address. This package never opens, reads or attaches to any other process.
// Administrator rights are necessary.
package audit

import (
	"encoding/xml"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/windows"

	"github.com/1ukah/cod-server-picker/internal/winexec"
)

// The GUID works on all Windows languages.
const subcategory = "{0CCE9226-69AE-11D9-BED3-505054503030}"

// Enabled reports whether connection auditing (success events) is on.
// If the app stops before Stop is called, the setting stays on. Administrator rights are necessary.
func Enabled() (bool, error) {
	out, err := winexec.Command("auditpol", "/get", "/subcategory:"+subcategory, "/r").Output()
	if err != nil {
		return false, fmt.Errorf("auditpol get: %v", err)
	}
	// The /r output is CSV. The last field is "Setting Value": 0 none, 1 success, 2 failure, 3 both.
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return false, fmt.Errorf("auditpol get: unexpected output")
	}
	fields := strings.Split(strings.TrimSpace(lines[len(lines)-1]), ",")
	v, err := strconv.Atoi(strings.TrimSpace(fields[len(fields)-1]))
	if err != nil {
		return false, fmt.Errorf("auditpol get: unexpected value")
	}
	return v&1 != 0, nil
}

// Reset turns off connection auditing (success events) and removes old backup files.
// Use it if the app stopped during a recording. Administrator rights are necessary.
func Reset() error {
	if out, err := winexec.Command("auditpol", "/set", "/subcategory:"+subcategory, "/success:disable").CombinedOutput(); err != nil {
		return fmt.Errorf("auditpol set: %v: %s", err, out)
	}
	old, _ := filepath.Glob(filepath.Join(os.TempDir(), "CoDServerPicker-auditpol-*.csv"))
	for _, f := range old {
		os.Remove(f)
	}
	return nil
}

// Conn is one connection from the Security log.
type Conn struct {
	Time   time.Time
	PID    int
	Path   string // DOS path of the program, for example C:\Games\cod.exe
	Remote netip.Addr
	Port   int
	Proto  string // "udp", "tcp" or "protoN"
}

// Session keeps the audit setting on until Stop is called.
type Session struct {
	backup string
	last   uint64
	local  map[netip.Addr]bool
	drives map[string]string
}

// Start saves the current audit policy and turns on connection auditing.
func Start() (*Session, error) {
	if !windows.GetCurrentProcessToken().IsElevated() {
		return nil, fmt.Errorf("administrator rights are necessary")
	}
	backup := filepath.Join(os.TempDir(), fmt.Sprintf("CoDServerPicker-auditpol-%d.csv", os.Getpid()))
	os.Remove(backup)
	if out, err := winexec.Command("auditpol", "/backup", "/file:"+backup).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("auditpol backup: %v: %s", err, out)
	}
	s := &Session{backup: backup, local: localAddrs(), drives: dosDevices()}
	if out, err := winexec.Command("auditpol", "/set", "/subcategory:"+subcategory, "/success:enable").CombinedOutput(); err != nil {
		os.Remove(backup)
		return nil, fmt.Errorf("auditpol set: %v: %s", err, out)
	}
	last, err := latestRecordID()
	if err != nil {
		s.Stop()
		return nil, err
	}
	s.last = last
	return s, nil
}

// Stop restores the audit policy that was saved by Start.
func (s *Session) Stop() error {
	out, err := winexec.Command("auditpol", "/restore", "/file:"+s.backup).CombinedOutput()
	if err != nil {
		return fmt.Errorf("auditpol restore: %v: %s (to fix it, run: auditpol /set /subcategory:%s /success:disable)", err, out, subcategory)
	}
	os.Remove(s.backup)
	return nil
}

// Read returns the connections recorded since the last call.
// Only connections to public addresses are returned.
func (s *Session) Read() ([]Conn, error) {
	q := fmt.Sprintf("*[System[(EventID=5156) and (EventRecordID>%d)]]", s.last)
	events, err := query("/q:"+q, "/f:xml", "/e:Events")
	if err != nil {
		return nil, err
	}
	var out []Conn
	for _, e := range events {
		if e.RecordID > s.last {
			s.last = e.RecordID
		}
		f := e.fields()
		remote, port, ok := s.pickRemote(f)
		if !ok {
			continue
		}
		pid, _ := strconv.Atoi(f["ProcessID"])
		t, _ := time.Parse(time.RFC3339Nano, e.Time.SystemTime)
		out = append(out, Conn{
			Time:   t,
			PID:    pid,
			Path:   s.dosPath(f["Application"]),
			Remote: remote,
			Port:   port,
			Proto:  protoName(f["Protocol"]),
		})
	}
	return out, nil
}

type evtData struct {
	Name  string `xml:"Name,attr"`
	Value string `xml:",chardata"`
}

type evt struct {
	RecordID uint64 `xml:"System>EventRecordID"`
	Time     struct {
		SystemTime string `xml:"SystemTime,attr"`
	} `xml:"System>TimeCreated"`
	Data []evtData `xml:"EventData>Data"`
}

func (e evt) fields() map[string]string {
	m := make(map[string]string, len(e.Data))
	for _, d := range e.Data {
		m[d.Name] = strings.TrimSpace(d.Value)
	}
	return m
}

func query(args ...string) ([]evt, error) {
	out, err := winexec.Command("wevtutil", append([]string{"qe", "Security"}, args...)...).Output()
	if err != nil {
		return nil, fmt.Errorf("wevtutil: %v", err)
	}
	var v struct {
		Events []evt `xml:"Event"`
	}
	if err := xml.Unmarshal([]byte(strings.ToValidUTF8(string(out), "?")), &v); err != nil {
		return nil, err
	}
	return v.Events, nil
}

func latestRecordID() (uint64, error) {
	events, err := query("/c:1", "/rd:true", "/f:xml", "/e:Events")
	if err != nil || len(events) == 0 {
		return 0, err
	}
	return events[0].RecordID, nil
}

func (s *Session) pickRemote(f map[string]string) (netip.Addr, int, bool) {
	src, srcErr := netip.ParseAddr(f["SourceAddress"])
	dst, dstErr := netip.ParseAddr(f["DestAddress"])
	var ip netip.Addr
	var port string
	switch {
	case dstErr == nil && !s.local[dst.Unmap()]:
		ip, port = dst, f["DestPort"]
	case srcErr == nil && !s.local[src.Unmap()]:
		ip, port = src, f["SourcePort"]
	default:
		return netip.Addr{}, 0, false
	}
	ip = ip.Unmap()
	if !IsPublic(ip) {
		return netip.Addr{}, 0, false
	}
	p, _ := strconv.Atoi(port)
	return ip, p, true
}

var cgnat = netip.MustParsePrefix("100.64.0.0/10")

// IsPublic reports whether ip is a public internet address.
func IsPublic(ip netip.Addr) bool {
	return ip.IsValid() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsMulticast() &&
		!ip.IsLinkLocalUnicast() && !ip.IsUnspecified() && !cgnat.Contains(ip)
}

func localAddrs() map[netip.Addr]bool {
	m := map[netip.Addr]bool{}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return m
	}
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok {
			if ip, ok := netip.AddrFromSlice(n.IP); ok {
				m[ip.Unmap()] = true
			}
		}
	}
	return m
}

func protoName(n string) string {
	switch n {
	case "6":
		return "tcp"
	case "17":
		return "udp"
	}
	return "proto" + n
}

// dosDevices maps NT device names (\device\harddiskvolume3) to drive letters (C:).
func dosDevices() map[string]string {
	m := map[string]string{}
	buf := make([]uint16, 1024)
	for d := 'A'; d <= 'Z'; d++ {
		drive := string(d) + ":"
		n, err := windows.QueryDosDevice(windows.StringToUTF16Ptr(drive), &buf[0], uint32(len(buf)))
		if err != nil || n == 0 {
			continue
		}
		m[strings.ToLower(windows.UTF16ToString(buf))] = drive
	}
	return m
}

// DOSPath converts an NT device path to a DOS path. It returns "" if no drive matches.
func DOSPath(p string) string {
	return (&Session{drives: dosDevices()}).dosPath(p)
}

func (s *Session) dosPath(p string) string {
	lower := strings.ToLower(p)
	for dev, drive := range s.drives {
		if strings.HasPrefix(lower, dev+`\`) {
			return drive + p[len(dev):]
		}
	}
	if len(p) > 2 && p[1] == ':' {
		return p
	}
	return ""
}
