-- Migration 0017: Clear synthetic 'Page %' chapter titles for pre-paginated books
UPDATE chapters
SET title = NULL
WHERE book_id IN (SELECT id FROM books WHERE layout = 'pre-paginated')
  AND title GLOB 'Page [0-9]*';
