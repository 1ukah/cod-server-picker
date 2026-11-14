// Package monitor records the servers that the game connects to (advanced feature).
//
// Use it to find new server locations. It uses the Windows connection log (see package
// audit) and never touches the game process. Administrator rights are necessary.
package monitor

import (
	"context"
	"encoding/json"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/1ukah/cod-server-picker/internal/audit"
	"github.com/1ukah/cod-server-picker/internal/regions"
)

// Server is one remote address that the game used.
type Server struct {
	IP        string    `json:"ip"`
	Proto     string    `json:"proto"`
	Ports     []int     `json:"ports"`
	Count     int       `json:"count"`
	FirstSeen time.Time `json:"firstSeen"`
	LastSeen  time.Time `json:"lastSeen"`
	// Location is the known location that contains the IP, or "" if it is unknown.
	Location string `json:"location,omitempty"`
}

// Recording is the result of one monitor session.
type Recording struct {
	Started  time.Time          `json:"started"`
	Stopped  time.Time          `json:"stopped"`
	Note     string             `json:"note,omitempty"`
	GamePath string             `json:"gamePath,omitempty"`
	Servers  map[string]*Server `json:"servers"`

	mu sync.Mutex
}

// Run records until ctx is cancelled. onNew is called for each new server (it can be nil).
// The program name to watch is usually game.ExeName.
func Run(ctx context.Context, exeName, note string, known *regions.File, onNew func(Server)) (*Recording, error) {
	s, err := audit.Start()
	if err != nil {
		return nil, err
	}
	rec := &Recording{Started: time.Now(), Note: note, Servers: map[string]*Server{}}
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			rec.Stopped = time.Now()
			return rec, s.Stop()
		case <-tick.C:
			conns, err := s.Read()
			if err != nil {
				s.Stop()
				return rec, err
			}
			for _, c := range conns {
				if !strings.EqualFold(filepath.Base(c.Path), exeName) {
					continue
				}
				if srv, isNew := rec.add(c, known); isNew && onNew != nil {
					onNew(srv)
				}
			}
		}
	}
}

func (r *Recording) add(c audit.Conn, known *regions.File) (Server, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.GamePath == "" {
		r.GamePath = c.Path
	}
	key := c.Remote.String() + "/" + c.Proto
	srv, ok := r.Servers[key]
	if !ok {
		srv = &Server{IP: c.Remote.String(), Proto: c.Proto, FirstSeen: c.Time}
		if known != nil {
			if l := known.Find(c.Remote); l != nil {
				srv.Location = l.ID
			}
		}
		r.Servers[key] = srv
	}
	if c.Port > 0 && !slices.Contains(srv.Ports, c.Port) {
		srv.Ports = append(srv.Ports, c.Port)
	}
	srv.Count++
	srv.LastSeen = c.Time
	return *srv, !ok
}

// Unknown returns UDP servers that are not in a known location. These can be new locations.
func (r *Recording) Unknown() []Server {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Server
	for _, s := range r.Servers {
		if s.Proto == "udp" && s.Location == "" {
			out = append(out, *s)
		}
	}
	slices.SortFunc(out, func(a, b Server) int {
		x, _ := netip.ParseAddr(a.IP)
		y, _ := netip.ParseAddr(b.IP)
		return x.Compare(y)
	})
	return out
}

// Save writes the recording to dir and returns the file path.
func (r *Recording) Save(dir string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	p := filepath.Join(dir, "recording-"+r.Started.Format("2006-01-02-150405")+".json")
	return p, os.WriteFile(p, data, 0o644)
}
