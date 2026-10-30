CREATE TABLE users (id INTEGER, name TEXT, active BOOLEAN);
INSERT INTO users (name, id) VALUES ('Amit', 42);
INSERT INTO users (id, name, active) VALUES (9223372036854775807, 'O''Brien; नमस्ते', FALSE);
INSERT INTO users (id, active) VALUES (NULL, TRUE);
SELECT name, id, name FROM users;
SELECT * FROM users;
