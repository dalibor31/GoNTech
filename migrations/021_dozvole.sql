CREATE TABLE IF NOT EXISTS dozvole (
    uloga       TEXT    NOT NULL,
    akcija      TEXT    NOT NULL,
    dozvoljeno  INTEGER NOT NULL DEFAULT 1,
    PRIMARY KEY (uloga, akcija)
);
