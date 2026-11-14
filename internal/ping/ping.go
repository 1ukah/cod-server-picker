// Package ping measures the round-trip time to CoD server locations.
//
// Most game servers do not reply to ping. For these servers, the package sends pings with
// a limited TTL, like tracert, and uses the time of the last router that replies before
// the server. That router is in or near the same data center, so the time is close to the
// time to the server.
//
// The package uses the Windows ICMP API (iphlpapi.dll). Administrator rights are not necessary.
package ping

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	iphlpapi        = windows.NewLazySystemDLL("iphlpapi.dll")
	procCreateFile  = iphlpapi.NewProc("IcmpCreateFile")
	procCloseHandle = iphlpapi.NewProc("IcmpCloseHandle")
	procSendEcho    = iphlpapi.NewProc("IcmpSendEcho")
)

const (
	ipSuccess           = 0
	ipTTLExpiredTransit = 11013
	maxHops             = 30
	defaultHopTimeout   = 1000 * time.Millisecond
)

// Method tells how a result was measured.
type Method string

const (
	Direct  Method = "direct"   // the server replied
	LastHop Method = "last-hop" // a router near the server replied
	HTTP    Method = "http"     // a Google Cloud test endpoint replied
)

type Result struct {
	RTT    time.Duration
	Method Method
	Hop    netip.Addr // address that replied
}

var ErrNoReply = errors.New("no reply")

type ipOptionInformation struct {
	TTL         uint8
	TOS         uint8
	Flags       uint8
	OptionsSize uint8
	OptionsData uintptr
}

type icmpEchoReply struct {
	Address       uint32
	Status        uint32
	RoundTripTime uint32
	DataSize      uint16
	Reserved      uint16
	Data          uintptr
	Options       ipOptionInformation
}

// echo sends one ICMP echo with the given TTL. It returns the reply address, status and time.
func echo(ip netip.Addr, ttl uint8, timeout time.Duration) (netip.Addr, uint32, time.Duration, error) {
	h, _, err := procCreateFile.Call()
	if windows.Handle(h) == windows.InvalidHandle {
		return netip.Addr{}, 0, 0, err
	}
	defer procCloseHandle.Call(h)

	a := ip.As4()
	dst := *(*uint32)(unsafe.Pointer(&a[0]))
	data := [32]byte{'c', 'o', 'd', '-', 's', 'p'}
	opts := ipOptionInformation{TTL: ttl}
	reply := make([]byte, int(unsafe.Sizeof(icmpEchoReply{}))+len(data)+8+64)
	procSendEcho.Call(h, uintptr(dst), uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)),
		uintptr(unsafe.Pointer(&opts)), uintptr(unsafe.Pointer(&reply[0])), uintptr(len(reply)),
		uintptr(timeout.Milliseconds()))
	r := (*icmpEchoReply)(unsafe.Pointer(&reply[0]))
	if r.Address == 0 {
		return netip.Addr{}, 0, 0, ErrNoReply
	}
	from := netip.AddrFrom4(*(*[4]byte)(unsafe.Pointer(&r.Address)))
	return from, r.Status, time.Duration(r.RoundTripTime) * time.Millisecond, nil
}

