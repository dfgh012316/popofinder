package reconcile

import "strings"

// ExistingRow is a minimal projection of a medical_personnel row used for
// reconciliation. HasSource reflects whether source IS NOT NULL.
type ExistingRow struct {
	ID        int    `db:"id"`
	Name      string `db:"name"`
	Hospital  string `db:"hospital"`
	HasSource bool   `db:"has_source"`
}

// Plan describes the reconciliation outcome: which existing rows to mark as
// sourced from the blog, and which blog records to insert as new rows.
type Plan struct {
	MarkBlogIDs []int
	Inserts     []Record
}

// normalizeKey builds the reconciliation key from name + hospital, stripping
// all surrounding and internal whitespace to match CJK fields robustly.
func normalizeKey(name, hospital string) string {
	return strings.Join(strings.Fields(name), "") + "\x00" + strings.Join(strings.Fields(hospital), "")
}

// BuildPlan classifies blog records against existing rows. It is a pure
// function: matched-but-unclassified rows are marked, blog-only records are
// inserted (deduped), and everything else is left untouched. Running it again
// after applying the plan yields an empty plan (idempotent).
func BuildPlan(blog []Record, existing []ExistingRow) Plan {
	blogKeys := map[string]bool{}
	for _, r := range blog {
		blogKeys[normalizeKey(r.Name, r.Hospital)] = true
	}

	existingKeys := map[string]bool{}
	for _, e := range existing {
		existingKeys[normalizeKey(e.Name, e.Hospital)] = true
	}

	markBlogIDs := []int{}
	for _, e := range existing {
		key := normalizeKey(e.Name, e.Hospital)
		if blogKeys[key] && !e.HasSource {
			markBlogIDs = append(markBlogIDs, e.ID)
		}
	}

	inserts := []Record{}
	seen := map[string]bool{}
	for _, r := range blog {
		key := normalizeKey(r.Name, r.Hospital)
		if !existingKeys[key] && !seen[key] {
			inserts = append(inserts, r)
			seen[key] = true
		}
	}

	return Plan{MarkBlogIDs: markBlogIDs, Inserts: inserts}
}
