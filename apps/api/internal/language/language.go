// Package language holds what surrounds a message in each language a customer
// can prefer: the discount code's label and the unsubscribe footer. The
// customer's profile locale picks it; the message copy itself is the author's.
// A language is added here and nowhere else in the backend.
package language

import "strings"

// Fallback is the language of a customer whose locale has no entry here.
const Fallback = "en"

// Language is one language the text around a message is written in.
type Language struct {
	Code string
	Name string
	// CodeLabel names the discount code in an email's code box and text line.
	CodeLabel string
	// NotWanted introduces the unsubscribe link in an email's footer.
	NotWanted   string
	Unsubscribe string
}

var all = []Language{
	{Code: "en", Name: "English", CodeLabel: "Your discount code", NotWanted: "Don't want these emails?", Unsubscribe: "Unsubscribe"},
	{Code: "tr", Name: "Turkish", CodeLabel: "İndirim kodun", NotWanted: "Bu e-postaları almak istemiyor musun?", Unsubscribe: "Abonelikten çık"},
	{Code: "de", Name: "German", CodeLabel: "Ihr Rabattcode", NotWanted: "Sie möchten diese E-Mails nicht mehr erhalten?", Unsubscribe: "Abmelden"},
	{Code: "fr", Name: "French", CodeLabel: "Votre code de réduction", NotWanted: "Vous ne souhaitez plus recevoir ces e-mails ?", Unsubscribe: "Se désabonner"},
	{Code: "es", Name: "Spanish", CodeLabel: "Tu código de descuento", NotWanted: "¿No quieres recibir estos correos?", Unsubscribe: "Darse de baja"},
}

// For reads a profile locale such as "tr", "tr-TR" or "de_AT" and returns its
// language, or English when there is none.
func For(locale string) Language {
	code := strings.ToLower(strings.TrimSpace(locale))
	if i := strings.IndexAny(code, "-_"); i >= 0 {
		code = code[:i]
	}
	for _, l := range all {
		if l.Code == code {
			return l
		}
	}
	return all[0]
}

// All lists every language.
func All() []Language { return append([]Language(nil), all...) }
