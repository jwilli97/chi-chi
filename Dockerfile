# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o resy_bot .

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install CA certificates, timezone data, and Chromium for headless browser automation
RUN apk --no-cache add \
    ca-certificates \
    tzdata \
    chromium \
    chromium-chromedriver \
    nss \
    freetype \
    harfbuzz \
    ttf-freefont \
    font-noto-emoji

# Set Chrome path for chromedp
ENV CHROME_PATH=/usr/bin/chromium-browser
ENV CHROME_BIN=/usr/bin/chromium-browser

# Copy the binary from builder
COPY --from=builder /app/resy_bot .

# Copy HTML templates and static files
COPY --from=builder /app/*.html ./
COPY --from=builder /app/static ./static

# Create a non-root user and give it access to chrome sandbox
RUN adduser -D -g '' appuser && \
    mkdir -p /home/appuser/.cache/chromium && \
    chown -R appuser:appuser /home/appuser
USER appuser

# Expose port
EXPOSE 8090

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8090/health || exit 1

# Run the binary
ENTRYPOINT ["./resy_bot"]



