package crawl

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// readFixture returns testdata/<source>/<name>.
func readFixture(t *testing.T, source, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", source, name))
	require.NoError(t, err)
	return string(b)
}

// serveFixture writes testdata/<source>/<name>, or a 404 when there is no such fixture.
func serveFixture(w http.ResponseWriter, source, name string) {
	b, err := os.ReadFile(filepath.Join("testdata", source, name))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Write(b)
}
