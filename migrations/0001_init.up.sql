CREATE TABLE IF NOT EXISTS medical_personnel (
    id SERIAL PRIMARY KEY,
    city VARCHAR(50) NOT NULL,
    hospital VARCHAR(200) NOT NULL,
    department VARCHAR(100),
    name VARCHAR(100) NOT NULL,
    education VARCHAR(500),
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS ix_medical_personnel_city ON medical_personnel (city);
