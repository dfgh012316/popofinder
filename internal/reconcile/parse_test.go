package reconcile

import (
	"os"
	"reflect"
	"testing"
)

func TestParsePost(t *testing.T) {
	data, err := os.ReadFile("testdata/sample_post.html")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	got := ParsePost(string(data))

	want := []Record{
		{City: "台中", Hospital: "大里仁愛醫院", Department: "", Name: "陳柏誠", Education: "波蘭"},
		{City: "台中", Hospital: "中山附醫", Department: "內科", Name: "薛崇亨", Education: "波蘭"},
		{City: "台中", Hospital: "中山醫", Department: "婦產科", Name: "黃允瑤", Education: "Poznan"},
		{City: "台北", Hospital: "某某診所", Department: "家醫科", Name: "林小明", Education: "波蘭&捷克"},
	}

	if len(got) != len(want) {
		t.Fatalf("got %d records, want %d: %+v", len(got), len(want), got)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("records mismatch:\n got: %+v\nwant: %+v", got, want)
	}
}
