package crawl

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

const (
	mdLine     = "國立台灣大學醫學系"
	masterLine = "輔仁大學公共衛生學系碩士"
)

// planDoctor is a fictional crawled doctor in 內科 with the given education lines.
func planDoctor(key string, education ...string) Doctor {
	return Doctor{
		Key:         key,
		Name:        "王一",
		City:        "新北",
		Hospital:    "甲醫院",
		Departments: []string{"內科"},
		Education:   education,
		URL:         "https://example.invalid/" + key,
	}
}

// planExisting is the stored row matching planDoctor(key, education).
func planExisting(id int, key, education string) ExistingRow {
	return ExistingRow{
		ID:         id,
		Key:        key,
		Name:       "王一",
		City:       "新北",
		Hospital:   "甲醫院",
		Department: sql.NullString{String: "內科", Valid: true},
		Education:  sql.NullString{String: education, Valid: true},
		URL:        sql.NullString{String: "https://example.invalid/" + key, Valid: true},
	}
}

func TestBuildSyncPlanInsertsNewDoctors(t *testing.T) {
	a := planDoctor("t:2", masterLine, mdLine)
	a.Name = "王 一"
	a.Departments = []string{"內科", "內科", "家醫科"}
	b := planDoctor("t:1", mdLine)

	plan := BuildSyncPlan([]Doctor{a, b}, nil, nil)

	require.Len(t, plan.Inserts, 2)
	require.Equal(t, "t:1", plan.Inserts[0].Key)
	got := plan.Inserts[1]
	require.Equal(t, "王一", got.Name)
	require.Equal(t, "內科、家醫科", got.Department)
	require.Equal(t, masterLine+"；"+mdLine, got.Education)
	require.False(t, plan.DeleteBlocked)
}

func TestBuildSyncPlanUpdatesTouchesAndDeletes(t *testing.T) {
	existing := []ExistingRow{
		planExisting(1, "t:a", mdLine),
		planExisting(2, "t:b", mdLine),
		planExisting(3, "t:c", mdLine),
	}
	crawled := []Doctor{
		planDoctor("t:a", mdLine),
		planDoctor("t:b", "國立陽明大學醫學系"),
		planDoctor("t:d", mdLine),
	}

	plan := BuildSyncPlan(crawled, existing, nil)

	require.Len(t, plan.Inserts, 1)
	require.Equal(t, "t:d", plan.Inserts[0].Key)
	require.Len(t, plan.Updates, 1)
	require.Equal(t, 2, plan.Updates[0].ID)
	require.Equal(t, "國立陽明大學醫學系", plan.Updates[0].Row.Education)
	require.Equal(t, []int{1}, plan.TouchIDs)
	require.Equal(t, []int{3}, plan.DeleteIDs)
	require.False(t, plan.DeleteBlocked)
}

func TestBuildSyncPlanSkipsSuppressedAndNoDegree(t *testing.T) {
	existing := []ExistingRow{
		planExisting(7, "t:g", mdLine),
		planExisting(8, "t:h", mdLine),
		planExisting(9, "t:i", mdLine),
		planExisting(10, "t:j", mdLine),
	}
	crawled := []Doctor{
		planDoctor("t:e", mdLine),
		planDoctor("t:f", masterLine),
		planDoctor("t:g", masterLine),
		planDoctor("t:h", mdLine),
		planDoctor("t:i", mdLine),
		planDoctor("t:j", mdLine),
	}

	plan := BuildSyncPlan(crawled, existing, map[string]bool{"t:e": true})

	require.Empty(t, plan.Inserts)
	require.Equal(t, 1, plan.SkippedSuppressed)
	require.Equal(t, 1, plan.SkippedNoDegree)
	require.Equal(t, []int{7}, plan.DeleteIDs)
	require.Equal(t, []int{8, 9, 10}, plan.TouchIDs)
	require.False(t, plan.DeleteBlocked)
}

func TestBuildSyncPlanBlocksMassDelete(t *testing.T) {
	var existing []ExistingRow
	var crawled []Doctor
	for i := 1; i <= 10; i++ {
		key := fmt.Sprintf("t:%d", i)
		existing = append(existing, planExisting(i, key, mdLine))
		if i <= 4 {
			crawled = append(crawled, planDoctor(key, mdLine))
		}
	}

	plan := BuildSyncPlan(crawled, existing, nil)

	require.True(t, plan.DeleteBlocked)
	require.Empty(t, plan.DeleteIDs)
	require.Len(t, plan.TouchIDs, 4)
}

func TestBuildSyncPlanFirstRunNeverBlocks(t *testing.T) {
	plan := BuildSyncPlan(nil, nil, nil)
	require.False(t, plan.DeleteBlocked)
}

func TestRowFromDoctorLimits(t *testing.T) {
	var depts []string
	for _, c := range []string{"甲", "乙", "丙", "丁", "戊"} {
		depts = append(depts, strings.Repeat(c, 30))
	}
	d := planDoctor("t:1", mdLine)
	d.Departments = depts
	got := rowFromDoctor(d).Department
	require.LessOrEqual(t, utf8.RuneCountInString(got), 100)
	require.Equal(t, strings.Join(depts[:3], "、"), got) // 3×30 + 2 separators = 92; a 4th would make 123

	d.Departments = []string{strings.Repeat("甲", 150)}
	require.Equal(t, 100, utf8.RuneCountInString(rowFromDoctor(d).Department))
}