// Measure returns the round-trip time to ip. Only IPv4 is supported.
func Measure(ctx context.Context, ip netip.Addr) (Result, error) {
	if !ip.Is4() {
		return Result{}, errors.New("only IPv4 is supported")
	}
	if from, status, rtt, err := echo(ip, 64, defaultHopTimeout); err == nil && status == ipSuccess && from == ip {
		return Result{RTT: rtt, Method: Direct, Hop: from}, nil
	}

	type hop struct {
		ttl  int
		from netip.Addr
		rtt  time.Duration
	}
	var (
		mu   sync.Mutex
		hops []hop
		wg   sync.WaitGroup
	)
	for ttl := 1; ttl <= maxHops; ttl++ {
		wg.Add(1)
		go func(ttl int) {
			defer wg.Done()
			if ctx.Err() != nil {
				return
			}
			from, status, rtt, err := echo(ip, uint8(ttl), defaultHopTimeout)
			if err != nil || (status != ipTTLExpiredTransit && status != ipSuccess) {
				return
			}
			mu.Lock()
			hops = append(hops, hop{ttl, from, rtt})
			mu.Unlock()
		}(ttl)
	}
	wg.Wait()
	if ctx.Err() != nil {
		return Result{}, ctx.Err()
	}
	if len(hops) == 0 {
		return Result{}, ErrNoReply
	}
	sort.Slice(hops, func(i, j int) bool { return hops[i].ttl < hops[j].ttl })
	for _, h := range hops {
		if h.from == ip {
			return Result{RTT: h.rtt, Method: Direct, Hop: h.from}, nil
		}
	}
	last := hops[len(hops)-1]
	// Routers often answer slowly. Measure the last hop again and keep the best time.
	best := last.rtt
	for range 2 {
		if from, status, rtt, err := echo(ip, uint8(last.ttl), defaultHopTimeout); err == nil && status == ipTTLExpiredTransit && from == last.from && rtt < best {
			best = rtt
		}
	}
	return Result{RTT: best, Method: LastHop, Hop: last.from}, nil
}

// MeasureURL returns the time of an HTTPS request to a gcping.com endpoint.
// The first request opens the connection. The next requests use the same connection, so
// their time is the round trip to the Google Cloud region. The best time is used.
func MeasureURL(ctx context.Context, url string) (Result, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	defer client.CloseIdleConnections()
	best := time.Duration(-1)
	for i := range 4 {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(url, "/")+"/api/ping", nil)
		if err != nil {
			return Result{}, err
		}
		start := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			return Result{}, err
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		d := time.Since(start)
		if i > 0 && (best < 0 || d < best) {
			best = d
		}
	}
	return Result{RTT: best.Round(time.Millisecond), Method: HTTP}, nil
}

// Target is one location to measure.
type Target struct {
	ID   string
	IPs  []string // measured with ICMP
	URLs []string // measured with HTTPS (gcping.com)
}

// MeasureTarget measures all IPs and URLs of a target in parallel.
//
// Direct and HTTP results are true round-trip times, so the lowest one is used.
// A last-hop result can only be lower than the true time (the router that replied can be
// far from the data center). If there are only last-hop results, the highest one is used.
func MeasureTarget(ctx context.Context, t Target) (Result, error) {
	var (
		mu         sync.Mutex
		exact, hop Result
		hasExact   bool
		hasHop     bool
		wg         sync.WaitGroup
	)
	keep := func(r Result, err error) {
		if err != nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if r.Method == LastHop {
			if !hasHop || r.RTT > hop.RTT {
				hop, hasHop = r, true
			}
		} else if !hasExact || r.RTT < exact.RTT {
			exact, hasExact = r, true
		}
	}
	for _, s := range t.IPs {
		ip, err := netip.ParseAddr(s)
		if err != nil || !ip.Is4() {
			continue
		}
		wg.Add(1)
		go func() { defer wg.Done(); keep(Measure(ctx, ip)) }()
	}
	for _, u := range t.URLs {
		wg.Add(1)
		go func() { defer wg.Done(); keep(MeasureURL(ctx, u)) }()
	}
	wg.Wait()
	switch {
	case hasExact:
		return exact, nil
	case hasHop:
		return hop, nil
	}
	return Result{}, ErrNoReply
}

// MeasureAll measures targets in parallel. onResult is called once for each target,
// from any goroutine, as soon as its result is ready.
func MeasureAll(ctx context.Context, targets []Target, parallel int, onResult func(id string, r Result, err error)) {
	if parallel < 1 {
		parallel = 8
	}
	sem := make(chan struct{}, parallel)
	var wg sync.WaitGroup
	for _, t := range targets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				onResult(t.ID, Result{}, ctx.Err())
				return
			}
			defer func() { <-sem }()
			r, err := MeasureTarget(ctx, t)
			onResult(t.ID, r, err)
		}()
	}
	wg.Wait()
}
