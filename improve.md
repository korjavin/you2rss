# Project Improvements

This document outlines potential improvements for the YT-Podcaster project, focusing on security, robustness, and documentation.

## Security Vulnerabilities

### 1. Server-Side Request Forgery (SSRF) in Subscription Handler

-   **File**: `internal/handlers/subscriptions.go`
-   **Function**: `PostSubscription`
-   **Issue**: The `channelURL` parameter, which is user-controlled, is passed directly to the `yt-dlp` command-line tool without proper validation. `yt-dlp` is a powerful tool that can access various URLs, including local file paths (`file://...`) and internal network addresses.
-   **Risk**: A malicious user could craft a URL that points to an internal service on the server's network or to a local file on the server. This could lead to information disclosure or allow an attacker to probe the internal network.
-   **Recommendation**: Before passing the URL to `yt-dlp`, implement strict validation to ensure it is a valid and expected YouTube channel URL. A regular expression can be used for this purpose. For example, it should match patterns like `https://www.youtube.com/channel/...` or `https://www.youtube.com/@...`.

## Robustness Issues

### 1. Missing Timeout in Channel Check Worker

-   **File**: `internal/worker/handlers.go`
-   **Function**: `HandleCheckChannelTask`
-   **Issue**: The `exec.Command` call to `yt-dlp` for fetching a channel's video list does not use a context with a timeout.
-   **Risk**: If `yt-dlp` hangs for any reason (e.g., a very large channel, network issues), the worker process could be stuck indefinitely, consuming resources and preventing other tasks from being processed.
-   **Recommendation**: Use `exec.CommandContext` with a reasonable timeout (e.g., 1-2 minutes) for this operation, similar to how it's done in `HandleProcessVideoTask`. This will ensure that the task will eventually fail if it takes too long, freeing up the worker for other jobs.

## Documentation Improvements

-   The project documentation has been updated to address several inconsistencies and to make it more comprehensive.
-   The `readme.md` file has been cleaned up.
-   A detailed `DEPLOYMENT.md` has been created.
-   A `LICENSE` file has been added.
