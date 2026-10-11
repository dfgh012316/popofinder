package crawl

import (
	"database/sql"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"
)

// ExistingRow is a crawled row already in medical_personnel (source_key IS NOT NULL).
type ExistingRow struct {
	ID         int            `db:"id"`
	Key        string         `db:"source_key"`
	Name       string         `db:"name"`
	City       string         `db:"city"`
	Hospital   string         `db:"hospital"`
	Department sql.NullString `db:"department"`
	Education  sql.NullString `db:"education"`
	URL        sql.NullString `db:"source_url"`
}

// Row is the normalized value written for one doctor; Department "" means NULL.
type Row struct {
	Key, Name, City, Hospital, Department, Education, URL string
}

type RowUpdate struct {
	ID  int
	Row Row
}

// SyncPlan is the outcome of comparing one source's crawl with its existing rows.
type SyncPlan struct {
	Inserts           []Row
	Updates           []RowUpdate
	TouchIDs          []int
	DeleteIDs         []int
	SkippedSuppressed int
	SkippedNoDegree   int
	DeleteBlocked     bool
}

// rowFromDoctor normalizes a crawled doctor to fit the medical_personnel column limits.
func rowFromDoctor(d Doctor) Row {
	var depts []string
	seen := map[string]bool{}
	for _, dept := range d.Departments {
		dept = strings.TrimSpace(dept)
		if dept == "" || seen[dept] {
			continue
		}
		seen[dept] = true
		depts = append(depts, dept)
	}
	// Drop whole departments from the end rather than cutting one in half.
	for len(depts) > 1 && utf8.RuneCountInString(strings.Join(depts, "、")) > 100 {
		depts = depts[:len(depts)-1]
	}
	return Row{
		Key:        d.Key,
		Name:       truncateRunes(strings.Join(strings.Fields(d.Name), ""), 100),
		City:       strings.TrimSpace(d.City),
		Hospital:   truncateRunes(strings.TrimSpace(d.Hospital), 200),
		Department: truncateRunes(strings.Join(depts, "、"), 100),
		Education:  formatEducation(d.Education),
		URL:        strings.TrimSpace(d.URL),
	}
}

func sameRow(e ExistingRow, r Row) bool {
	return e.Name == r.Name && e.City == r.City && e.Hospital == r.Hospital &&
		e.Department.String == r.Department && e.Education.String == r.Education && e.URL.String == r.URL
}

// BuildSyncPlan compares one source's crawl with the rows it wrote before
// (docs/open-decisions.md OD-10). It refuses to delete anything when fewer than
// half as many eligible doctors were crawled as there are existing rows.
func BuildSyncPlan(crawled []Doctor, existing []ExistingRow, suppressed map[string]bool) SyncPlan {
	var plan SyncPlan
	byKey := make(map[string]ExistingRow, len(existing))
	for _, e := range existing {
		byKey[e.Key] = e
	}

	seen := map[string]bool{}
	kept := map[int]bool{}
	eligible := 0
	for _, d := range crawled {
		if seen[d.Key] {
			continue
		}
		seen[d.Key] = true
		if suppressed[d.Key] {
			plan.SkippedSuppressed++
			continue
		}
		e, exists := byKey[d.Key]
		if !hasMedicalDegree(d.Education) {
			if !exists {
				plan.SkippedNoDegree++
			}
			continue
		}
		eligible++
		row := rowFromDoctor(d)
		if !exists {
			plan.Inserts = append(plan.Inserts, row)
			continue
		}
		kept[e.ID] = true
		if sameRow(e, row) {
			plan.TouchIDs = append(plan.TouchIDs, e.ID)
		} else {
			plan.Updates = append(plan.Updates, RowUpdate{ID: e.ID, Row: row})
		}
	}
	for _, e := range existing {
		if !kept[e.ID] {
			plan.DeleteIDs = append(plan.DeleteIDs, e.ID)
		}
	}
	if len(existing) > 0 && eligible*2 < len(existing) {
		plan.DeleteIDs = nil
		plan.DeleteBlocked = true
	}

	sort.Slice(plan.Inserts, func(i, j int) bool { return plan.Inserts[i].Key < plan.Inserts[j].Key })
	sort.Slice(plan.Updates, func(i, j int) bool { return plan.Updates[i].ID < plan.Updates[j].ID })
	slices.Sort(plan.TouchIDs)
	slices.Sort(plan.DeleteIDs)
	return plan
}
