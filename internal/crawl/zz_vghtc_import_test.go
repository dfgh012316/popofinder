//go:build vghtcimport

// Temporary, not committed: crawls 中榮 through a page cache and writes the psql
// import script (wiki repos/popofinder.md, offline crawl then import).
package crawl

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// cacheTransport answers from dir when a page was fetched before; otherwise it
// fetches it (1 second apart) and stores 200 responses and robots.txt.
type cacheTransport struct {
	dir  string
	last time.Time
}

func (c *cacheTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	sum := sha256.Sum256([]byte(r.URL.String()))
	path := filepath.Join(c.dir, hex.EncodeToString(sum[:]))
	if b, err := os.ReadFile(path); err == nil {
		i := bytes.IndexByte(b, '\n')
		status, _ := strconv.Atoi(string(b[:i]))
		return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(b[i+1:])), Request: r}, nil
	}
	if wait := time.Second - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	resp, err := http.DefaultTransport.RoundTrip(r)
	c.last = time.Now()
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusOK || r.URL.Path == "/robots.txt" {
		if err := os.WriteFile(path, append([]byte(strconv.Itoa(resp.StatusCode)+"\n"), body...), 0o644); err != nil {
			return nil, err
		}
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	return resp, nil
}

func TestVghtcImport(t *testing.T) {
	dir, out, date := os.Getenv("VGHTC_CACHE"), os.Getenv("VGHTC_SQL"), os.Getenv("VGHTC_DATE")
	require.NotEmpty(t, dir)
	require.NotEmpty(t, out)
	require.Regexp(t, `^\d{4}-\d{2}-\d{2}$`, date)
	require.NoError(t, os.MkdirAll(dir, 0o755))

	f := NewFetcher(0)
	f.client.Transport = &cacheTransport{dir: dir}
	doctors, err := NewVghtc("https://register.vghtc.gov.tw/register", f).Crawl(context.Background())
	require.NoError(t, err)
	plan := BuildSyncPlan(doctors, nil, nil)
	t.Logf("crawled=%d insert=%d skipped_no_degree=%d", len(doctors), len(plan.Inserts), plan.SkippedNoDegree)

	var sql strings.Builder
	sql.WriteString("DO $$ BEGIN IF EXISTS (SELECT 1 FROM medical_personnel WHERE source_key LIKE 'vghtc:%') THEN RAISE EXCEPTION 'vghtc rows already exist'; END IF; END $$;\n")
	sql.WriteString("SELECT set_config('popo.change_reason', 'crawl:vghtc', true);\n")
	sql.WriteString("COPY medical_personnel (city, hospital, department, name, education, source, source_url, source_key, source_checked_at) FROM STDIN WITH (FORMAT csv);\n")
	w := csv.NewWriter(&sql)
	for _, r := range plan.Inserts {
		// An empty department is written unquoted, which COPY reads as NULL (nullIfEmpty).
		require.NoError(t, w.Write([]string{r.City, r.Hospital, r.Department, r.Name, r.Education, "hospital_directory", r.URL, r.Key, date}))
	}
	w.Flush()
	require.NoError(t, w.Error())
	sql.WriteString("\\.\n")
	sql.WriteString("SELECT count(*) AS vghtc_rows FROM medical_personnel WHERE source_key LIKE 'vghtc:%';\n")
	sql.WriteString("SELECT count(*) AS vghtc_history FROM medical_personnel_history WHERE reason = 'crawl:vghtc';\n")
	require.NoError(t, os.WriteFile(out, []byte(sql.String()), 0o644))
}
