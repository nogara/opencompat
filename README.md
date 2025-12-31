# OpenCompat

> **NOTICE**: This is an independent open-source project for **personal,
> non-commercial use only**. It is NOT affiliated with, endorsed by, or
> sponsored by OpenAI or any other company. Users are responsible for
> compliance with all applicable terms of service. See [Disclaimer](#disclaimer).

A personal API compatibility layer that provides an OpenAI-compatible interface
for your existing subscription. For individual, non-commercial use only.

## Overview

OpenCompat provides a local API server with endpoints compatible with standard
AI client libraries (`/v1/chat/completions`, `/v1/models`). It allows you to
use your existing subscription through tools that support the OpenAI API format.

### Features

- OpenAI-compatible API endpoints
- OAuth authentication with PKCE
- Automatic token refresh
- Streaming and non-streaming responses
- Tool/function calling support
- Image input support

## Installation

### From Source

```bash
git clone https://github.com/edgard/opencompat.git
cd opencompat
make build
```

### Pre-built Binaries

Download from the [Releases](https://github.com/edgard/opencompat/releases) page.

## Quick Start

```bash
# 1. Login with your account
opencompat login

# 2. Start the server
opencompat serve

# 3. Use with any OpenAI-compatible client
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-5",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

## Usage

### Commands

```bash
opencompat login    # Authenticate (opens browser)
opencompat logout   # Remove stored credentials
opencompat info     # Show authentication status
opencompat serve    # Start the API server (default)
opencompat version  # Show version information
opencompat help     # Show help message
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `OPENCOMPAT_HOST` | `127.0.0.1` | Server bind address |
| `OPENCOMPAT_PORT` | `8080` | Server listen port |
| `OPENCOMPAT_VERBOSE` | `false` | Enable verbose logging |
| `OPENCOMPAT_REASONING_EFFORT` | `medium` | Reasoning effort level |
| `OPENCOMPAT_REASONING_SUMMARY` | `auto` | Reasoning summary mode |
| `OPENCOMPAT_TEXT_VERBOSITY` | `medium` | Text verbosity level |
| `OPENCOMPAT_INSTRUCTIONS_REFRESH` | `1440` | Refresh interval (minutes) |

### API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/chat/completions` | POST | Chat completions |
| `/v1/models` | GET | List available models |
| `/health` | GET | Health check |

## Client Examples

### Python

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://127.0.0.1:8080/v1",
    api_key="not-needed"
)

response = client.chat.completions.create(
    model="gpt-5",
    messages=[{"role": "user", "content": "Hello!"}]
)
print(response.choices[0].message.content)
```

### Node.js

```javascript
import OpenAI from 'openai';

const client = new OpenAI({
  baseURL: 'http://127.0.0.1:8080/v1',
  apiKey: 'not-needed',
});

const response = await client.chat.completions.create({
  model: 'gpt-5',
  messages: [{ role: 'user', content: 'Hello!' }],
});

console.log(response.choices[0].message.content);
```

## Development

```bash
make check      # Run checks (fmt, vet, lint)
make test       # Run tests
make dev        # Run with go run
make build-all  # Build for all platforms
```

## Requirements

- Go 1.21+ (for building from source)
- A compatible subscription
- A web browser for OAuth login

## Technical Notes

This software:
- Uses standard OAuth PKCE authentication
- Translates between API formats
- Uses your own credentials and subscription
- Fetches instruction files from open-source repositories (Apache 2.0)

## License

MIT License - see [LICENSE](LICENSE) file.

---

## Disclaimer

### No Affiliation

This software is NOT affiliated with, endorsed by, or sponsored by OpenAI
or any other company. This is an independent open-source project.

### Personal, Non-Commercial Use Only

This software is for personal, individual, non-commercial use only.

Not intended for:
- Commercial services or resale
- Multi-user or shared deployments
- High-volume automated processing

### Your Responsibility

You are responsible for:
- Compliance with all applicable terms of service
- How you use this software
- Any consequences of your use

### Assumption of Risk

By using this software, you acknowledge:
- Your use may be subject to third-party terms of service
- You assume all risk for any consequences
- The author is not liable for any damages

### No Warranty

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND. See LICENSE
for full terms.
