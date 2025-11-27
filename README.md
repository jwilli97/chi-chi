# GoResyBot 🍽️

A Go-based Resy reservation bot that helps you automatically book restaurant reservations on the Resy platform. Features a web UI, REST API, and scheduled reservation support with Redis persistence.

## Features

- **Web Interface** - User-friendly HTML pages for selecting restaurants, logging in, and making reservations
- **REST API** - JSON endpoints for programmatic access
- **Scheduled Reservations** - Queue reservations to execute at a specific time (perfect for when bookings open)
- **Redis Persistence** - Scheduled reservations survive server restarts
- **Table Preferences** - Specify seating preferences (dining room, outdoor, bar, booth, etc.)
- **Automatic Cookie Refresh** - Built-in headless browser automation to bypass Imperva challenges
- **Admin Dashboard** - Monitor venue status and manage cookies

---

## Quick Start

### Docker (Recommended)

```bash
docker-compose up -d
```

This starts both the bot and a Redis instance. The app will be available at `http://localhost:8090`.

### Run Locally

Ensure you have Redis running locally, then:

```bash
go build -o resy_bot .
./resy_bot
```

---

## Configuration

Set these environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8090` | Server port |
| `REDIS_URL` | `localhost:6379` | Redis connection URL |
| `REDIS_PASSWORD` | *(empty)* | Redis password |
| `ADMIN_TOKEN` | *(empty)* | Token for admin endpoints |
| `RESY_API_KEY` | Provided default | Resy API key |
| `COOKIE_REFRESH_ENABLED` | `true` | Enable automatic cookie refresh via headless browser |
| `COOKIE_REFRESH_INTERVAL` | `6h` | How often to check/refresh cookies (e.g., `6h`, `30m`) |
| `COOKIE_SECRET_KEY` | Random | 64-char hex string for session persistence |
| `COOKIE_BLOCK_KEY` | Random | 64-char hex string for session persistence |

**Note:** If `COOKIE_SECRET_KEY` and `COOKIE_BLOCK_KEY` are not set, random keys are generated on startup (sessions won't survive restarts).

---

## User Workflow

### 1. Select a Restaurant

Navigate to `http://localhost:8090/` and click on a restaurant. Pre-configured venues:

- **Crevette** (Venue ID: 89607)
- **Farzi NewYork** (Venue ID: 89678)
- **Nonna Dora's Tribeca** (Venue ID: 92807)

### 2. Login with Your Resy Account

After selecting a restaurant, you're redirected to `/login`. Enter your Resy email and password.

### 3. Make a Reservation

Navigate to `/reserve` where you can:

- Set your **reservation time** (in NYC timezone)
- Set **party size**
- Choose **table preferences** (dining room, outdoor, bar, booth, etc.)
- Choose **immediate booking** or **schedule for later**

---

## API Reference

### Public Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check (returns Redis status) |
| `/api/search` | POST | Search for restaurants by name |
| `/api/select-venue` | POST | Select a restaurant (stores in session) |
| `/api/login` | POST | Authenticate with Resy credentials |
| `/api/reserve` | POST | Make a reservation |
| `/api/logs` | GET | View recent server logs |

### Admin Endpoints

Require `ADMIN_TOKEN` via `Authorization: Bearer <token>` header or `?token=<token>` query param.

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/admin/status` | GET | View venue cookie status & pending reservations |
| `/admin/cookies/import` | POST | Import browser cookies for a venue |
| `/admin/cookies/{venue_id}` | GET | Check cookie status for a venue |
| `/admin/cookies/{venue_id}` | DELETE | Delete cookies for a venue |

---

## API Examples

### Search for Restaurants

```bash
curl -X POST http://localhost:8090/api/search \
  -H "Content-Type: application/json" \
  -d '{"name": "Crevette", "limit": 5}'
