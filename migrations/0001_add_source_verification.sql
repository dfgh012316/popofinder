ALTER TABLE medical_personnel
    ADD COLUMN source TEXT
        CHECK (source IN ('blog', 'public_report')),
    ADD COLUMN verification_status TEXT NOT NULL DEFAULT 'unverified'
        CHECK (verification_status IN ('unverified', 'self_reported', 'verified')),
    ADD COLUMN source_url TEXT;
