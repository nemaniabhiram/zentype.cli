# ZenType Server

PostgreSQL backend API server for ZenType global leaderboard.

## Setup

1. Set environment variables:
```bash
DATABASE_URL=postgres://user:pass@host:port/dbname
GITHUB_CLIENT_ID=your_github_client_id
GITHUB_CLIENT_SECRET=your_github_client_secret
```

2. Run the server:
```bash
go run main.go
```

## Environment Variables

- `DATABASE_URL` - PostgreSQL connection string (required)
- `GITHUB_CLIENT_ID` - GitHub OAuth App Client ID (required)
- `GITHUB_CLIENT_SECRET` - GitHub OAuth App Client Secret (required)
- `PORT` - Server port (default: 8080)
- `GITHUB_REDIRECT_URL` - OAuth callback URL (optional)

## GitHub OAuth Setup

1. Go to https://github.com/settings/applications/new
2. Create OAuth App with callback URL: `http://localhost:8080/api/auth/github/callback`
3. Save Client ID and Secret

## API Endpoints

- `GET /api/health` - Health check
- `GET /api/auth/github` - Get OAuth URL
- `POST /api/scores` - Submit score (auth required)
- `GET /api/leaderboard` - Get top 10 rankings
- `GET /api/user/rank` - Get user rank (auth required)

The server automatically creates database tables on startup.