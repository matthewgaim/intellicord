CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS chunks (
    id SERIAL PRIMARY KEY,
    message_id TEXT,
    title TEXT,
    doc_url TEXT,
    content TEXT,
    embedding vector(1536) -- OpenAI embeddings are 1536-dimensional
);

CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    discord_id TEXT UNIQUE NOT NULL,
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE joined_servers (
    id SERIAL PRIMARY KEY,
    discord_server_id TEXT UNIQUE NOT NULL,
    owner_id TEXT NOT NULL,
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (owner_id) REFERENCES users(discord_id) ON DELETE CASCADE
);