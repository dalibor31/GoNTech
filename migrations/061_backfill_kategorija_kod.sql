UPDATE kategorije
SET kod = UPPER(SUBSTR(REPLACE(naziv, ' ', ''), 1, 4))
WHERE (kod IS NULL OR kod = '') AND naziv IS NOT NULL AND naziv != '';
