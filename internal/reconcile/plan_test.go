package reconcile

import (
	"reflect"
	"testing"
)

func TestBuildPlan_MarksUnclassifiedMatch(t *testing.T) {
	existing := []ExistingRow{{ID: 1, Name: "陳柏誠", Hospital: "大里仁愛醫院", HasSource: false}}
	blog := []Record{{City: "台中", Hospital: "大里仁愛醫院", Name: "陳柏誠", Education: "波蘭"}}

	got := BuildPlan(blog, existing)

	if !reflect.DeepEqual(got.MarkBlogIDs, []int{1}) {
		t.Fatalf("MarkBlogIDs = %v, want [1]", got.MarkBlogIDs)
	}
}

func TestBuildPlan_SkipsAlreadyClassified(t *testing.T) {
	existing := []ExistingRow{{ID: 1, Name: "陳柏誠", Hospital: "大里仁愛醫院", HasSource: true}}
	blog := []Record{{City: "台中", Hospital: "大里仁愛醫院", Name: "陳柏誠", Education: "波蘭"}}

	got := BuildPlan(blog, existing)

	if len(got.MarkBlogIDs) != 0 {
		t.Fatalf("MarkBlogIDs = %v, want empty", got.MarkBlogIDs)
	}
}

func TestBuildPlan_InsertsBlogOnly(t *testing.T) {
	blog := []Record{{City: "台中", Hospital: "大里仁愛醫院", Department: "", Name: "陳柏誠", Education: "波蘭"}}

	got := BuildPlan(blog, nil)

	if len(got.Inserts) != 1 {
		t.Fatalf("Inserts len = %d, want 1", len(got.Inserts))
	}
	if !reflect.DeepEqual(got.Inserts[0], blog[0]) {
		t.Fatalf("Inserts[0] = %+v, want %+v", got.Inserts[0], blog[0])
	}
}

func TestBuildPlan_LeavesDbOnlyUntouched(t *testing.T) {
	existing := []ExistingRow{{ID: 9, Name: "王大明", Hospital: "某院", HasSource: false}}
	blog := []Record{{City: "台中", Hospital: "大里仁愛醫院", Name: "陳柏誠", Education: "波蘭"}}

	got := BuildPlan(blog, existing)

	if len(got.MarkBlogIDs) != 0 {
		t.Fatalf("MarkBlogIDs = %v, want empty", got.MarkBlogIDs)
	}
	if len(got.Inserts) != 1 {
		t.Fatalf("Inserts len = %d, want 1", len(got.Inserts))
	}
}

func TestBuildPlan_NormalizesWhitespace(t *testing.T) {
	existing := []ExistingRow{{ID: 5, Name: " 陳柏誠 ", Hospital: "大里仁愛醫院 ", HasSource: false}}
	blog := []Record{{City: "台中", Hospital: "大里仁愛醫院", Name: "陳柏誠", Education: "波蘭"}}

	got := BuildPlan(blog, existing)

	if !reflect.DeepEqual(got.MarkBlogIDs, []int{5}) {
		t.Fatalf("MarkBlogIDs = %v, want [5]", got.MarkBlogIDs)
	}
	if len(got.Inserts) != 0 {
		t.Fatalf("Inserts len = %d, want 0", len(got.Inserts))
	}
}

func TestBuildPlan_DedupsBlogInserts(t *testing.T) {
	blog := []Record{
		{City: "台中", Hospital: "大里仁愛醫院", Name: "陳柏誠", Education: "波蘭"},
		{City: "台中", Hospital: "大里仁愛醫院", Name: "陳柏誠", Education: "波蘭"},
	}

	got := BuildPlan(blog, nil)

	if len(got.Inserts) != 1 {
		t.Fatalf("Inserts len = %d, want 1", len(got.Inserts))
	}
}

func TestBuildPlan_Idempotent(t *testing.T) {
	existing := []ExistingRow{{ID: 1, Name: "陳柏誠", Hospital: "大里仁愛醫院", HasSource: false}}
	blog := []Record{
		{City: "台中", Hospital: "大里仁愛醫院", Name: "陳柏誠", Education: "波蘭"},
		{City: "台北", Hospital: "某某診所", Name: "林小明", Education: "波蘭"},
	}

	p1 := BuildPlan(blog, existing)

	markSet := map[int]bool{}
	for _, id := range p1.MarkBlogIDs {
		markSet[id] = true
	}
	for i := range existing {
		if markSet[existing[i].ID] {
			existing[i].HasSource = true
		}
	}
	for _, r := range p1.Inserts {
		existing = append(existing, ExistingRow{Name: r.Name, Hospital: r.Hospital, HasSource: true})
	}

	p2 := BuildPlan(blog, existing)

	if len(p2.MarkBlogIDs) != 0 {
		t.Fatalf("second MarkBlogIDs = %v, want empty", p2.MarkBlogIDs)
	}
	if len(p2.Inserts) != 0 {
		t.Fatalf("second Inserts = %+v, want empty", p2.Inserts)
	}
}