```

### Make an Immediate Reservation

```bash
curl -X POST http://localhost:8090/api/reserve \
  -H "Content-Type: application/json" \
  -d '{
    "venue_id": 89607,
    "reservation_time": "2025-12-01T19:00",
    "party_size": 2,
    "table_preferences": ["dining", "indoor"],
    "is_immediate": true
  }'
```

### Schedule a Future Reservation

```bash
curl -X POST http://localhost:8090/api/reserve \
  -H "Content-Type: application/json" \
  -d '{
    "venue_id": 89607,
    "reservation_time": "2025-12-01T19:00",
    "party_size": 2,
    "table_preferences": ["dining"],
    "is_immediate": false,
    "request_time": "2025-11-28T09:00"
  }'
```

This schedules the bot to attempt the booking at 9:00 AM NYC time on Nov 28 — useful for when reservations open.

---

## Handling Imperva Challenges

Resy uses Imperva for bot protection. This bot includes **automatic cookie refresh** using a headless browser (Chromium) to solve JavaScript challenges.

### Automatic Cookie Refresh (Default)

When running with Docker, the bot automatically:

1. Fetches fresh Imperva cookies on startup for all known venues
2. Checks cookie validity every 6 hours (configurable via `COOKIE_REFRESH_INTERVAL`)
3. Refreshes cookies when they're expiring within 2 hours
4. Stores cookies in Redis with a 24-hour TTL

**No manual intervention required** in most cases. Check logs via `/api/logs` to monitor cookie refresh status.

### Disabling Automatic Refresh

If you prefer manual cookie management, disable auto-refresh:

```bash
COOKIE_REFRESH_ENABLED=false
```

### Manual Cookie Import (Fallback)

If automatic refresh fails (e.g., visual CAPTCHA), you can manually import cookies:

```bash
curl -X POST "http://localhost:8090/admin/cookies/import" \
  -H "Authorization: Bearer YOUR_ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "venue_id": 89607,
    "cookies": [
      {"name": "cookie_name", "value": "cookie_value", "domain": ".resy.com", "path": "/"}
    ],
    "user_agent": "Mozilla/5.0 ...",
    "ttl_hours": 24
  }'
```

### How to Export Cookies Manually

1. Log into Resy in your browser
2. Use a browser extension to export cookies (e.g., "EditThisCookie" or browser DevTools)
3. Import them via the admin endpoint above

---

## Table Preferences

When making a reservation, you can specify seating preferences:

| Value | Description |
|-------|-------------|
| `dining` | Dining room |
| `indoor` | Indoor seating |
| `outdoor` | Outdoor seating |
| `patio` | Patio seating |
| `bar` | Bar seating |
| `lounge` | Lounge seating |
| `booth` | Booth seating |

---

## Project Structure

```
resy_bot/
├── main.go              # Entry point, HTTP handlers, schedulers
├── api/
│   ├── api.go           # API interface & types
│   └── resy/
│       └── api.go       # Resy-specific implementation
├── app/                 # Application context
├── config/
│   └── config.go        # Configuration management
├── imperva/
│   └── cookie_fetcher.go # Headless browser cookie automation
├── store/
│   ├── redis.go         # Redis client
│   ├── cookies.go       # Cookie storage
│   └── reservations.go  # Scheduled reservation storage
├── static/
│   └── styles.css       # Stylesheets
├── index.html           # Home page
├── login.html           # Login page
├── reserve.html         # Reservation page
├── Dockerfile           # Container build (includes Chromium)
├── docker-compose.yml   # Full stack deployment
├── go.mod               # Go module definition
└── go.sum               # Dependency checksums
```

---

## Notes

- **All times are in NYC timezone** — Reservation and request times are parsed as Eastern Time and stored in UTC
- **Scheduled reservations persist in Redis** — They survive server restarts
- **Check logs** — Visit `/api/logs` or check console output for reservation status
- **Health endpoint** — Use `/health` to verify the server and Redis are running

---

## License

MIT

