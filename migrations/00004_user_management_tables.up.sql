CREATE TABLE IF NOT EXISTS user (
    username VARCHAR(255) PRIMARY KEY,
    password_hash TEXT NOT NULL
);

INSERT INTO user (username, password_hash) VALUES ('mbolis', '$2y$05$aemXe/8YSs7DLivA/rkPoeXUsQbDOXBbpRLlv5A1FzHPkXNibUj1S');

CREATE TABLE IF NOT EXISTS token (
    username VARCHAR(255) NOT NULL REFERENCES user(username)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    token_id VARCHAR(255) NOT NULL,
    refresh_token_id VARCHAR(255) NOT NULL,
    expiration TIMESTAMP NOT NULL,
    PRIMARY KEY (username, token_id, refresh_token_id)
);