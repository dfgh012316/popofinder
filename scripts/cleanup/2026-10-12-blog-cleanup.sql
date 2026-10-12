-- One-off cleanup of blog-era rows (docs/open-decisions.md OD-13). Run once, in one transaction:
--   psql -1 -v ON_ERROR_STOP=1 -f scripts/cleanup/2026-10-12-blog-cleanup.sql
-- Rows written by the hospital-directory crawler (source = 'hospital_directory') are never touched.
-- Every change goes through the history trigger with its own popo.change_reason.

-- Text compared without punctuation or spaces: the first import dropped "/" and similar
-- characters that the blog keeps, so the same entry can differ only in those.
CREATE OR REPLACE FUNCTION pg_temp.cleanup_norm(t text) RETURNS text
LANGUAGE sql IMMUTABLE AS $$ SELECT regexp_replace(coalesce(t, ''), '[[:punct:][:space:]、，。／/．.]', '', 'g') $$;

-- 1. Non-practitioners: the hospital field is a status (在學, 待實習, 國考尚未通過 …), not a workplace.
SET LOCAL popo.change_reason = 'cleanup:non-practitioner';
DELETE FROM medical_personnel
WHERE source IS DISTINCT FROM 'hospital_directory'
  AND hospital ~ '(就學|在學|實習|畢業|待業|國考|學生)';

-- 2. Unclassified rows that repeat a blog row except for punctuation and spaces; the blog row
--    (with its source link) stays.
SET LOCAL popo.change_reason = 'cleanup:unclassified-duplicate';
DELETE FROM medical_personnel n
WHERE n.source IS NULL
  AND EXISTS (
    SELECT 1 FROM medical_personnel b
    WHERE b.source = 'blog' AND b.name = n.name AND b.city = n.city
      AND pg_temp.cleanup_norm(b.hospital) = pg_temp.cleanup_norm(n.hospital)
      AND pg_temp.cleanup_norm(b.department) = pg_temp.cleanup_norm(n.department)
      AND pg_temp.cleanup_norm(b.education) = pg_temp.cleanup_norm(n.education));

-- 3. Blog rows with the same name, city and hospital are one doctor listed several times.
--    Keep the smallest id; departments are joined with 、 and educations with ；, both in
--    first-seen order, dropping an education whose text is contained in another one.
SET LOCAL popo.change_reason = 'cleanup:blog-merge';
CREATE TEMP TABLE cleanup_blog_group ON COMMIT DROP AS
SELECT * FROM (
  SELECT id,
         nullif(btrim(department), '') AS dept,
         nullif(btrim(education), '') AS edu,
         pg_temp.cleanup_norm(education) AS edu_norm,
         min(id) OVER w AS keep_id,
         count(*) OVER w AS group_size
  FROM medical_personnel
  WHERE source = 'blog'
  WINDOW w AS (PARTITION BY name, city, pg_temp.cleanup_norm(hospital))
) g
WHERE group_size > 1;

UPDATE medical_personnel m
SET department = (
      SELECT string_agg(dept, '、' ORDER BY first_id)
      FROM (SELECT dept, min(id) AS first_id FROM cleanup_blog_group g
            WHERE g.keep_id = m.id AND dept IS NOT NULL GROUP BY dept) d),
    education = (
      SELECT string_agg(edu, '；' ORDER BY first_id)
      FROM (SELECT DISTINCT ON (edu_norm) edu, id AS first_id FROM cleanup_blog_group g
            WHERE g.keep_id = m.id AND g.edu_norm <> ''
              AND NOT EXISTS (SELECT 1 FROM cleanup_blog_group o
                              WHERE o.keep_id = g.keep_id AND o.edu_norm <> g.edu_norm
                                AND strpos(o.edu_norm, g.edu_norm) > 0)
            ORDER BY edu_norm, id) e)
WHERE m.id IN (SELECT keep_id FROM cleanup_blog_group);

DELETE FROM medical_personnel
WHERE id IN (SELECT id FROM cleanup_blog_group WHERE id <> keep_id);
