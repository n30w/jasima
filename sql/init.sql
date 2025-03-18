-- Define the ChatRole enum
CREATE TYPE chat_role AS ENUM ('user', 'assistant', 'system');

-- Create the agents table
CREATE TABLE agents (
    id BIGSERIAL PRIMARY KEY,
    name TEXT UNIQUE NOT NULL
);

INSERT INTO senders (name) VALUES ('toki'), ('pona');

-- Create the messages table
CREATE TABLE messages (
    id BIGSERIAL PRIMARY KEY,
    role CHAT_ROLE NOT NULL,
    text TEXT NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT now(),
    inserted_by BIGINT NOT NULL,
    sender_id BIGINT NOT NULL,
    receiver_id BIGINT NOT NULL,
    CONSTRAINT fk_sender FOREIGN KEY (sender_id) REFERENCES agents (
        id
    ) ON DELETE CASCADE,
    CONSTRAINT fk_receiver FOREIGN KEY (receiver_id) REFERENCES agents (
        id
    ) ON DELETE CASCADE
);

-- Indexes for fast lookup
CREATE INDEX idx_messages_sender_id ON messages (sender_id);

CREATE INDEX idx_messages_receiver_id ON messages (receiver_id);

CREATE INDEX idx_messages_inserted_by ON messages (inserted_by);

CREATE INDEX idx_messages_timestamp ON messages (timestamp);
