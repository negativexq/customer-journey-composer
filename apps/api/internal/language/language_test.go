package language

import "testing"

func TestALocaleFindsItsLanguageAndAnUnknownOneFallsBackToEnglish(t *testing.T) {
	for locale, want := range map[string]string{"tr": "tr", "tr-TR": "tr", " DE_at ": "de", "fr": "fr", "es-419": "es", "ru": Fallback, "": Fallback, "en-GB": "en"} {
		if got := For(locale).Code; got != want {
			t.Errorf("%q: got %s, want %s", locale, got, want)
		}
	}
	for _, l := range All() {
		if l.CodeLabel == "" || l.NotWanted == "" || l.Unsubscribe == "" {
			t.Errorf("%s is missing text", l.Code)
		}
	}
}
