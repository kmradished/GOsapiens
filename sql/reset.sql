-- Только для отдельной учебной БД: удаляет её бизнес-данные.
BEGIN;
TRUNCATE matches, pairs, profiles;

-- Порядок совпадает с фикстурами ЛР3. Для u1 доступны u6, u2, u5, u4.
INSERT INTO profiles (id, name, active, movies, position) VALUES
    ('u1', 'u1', true,  ARRAY['Dune'], 1),
    ('u6', 'u6', true,  ARRAY['Dune'], 2),
    ('u2', 'u2', true,  ARRAY['Dune'], 3),
    ('u3', 'u3', false, ARRAY['Dune'], 4),
    ('u5', 'u5', true,  ARRAY['Dune'], 5),
    ('u4', 'u4', true,  ARRAY['Dune'], 6),
    ('u7', 'u7', true,  ARRAY['Dune'], 7);

INSERT INTO pairs (left_id, right_id, left_liked, right_liked, right_blocked) VALUES
    ('u1', 'u7', false, false, true),
    ('u2', 'u5', true,  false, false),
    ('u4', 'u6', true,  true,  false);

INSERT INTO matches (left_id, right_id) VALUES ('u4', 'u6');
COMMIT;
