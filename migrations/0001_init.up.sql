CREATE TABLE IF NOT EXISTS medical_personnel (
    id SERIAL PRIMARY KEY,
    city TEXT NOT NULL,
    hospital TEXT NOT NULL,
    department TEXT,
    name TEXT NOT NULL,
    education TEXT
);
