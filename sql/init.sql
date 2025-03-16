-- Define the ChatRole enum
CREATE TYPE chat_role AS ENUM ('user', 'assistant', 'system');

-- Create the senders table
CREATE TABLE senders (
    id BIGSERIAL PRIMARY KEY,
    name TEXT UNIQUE NOT NULL
);

-- Create the messages table
CREATE TABLE messages (
    id BIGSERIAL PRIMARY KEY,
    role chat_role NOT NULL,
    text TEXT NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT now(),
    sender_id BIGINT NOT NULL,
    receiver_id BIGINT NOT NULL,
    CONSTRAINT fk_sender FOREIGN KEY (sender_id) REFERENCES senders(id) ON DELETE CASCADE,
    CONSTRAINT fk_receiver FOREIGN KEY (receiver_id) REFERENCES senders(id) ON DELETE CASCADE
);

-- Indexes for fast lookup
CREATE INDEX idx_messages_sender_id ON messages(sender_id);
CREATE INDEX idx_messages_receiver_id ON messages(receiver_id);
CREATE INDEX idx_messages_timestamp ON messages(timestamp);
