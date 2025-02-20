CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS chunks (
    id SERIAL PRIMARY KEY,
    message_id TEXT,
    title TEXT,
    doc_url TEXT,
    content TEXT,
    embedding vector(1536) -- OpenAI embeddings are 1536-dimensional
);