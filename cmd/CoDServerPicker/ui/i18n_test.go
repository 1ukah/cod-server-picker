package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/1ukah/cod-server-picker/internal/config"
	"github.com/1ukah/cod-server-picker/internal/game"
	"github.com/1ukah/cod-server-picker/internal/regions"
)

func TestTranslationsHaveTheSameKeys(t *testing.T) {
	tr, err := NewTranslator()
	if err != nil {
		t.Fatal(err)
	}
	en := tr.tables["en"]
	for _, lang := range config.Languages {
		table := tr.tables[lang]
		for key := range en {
			if _, ok := table[key]; !ok {
				t.Errorf("%s: missing key %q", lang, key)
			}
		}
		for key, text := range table {
			if _, ok := en[key]; !ok {
				t.Errorf("%s: key %q is not in the English file", lang, key)
			}
			if strings.TrimSpace(text) == "" {
				t.Errorf("%s: key %q is empty", lang, key)
			}
			if strings.Count(text, "%") != strings.Count(en[key], "%") {
				t.Errorf("%s: key %q has different format verbs", lang, key)
			}
		}
	}
}

func isEmoji(r rune) bool {
	switch {
	case r >= 0x1F000 && r <= 0x1FAFF, // pictographs, flags, symbols
		r >= 0x2600 && r <= 0x27BF,   // miscellaneous symbols, dingbats
		r >= 0x2300 && r <= 0x23FF,   // technical symbols
		r >= 0x2B00 && r <= 0x2BFF,   // stars and arrows
		r >= 0xE0020 && r <= 0xE007F, // tags
		r == 0xFE0F, r == 0x200D, r == 0x20E3:
		return true
	}
	return false
}

func TestTranslationsHaveNoEmoji(t *testing.T) {
	tr, err := NewTranslator()
	if err != nil {
		t.Fatal(err)
	}
	for lang, table := range tr.tables {
		for key, text := range table {
			for _, r := range text {
				if isEmoji(r) {
					t.Errorf("%s: key %q contains the emoji %U", lang, key, r)
				}
			}
		}
	}
	for _, name := range languageNames {
		for _, r := range name {
			if isEmoji(r) {
				t.Errorf("language name %q contains an emoji", name)
			}
		}
	}
}

func TestTranslatorFallback(t *testing.T) {
	tr := &Translator{lang: "pt", tables: map[string]map[string]string{
		"en": {"a": "A", "b": "B %d", "region.x": "X"},
		"pt": {"a": "Á"},
	}}
	if got := tr.T("a"); got != "Á" {
		t.Errorf("current language: %q", got)
	}
	if got := tr.T("b", 3); got != "B 3" {
		t.Errorf("fallback to English: %q", got)
	}
	if got := tr.T("missing.key"); got != "missing.key" {
		t.Errorf("fallback to the key: %q", got)
	}
	if got := tr.Name("region.", "x", "fallback"); got != "X" {
		t.Errorf("name: %q", got)
	}
	if got := tr.Name("region.", "new", "New Region"); got != "New Region" {
		t.Errorf("name of an unknown ID: %q", got)
	}
	tr.SetLang("xx")
	if tr.Lang() != "en" {
		t.Errorf("unknown language selects %q", tr.Lang())
	}
}

// The source files must use only keys that are in the English file.
func TestSourceKeysExist(t *testing.T) {
	tr, err := NewTranslator()
	if err != nil {
		t.Fatal(err)
	}
	en := tr.tables["en"]
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	call := regexp.MustCompile(`(?:\.t|settingCheck|reapply)\("([^"]+)"`)
	literal := regexp.MustCompile(`"((?:status|error)\.[A-Za-z.]+)"`)
	used := map[string]bool{}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, re := range []*regexp.Regexp{call, literal} {
			for _, m := range re.FindAllStringSubmatch(string(data), -1) {
				used[m[1]] = true
				if _, ok := en[m[1]]; !ok {
					t.Errorf("%s: key %q is not in the English file", file, m[1])
				}
			}
		}
	}
	for key := range en {
		dynamic := strings.HasPrefix(key, "region.") || strings.HasPrefix(key, "preset.") || strings.HasPrefix(key, "platform.")
		if !dynamic && !used[key] {
			t.Errorf("key %q is not used", key)
		}
	}
}

// Each region, preset and platform must have a translated name.
func TestDataNamesAreTranslated(t *testing.T) {
	tr, err := NewTranslator()
	if err != nil {
		t.Fatal(err)
	}
	f, err := regions.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	en := tr.tables["en"]
	for _, r := range f.Regions {
		if en["region."+r.ID] != r.Name {
			t.Errorf("region %q: English name %q, want %q", r.ID, en["region."+r.ID], r.Name)
		}
	}
	for _, p := range f.Presets {
		if en["preset."+p.ID] != p.Name {
			t.Errorf("preset %q: English name %q, want %q", p.ID, en["preset."+p.ID], p.Name)
		}
	}
	for _, p := range []game.Platform{game.Steam, game.BattleNet, game.Xbox, game.Other} {
		if _, ok := en["platform."+string(p)]; !ok {
			t.Errorf("platform %q has no name", p)
		}
	}
}

func TestFlagsExist(t *testing.T) {
	for _, lang := range config.Languages {
		if languageNames[lang] == "" {
			t.Errorf("language %q has no name", lang)
		}
		res := flagResource(lang)
		if res == nil || !strings.Contains(string(res.Content()), "<svg") {
			t.Errorf("language %q has no flag image", lang)
		}
	}
}
