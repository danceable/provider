// Package i18n provides the blog's internationalization: a Language type, a
// translation Repository port, and a per-language Translator.
//
// The concrete, in-memory repository lives in the infrastructure layer. A
// request-scoped Translator is produced by a scoped service provider: the HTTP
// layer opens a scope per request seeded with the visitor's Language, and the
// scoped provider binds a Translator for it.
package i18n

import "strings"

// Language is a supported UI language, identified by its ISO 639-1 code.
type Language string

// The languages the blog ships translations for.
const (
	English Language = "en"
	German  Language = "de"
	Persian Language = "fa"
	Chinese Language = "zh"
)

// Default is the language used when none is requested or a key is missing.
const Default = English

// Supported lists every language the blog can render, in display order.
var Supported = []Language{English, German, Persian, Chinese}

// Native maps each language to its endonym, for the language switcher.
var Native = map[Language]string{
	English: "English",
	German:  "Deutsch",
	Persian: "فارسی",
	Chinese: "中文",
}

// Parse returns the Language for code, reporting whether it is supported.
func Parse(code string) (Language, bool) {
	lang := Language(strings.ToLower(strings.TrimSpace(code)))
	if _, ok := Native[lang]; ok {
		return lang, true
	}

	return "", false
}

// Repository returns translation values by language and key. Implementations
// are read-only and safe for concurrent use.
type Repository interface {
	// Translate returns the value for key in lang and whether it was found.
	Translate(lang Language, key string) (string, bool)
}

// Translator resolves keys for a single language, falling back to the default
// language and finally to the key itself, so a missing translation degrades
// visibly rather than rendering an empty string.
type Translator struct {
	repo Repository
	lang Language
}

// NewTranslator binds repo to a fixed language.
func NewTranslator(repo Repository, lang Language) *Translator {
	return &Translator{repo: repo, lang: lang}
}

// Lang returns the translator's language.
func (t *Translator) Lang() Language { return t.lang }

// Dir returns the text direction for the language: "rtl" for Persian, else "ltr".
func (t *Translator) Dir() string {
	if t.lang == Persian {
		return "rtl"
	}

	return "ltr"
}

// T returns the translation of key, falling back to the default language and
// then to the key itself.
func (t *Translator) T(key string) string {
	if v, ok := t.repo.Translate(t.lang, key); ok {
		return v
	}

	if v, ok := t.repo.Translate(Default, key); ok {
		return v
	}

	return key
}
