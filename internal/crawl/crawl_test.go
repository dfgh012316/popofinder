package crawl

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDoctorIndex(t *testing.T) {
	x := newDoctorIndex()
	x.add("b", "D1", "甲")
	x.add("a", "D1", "甲")
	x.add("b", "D2", "乙")

	require.Equal(t, []string{"b", "a"}, x.order)
	require.Equal(t, "D1", x.first["b"])
	require.Equal(t, []string{"甲", "乙"}, x.depts["b"])
	require.Equal(t, []string{"甲"}, x.depts["a"])
}
