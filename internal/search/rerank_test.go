package search

import "testing"

func TestTitleMatchBoostUsesOriginalQuery(t *testing.T) {
	// Rewritten FTS query contains "ranged" but original does not — boost should not apply.
	rewritten := "ranged attacks rate fire shooting crossbow"
	original := "how does RoF work for my weapon"
	boost := titleMatchBoost("Ranged Attacks", original)
	rewrittenBoost := titleMatchBoost("Ranged Attacks", rewritten)
	if boost >= rewrittenBoost {
		t.Fatalf("original boost %f should be less than rewritten boost %f when only rewritten matches title", boost, rewrittenBoost)
	}
	if rewrittenBoost <= 0 {
		t.Fatalf("expected positive boost from rewritten query, got %f", rewrittenBoost)
	}
}

func TestTableDensityPenalty(t *testing.T) {
	prose := `Rate of Fire is how many shots a ranged weapon can fire in one action.
For weapons with a Rate of Fire of 2 or higher, declare how many shots you are firing.`
	table := `Weapon Range Damage RoF Min Str Notes
Crossbow 15/30/60 2d6 1 d6 Reload 1
Pistol 5/10/20 2d6 1 d6 —
Repeating Crossbow 12/24/48 2d6 2 d6 Reload 2`

	prosePenalty := tableDensityPenalty(prose)
	tablePenalty := tableDensityPenalty(table)
	if tablePenalty <= prosePenalty {
		t.Fatalf("table penalty %f should exceed prose penalty %f", tablePenalty, prosePenalty)
	}
}

func TestAdjustedScorePrefersProseSection(t *testing.T) {
	embedScore := 0.72
	prose := adjustedScore(embedScore, "sec1", "Ranged Attacks", "Rate of Fire is how many shots a weapon fires.", "ranged attacks rate fire", nil)
	table := adjustedScore(embedScore, "sec2", "Black Powder Weapons", "Crossbow 15/30/60 2d6 1 d6\nPistol 5/10/20 2d6 1 d6", "repeating crossbow rate fire", nil)
	if prose <= table {
		t.Fatalf("prose adjusted %f should beat table adjusted %f", prose, table)
	}
}
