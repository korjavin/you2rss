# Plan to Address Missing Features and Vulnerabilities

This plan outlines the tasks required to implement missing features and fix vulnerabilities identified during the code review. The tasks are prioritized based on their severity.

## High-Priority (Security Vulnerability)

### Task 1: Implement Authentication Signature Validation

**Goal:** Fix the critical security vulnerability in the authentication middleware by adding cryptographic signature validation for Telegram `initData`.

**File to modify:** `internal/middleware/auth.go`

**Steps:**
1.  Read the `TELEGRAM_BOT_TOKEN` from the environment.
2.  In the `AuthMiddleware`, replace the call to `initdata.Parse()` with `initdata.Validate()`.
3.  The `initdata.Validate()` function requires the `initData` string, the bot token, and an expiration duration. Use a reasonable expiration time (e.g., 1 hour) to prevent replay attacks.
4.  Handle any validation errors by returning a `401 Unauthorized` response.

## Medium-Priority (Resource Management & Abuse Prevention)

### Task 2: Implement API Rate Limiting

**Goal:** Protect the server from abuse and ensure stability by adding rate limiting to the API.

**Files to modify:** `cmd/server/main.go`

**Steps:**
1.  Choose a rate-limiting library for Go. A simple and effective option is `golang.org/x/time/rate`.
2.  Create a new rate-limiting middleware. This middleware will use a map to store a rate limiter for each unique visitor identifier (e.g., their IP address or Telegram User ID).
3.  Apply this middleware to all relevant API endpoints in `cmd/server/main.go`.
4.  Configure a sensible rate limit (e.g., 100 requests per minute).

### Task 3: Enforce Subscription Limits

**Goal:** Prevent users from overloading the system by enforcing a limit on the number of subscriptions per user.

**Files to modify:** `internal/handlers/subscriptions.go`, `internal/db/subscriptions.go`

**Steps:**
1.  Define a constant for the maximum number of subscriptions per user (e.g., 20).
2.  In `internal/db/subscriptions.go`, create a new function `CountSubscriptionsByUserID(userID int64) (int, error)` that returns the current number of subscriptions for a user.
3.  In the `PostSubscription` handler in `internal/handlers/subscriptions.go`, before adding a new subscription, call the new count function.
4.  If the user has reached the limit, return a `403 Forbidden` error with a user-friendly message.

### Task 4: Add Timeouts to Worker Processes

**Goal:** Prevent worker processes from hanging indefinitely by adding a timeout to the `yt-dlp` command execution.

**File to modify:** `internal/worker/handlers.go`

**Steps:**
1.  In the `HandleProcessVideoTask` function, when creating the `exec.Command`, use `exec.CommandContext` instead.
2.  Create a new context with a timeout (e.g., 15 minutes) using `context.WithTimeout()`.
3.  Pass this cancellable context to `exec.CommandContext`. If the command execution exceeds the timeout, it will be automatically killed.
4.  Log timeout errors for monitoring purposes.

## Low-Priority (Functionality/UX Improvement)

### Task 5: Improve YouTube URL Handling

**Goal:** Improve user experience by allowing various formats of YouTube channel URLs.

**Files to modify:** `internal/handlers/subscriptions.go`, `internal/worker/handlers.go`

**Steps:**
1.  The `PostSubscription` handler should be updated to accept more flexible URL formats (`/user/`, `/c/`, `/@`, etc.). A robust way to handle this is to use `yt-dlp` itself to resolve any valid YouTube channel URL to its canonical channel ID.
2.  Modify the `PostSubscription` handler: instead of using a regex, it should invoke `yt-dlp` with options like `--get-id` to extract the channel ID and `--get-title` to get the channel title from the provided URL.
3.  Store both the resolved `channel_id` and the fetched `channel_title` in the database. This eliminates the placeholder title issue.

### Task 6: Implement `serveWebApp` handler

**Goal:** Implement the `serveWebApp` handler to serve the `index.html` file.

**Files to modify:** `internal/handlers/handlers.go`

**Steps:**
1.  Create a new handler function `serveWebApp` that serves the `index.html` file.
2.  Update `cmd/server/main.go` to use this new handler for the root path (`/`).
3.  Remove the `GetRoot` handler.

### Task 7: Fetch Channel Title on Subscription

**Goal:** Fetch the channel title when a new subscription is added and store it in the database.

**Files to modify:** `internal/handlers/subscriptions.go`, `internal/db/subscriptions.go`

**Steps:**
1.  When adding a new subscription in `PostSubscription`, use `yt-dlp` to fetch the channel title.
2.  Update the `AddSubscription` function in `internal/db/subscriptions.go` to accept the channel title as an argument.
3.  Store the fetched title in the `subscriptions` table.
