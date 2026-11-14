// Package config stores the user settings of the app in %LOCALAPPDATA%\CoDServerPicker.
package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/1ukah/cod-server-picker/internal/game"
)

const fileName = "settings.json"

type Settings struct {
	// Platform is empty until the user selects one on the first start.
	Platform game.Platform `json:"platform,omitempty"`
	// GamePath is the full path of cod.exe.
	GamePath string `json:"gamePath,omitempty"`
	// AllPrograms makes the rules apply to all programs, not only cod.exe.
	// Use it when rules for cod.exe do not work, for example with the Xbox app.
	AllPrograms bool `json:"allPrograms,omitempty"`
	// UpdateRegions downloads a newer regions list when the app starts.
	UpdateRegions bool `json:"updateRegions"`
	// CheckAppUpdates looks for a newer app release when the app starts.
	CheckAppUpdates bool `json:"checkAppUpdates"`
	// Language is the user interface language: "en", "pt" or "es".
	Language string `json:"language,omitempty"`
}

// Languages are the supported user interface languages. The first one is the default.
var Languages = []string{"en", "pt", "es"}

// Defaults returns the settings of a new user.
func Defaults() Settings {
	return Settings{UpdateRegions: true, CheckAppUpdates: true, Language: Languages[0]}
}

// Dir returns the data folder of the app and makes it if necessary.
func Dir() (string, error) {
	base, err := os.UserCacheDir() // %LOCALAPPDATA% on Windows
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "CoDServerPicker")
	return dir, os.MkdirAll(dir, 0o755)
}

// Load reads the settings. If no settings file exists, it returns Defaults.
func Load() (Settings, error) {
	s := Defaults()
	dir, err := Dir()
	if err != nil {
		return s, err
	}
	data, err := os.ReadFile(filepath.Join(dir, fileName))
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	err = json.Unmarshal(data, &s)
	return s, err
}

// Save writes the settings.
func Save(s Settings) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, fileName+".tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, fileName))
}
