CREATE TABLE users (
    id INTEGER,
    name TEXT,
    active BOOLEAN
);
INSERT INTO users (id, name, active) VALUES (42, 'Amit', TRUE);
INSERT INTO users (id, name, active) VALUES (43, 'O''Brien; नमस्ते', FALSE);
SELECT * FROM users;
SELECT id, name FROM users;
