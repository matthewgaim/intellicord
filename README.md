# Intellicord

Intellicord is a Discord bot that brings AI-assisted file analysis and conversation capabilities directly into your server. Upload documents, ask questions, and get intelligent, context-aware responses without ever leaving Discord and needing to share responses.

---

## Features

**AI-Powered File Upload & Analysis**  
Upload PDFs, Word documents, or spreadsheets — Intellicord processes them and provides contextual replies in chat.

**Thread-Based Responses**  
Intellicord replies in threads, maintaining the full conversation context, including previous messages and uploaded files.

**No-Context LLM Chat**  
Just need a quick answer? Use the `/ask` command to chat with the AI without uploading anything.

**Multi-Format Support**  
Supports a range of file types, including `.pdf`, `.docx`, and more.

---

## How It Works

1. **Invite Intellicord to Your Server**  
   - Sign in with your Discord account  
   - Go to the **Servers** tab and invite Intellicord  
   - Use the **Manage** button to control which channels have access

2. **Upload Files in Your Discord Channel**  
   Intellicord will process your file and confirm once it's ready to answer questions.

3. **Ask Questions in Threads**  
   Ask about specific parts of the file — Intellicord uses both the file and the thread history to respond intelligently.

4. **Use `/ask` for General Questions**  
   Quickly chat with the AI without requiring file uploads or context.

---

## Setup

### Prerequisites

- [Intellicord Website](https://github.com/matthewgaim/intellicord-website) (needed for initial setup)
- Docker and Docker Compose
- [A Discord bot](#discord-bot-setup-for-new-bot)

### Discord Bot Setup (For New Bot)

1. Go to the [Discord Developer Portal](https://discord.com/developers/applications)
2. Create a new application
3. Navigate to the "Bot" section
4. Create a bot and copy the token to your [environment variables](#2-environment-configuration)

### 1. Clone the Repository

```bash
git clone https://github.com/matthewgaim/intellicord.git
cd intellicord
```

### 2. Environment Configuration

Create a `.env` file in the root directory with the following variables:

```env
# Your discord bot's token
DISCORD_TOKEN=
DISCORD_CLIENT_ID=
DISCORD_CLIENT_SECRET=
DISCORD_REDIRECT_URI=

# You don't need all 4, you can pick OpenAI, Gemini, or your own LLM provider like Ollama or Groq (custom)
OPENAI_API_KEY=
GEMINI_API_KEY=

CUSTOM_API_KEY=
CUSTOM_BASE_URL=

# By default, these come with the Docker Compose
PARSER_API_URL=http://parser_api:8081
REDIS_URL=redis://redis:6379

POSTGRES_DB=
POSTGRES_USER=
POSTGRES_PASSWORD=
```

### 3. Running with Docker

If you're using a .env file:
```bash
docker compose --env-file .env up -d
```

If you have your environment variables in the system:
```bash
docker compose up -d
```