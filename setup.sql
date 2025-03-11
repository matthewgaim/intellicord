CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS chunks (
    id SERIAL PRIMARY KEY,
    message_id TEXT,
    discord_server_id TEXT NOT NULL,
    title TEXT,
    doc_url TEXT,
    content TEXT,
    embedding vector(1536) -- OpenAI embeddings are 1536-dimensional
);

CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    discord_id TEXT UNIQUE NOT NULL,
    price_id VARCHAR(255) NOT NULL DEFAULT '',
    plan VARCHAR(255) NOT NULL DEFAULT 'free',
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS joined_servers (
    id SERIAL PRIMARY KEY,
    discord_server_id TEXT UNIQUE NOT NULL,
    owner_id TEXT NOT NULL,
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    allowed_channels TEXT[] DEFAULT '{}',
    FOREIGN KEY (owner_id) REFERENCES users(discord_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS uploaded_files (
    id SERIAL PRIMARY KEY,
    discord_server_id TEXT NOT NULL,
    channel_id TEXT NOT NULL,
    uploader_id TEXT NOT NULL,
    title TEXT NOT NULL,
    file_url TEXT NOT NULL,
    file_size BIGINT NOT NULL,
    uploaded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (discord_server_id) REFERENCES joined_servers(discord_server_id)
);

CREATE TABLE IF NOT EXISTS message_logs (
    id SERIAL PRIMARY KEY,
    message_id TEXT UNIQUE NOT NULL,
    discord_server_id TEXT NOT NULL,
    channel_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (discord_server_id) REFERENCES joined_servers(discord_server_id)
);
