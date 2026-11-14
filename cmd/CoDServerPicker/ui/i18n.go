package ui

import (
	"embed"
	"encoding/json"
	"fmt"

	"fyne.io/fyne/v2"

	"github.com/1ukah/cod-server-picker/internal/config"
)

//go:embed i18n/*.json
var i18nFS embed.FS

//go:embed flags/*.svg
var flagFS embed.FS

// defaultLang is the fallback language for missing keys.
var defaultLang = config.Languages[0]

// languageNames are shown in the language selector. They are not translated.
var languageNames = map[string]string{
	"en": "English",
	"pt": "Português",
	"es": "Español",
}

// languageFlags maps a language to its flag image.
var languageFlags = map[string]string{
	"en": "us",
	"pt": "br",
	"es": "es",
}

// Translator returns the user interface text for a key.
type Translator struct {
	lang   string
	tables map[string]map[string]string
}

// NewTranslator loads the embedded translation files.
func NewTranslator() (*Translator, error) {
	tables := map[string]map[string]string{}
	for _, lang := range config.Languages {
		table, err := loadTable(lang)
		if err != nil {
			return nil, err
		}
		tables[lang] = table
	}
	return &Translator{lang: defaultLang, tables: tables}, nil
}

func loadTable(lang string) (map[string]string, error) {
	data, err := i18nFS.ReadFile("i18n/" + lang + ".json")
	if err != nil {
		return nil, err
	}
	table := map[string]string{}
	if err := json.Unmarshal(data, &table); err != nil {
		return nil, fmt.Errorf("i18n/%s.json: %w", lang, err)
	}
	return table, nil
}

// SetLang selects the language. An unknown language selects the default language.
func (t *Translator) SetLang(lang string) {
	if _, ok := t.tables[lang]; !ok {
		lang = defaultLang
	}
	t.lang = lang
}

func (t *Translator) Lang() string { return t.lang }

func (t *Translator) lookup(key string) (string, bool) {
	if s, ok := t.tables[t.lang][key]; ok {
		return s, true
	}
	s, ok := t.tables[defaultLang][key]
	return s, ok
}

// T returns the text for key. A missing key falls back to English, then to the key itself.
// If args are given, the text is a format string.
func (t *Translator) T(key string, args ...any) string {
	s, ok := t.lookup(key)
	if !ok {
		return key
	}
	if len(args) > 0 {
		return fmt.Sprintf(s, args...)
	}
	return s
}

// Name returns the translated name of a region or a preset. An unknown ID returns fallback.
func (t *Translator) Name(prefix, id, fallback string) string {
	if s, ok := t.lookup(prefix + id); ok {
		return s
	}
	return fallback
}

// flagResource returns the flag image of a language.
func flagResource(lang string) fyne.Resource {
	name := languageFlags[lang] + ".svg"
	data, err := flagFS.ReadFile("flags/" + name)
	if err != nil {
		return nil
	}
	return fyne.NewStaticResource(name, data)
}
