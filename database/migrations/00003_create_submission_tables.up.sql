CREATE TABLE IF NOT EXISTS submission (
    id INTEGER PRIMARY KEY,
    `version` INTEGER DEFAULT 1,
    survey_id INTEGER NOT NULL REFERENCES survey(id)
        ON UPDATE CASCADE
        ON DELETE RESTRICT,
    `time` DATETIME NOT NULL,
    ip VARCHAR(50) NOT NULL CHECK (LENGTH(ip) > 0)
);

CREATE TABLE IF NOT EXISTS submission_field (
    id INTEGER PRIMARY KEY,
    `version` INTEGER DEFAULT 1,
    submission_id INTEGER NOT NULL REFERENCES submission(id)
        ON UPDATE CASCADE
        ON DELETE RESTRICT,
    field_id INTEGER NOT NULL REFERENCES survey_field(id)
        ON UPDATE CASCADE
        ON DELETE RESTRICT,
    `value` TEXT NOT NULL ON CONFLICT REPLACE DEFAULT ''
);
