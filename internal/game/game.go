// Package game finds the path of cod.exe.
//
// No function in this package opens, reads, writes or attaches to the game process.
// Ricochet anti-cheat watches for that type of access.
package game

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"github.com/1ukah/cod-server-picker/internal/audit"
)

// ExeName is the program name of all games in the Call of Duty HQ launcher.
const ExeName = "cod.exe"

// SteamAppID is the Steam app ID of Call of Duty HQ.
const SteamAppID = "1938090"

type Platform string

const (
	Steam     Platform = "steam"
	BattleNet Platform = "battlenet"
	Xbox      Platform = "xbox"
	Other     Platform = "other"
)

var ErrNotFound = errors.New("cod.exe was not found")

// FindSteam finds cod.exe in the Steam libraries. The game does not need to run.
func FindSteam() (string, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Valve\Steam`, registry.QUERY_VALUE)
	if err != nil {
		return "", fmt.Errorf("Steam is not installed: %w", err)
	}
	steam, _, err := k.GetStringValue("SteamPath")
	k.Close()
	if err != nil {
		return "", fmt.Errorf("Steam path not found: %w", err)
	}
	libs := []string{filepath.Clean(steam)}
	libs = append(libs, vdfValues(filepath.Join(steam, "steamapps", "libraryfolders.vdf"), "path")...)
	for _, lib := range libs {
		manifest := filepath.Join(lib, "steamapps", "appmanifest_"+SteamAppID+".acf")
		dirs := vdfValues(manifest, "installdir")
		if len(dirs) == 0 {
			continue
		}
		p := filepath.Join(lib, "steamapps", "common", dirs[0], ExeName)
		if fileExists(p) {
			return p, nil
		}
	}
	return "", ErrNotFound
}

var vdfLine = regexp.MustCompile(`^\s*"([^"]+)"\s+"([^"]*)"`)

// vdfValues returns all values of key in a Steam .vdf or .acf file.
func vdfValues(path, key string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if m := vdfLine.FindStringSubmatch(sc.Text()); m != nil && strings.EqualFold(m[1], key) {
			out = append(out, strings.ReplaceAll(m[2], `\\`, `\`))
		}
	}
	return out
}

// FindRunning returns the path of a running cod.exe.
//
// It lists processes with a snapshot and asks the kernel for the image path by process ID
// (SystemProcessIdInformation). Neither step opens a handle to the game process.
func FindRunning() (string, error) {
	pids, err := processIDs(ExeName)
	if err != nil {
		return "", err
	}
	for _, pid := range pids {
		nt, err := imagePath(pid)
		if err != nil {
			continue
		}
		if p := audit.DOSPath(nt); p != "" {
			return p, nil
		}
	}
	return "", ErrNotFound
}

// IsRunning reports whether a cod.exe process exists.
func IsRunning() bool {
	pids, err := processIDs(ExeName)
	return err == nil && len(pids) > 0
}

func processIDs(name string) ([]uint32, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snap)
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	var pids []uint32
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		if strings.EqualFold(windows.UTF16ToString(e.ExeFile[:]), name) {
			pids = append(pids, e.ProcessID)
		}
	}
	return pids, nil
}

const systemProcessIDInformation = 88

type systemProcessIDInfo struct {
	ProcessID uintptr
	ImageName windows.NTUnicodeString
}

// imagePath returns the NT path (\Device\HarddiskVolumeN\...) of a process without a handle.
func imagePath(pid uint32) (string, error) {
	buf := make([]uint16, 1024)
	for range 3 {
		info := systemProcessIDInfo{ProcessID: uintptr(pid)}
		info.ImageName.MaximumLength = uint16(len(buf) * 2)
		info.ImageName.Buffer = &buf[0]
		err := windows.NtQuerySystemInformation(systemProcessIDInformation, unsafe.Pointer(&info), uint32(unsafe.Sizeof(info)), nil)
		if err == windows.STATUS_INFO_LENGTH_MISMATCH {
			buf = make([]uint16, int(info.ImageName.MaximumLength)/2+1)
			continue
		}
		if err != nil {
			return "", err
		}
		return windows.UTF16ToString(buf[:info.ImageName.Length/2]), nil
	}
	return "", ErrNotFound
}

// FindFromConnections waits until cod.exe makes a network connection and returns its path.
// It uses the Windows connection log (see package audit). Administrator rights are necessary.
// The game must be running, or must be started before the timeout.
func FindFromConnections(ctx context.Context, timeout time.Duration) (string, error) {
	s, err := audit.Start()
	if err != nil {
		return "", err
	}
	defer s.Stop()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return "", ErrNotFound
		case <-tick.C:
			conns, err := s.Read()
			if err != nil {
				return "", err
			}
			for _, c := range conns {
				if strings.EqualFold(filepath.Base(c.Path), ExeName) {
					return c.Path, nil
				}
			}
		}
	}
}

// Find tries FindRunning first and FindFromConnections second.
func Find(ctx context.Context, timeout time.Duration) (string, error) {
	if p, err := FindRunning(); err == nil {
		return p, nil
	}
	return FindFromConnections(ctx, timeout)
}

// Valid reports whether path is an existing cod.exe file.
func Valid(path string) bool {
	return strings.EqualFold(filepath.Base(path), ExeName) && fileExists(path)
}

// IsProtectedPath reports whether the path is in the protected WindowsApps folder.
// Firewall rules for one program can fail for programs in this folder.
func IsProtectedPath(path string) bool {
	return strings.Contains(strings.ToLower(path), `\windowsapps\`)
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
