package workflow

import (
	"math"
	"testing"
)

func TestCorrelationRequiresPresentScalarIdentity(t *testing.T) {
	c := &ExitCriteria{Correlation: &EventCorrelation{EntryPath: "event.properties.cart_id", EventPath: "event.properties.paid_cart"}}
	for _, tc := range []struct {
		name            string
		entry, incoming any
		want            bool
	}{
		{"same", "A", "A", true}, {"different", "A", "B", false}, {"missing", nil, nil, false},
		{"blank", "", "", false}, {"whitespace", " ", " ", false}, {"typed", 1, "1", false},
		{"numeric", 1, 1.0, true}, {"object", map[string]any{"id": "A"}, map[string]any{"id": "A"}, false},
		{"boolean", true, true, false}, {"infinite", math.Inf(1), math.Inf(1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := c.MatchesEntry(map[string]any{"cart_id": tc.entry}, map[string]any{"paid_cart": tc.incoming}); got != tc.want {
				t.Fatal(got, tc.want)
			}
		})
	}
	if !(&ExitCriteria{}).MatchesEntry(nil, nil) {
		t.Fatal("legacy customer-wide behavior changed")
	}
	if c.MatchesEntry(map[string]any{}, map[string]any{}) {
		t.Fatal("missing paths matched")
	}
}
func TestCorrelationPathValidation(t *testing.T) {
	for _, path := range []string{"event.properties.cart_id", "event.properties.order.id", "customer.id", "context.cart", "event.properties.", "event.properties.__proto__", "event.properties.cart[0]"} {
		w := valid()
		w.GlobalExit = &ExitCriteria{EventName: "purchased", Correlation: &EventCorrelation{EntryPath: path, EventPath: path}}
		want := path == "event.properties.cart_id" || path == "event.properties.order.id"
		if got := len(Validate(w)) == 0; got != want {
			t.Fatalf("%s valid=%v", path, got)
		}
	}
}
