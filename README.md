# Intellicord

Intellicord is a powerful Discord bot that brings AI-assisted file analysis and conversation capabilities directly into your server. Upload documents, ask questions, and get intelligent, context-aware responses without ever leaving Discord.

---

## 🚀 Features

**AI-Powered File Upload & Analysis**  
Upload PDFs, Word documents, or spreadsheets — Intellicord processes them and provides contextual replies in chat.

**Thread-Based Responses**  
Intellicord replies in threads, maintaining the full conversation context, including previous messages and uploaded files.

**No-Context LLM Chat**  
Just need a quick answer? Use the `/ask` command to chat with the AI without uploading anything.

**Multi-Format Support**  
Supports a range of file types, including `.pdf`, `.docx`, and more.

---

## 🛠 How It Works

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

## ⚙️ Local Setup

### 1. Clone the Repo

```shell
git clone https://github.com/matthewgaim/intellicord.git
cd intellicord
```

### 2. Create a `.env` file with the following keys:

```
# Discord Bot Token
# Find your bot's token here: https://discord.com/developers/applications
DISCORD_TOKEN=

# OpenAI API Key
OPENAI_API_KEY=

# PostgreSQL connection string
# Format: postgres://{user}:{password}@{host}:{port}/{database}
DATABASE_URL=

# URL for the file parser API
# Repo: https://github.com/matthewgaim/intellicord_parser_api
PARSER_API_URL=

# URL of the Intellicord frontend (used for redirects)
INTELLICORD_FRONTEND_URL=

# Stripe API Keys
STRIPE_SECRET_KEY=
STRIPE_WEBHOOK_SECRET=

# Redis connection URL
# Default: redis://localhost:6379
REDIS_URL=

```

### 3. Build & Run Dockerfile

```docker
docker build . -t intellicord:1.0
docker run --env-file .env -d intellicord:1.0
```