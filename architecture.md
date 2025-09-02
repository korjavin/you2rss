# System Architecture Overview

The YT-Podcaster service is designed as a distributed system composed of several distinct, cooperating components. This architectural approach deliberately decouples the user-facing API from the resource-intensive processing workloads. This separation is the cornerstone of the system's design, ensuring that the user interface remains responsive and available, even when the background processing system is under heavy load. It also allows for independent scaling of components; for instance, if video processing becomes a bottleneck, more worker instances can be deployed without altering the web server fleet.

The core components and their interactions are as follows:

- **User & Telegram Mini App**: The user interacts with the service through a web interface rendered inside Telegram. This Mini App is built with HTML and htmx, providing a dynamic experience without client-side JavaScript complexity.
- **Go Web Server**: A lightweight Go application responsible for serving the frontend, handling user authentication via Telegram initData, and managing API requests for subscriptions. It acts as an Asynq client, enqueuing tasks for background processing but not executing them directly.
- **Asynq Worker**: A separate Go process that acts as an Asynq server. It continuously polls Redis for new tasks, such as checking a channel for new videos or processing a specific video. It contains the logic to execute external tools like yt-dlp.
- **Redis**: Serves as the message broker for the Asynq task queue. It provides a durable and high-performance backbone for communication between the web server and the workers.
- **PostgreSQL Database**: The system's source of truth, storing all persistent data, including user profiles, channel subscriptions, and episode metadata.
- **External Services**: The system interacts with the YouTube platform via yt-dlp to fetch video information and content.

The following diagram illustrates the flow of data and requests through the system:

```mermaid
graph TD
    subgraph User
        A[Telegram Client] --> B{Mini App (htmx)}
        H[Podcast Player]
    end

    subgraph "YT-Podcaster Service"
        B -- HTTPS Request --> C[Go Web Server / API]
        C -- Enqueue Task --> D[Redis (Asynq)]
        E[Asynq Worker] -- Dequeue Task --> D
        C <--> F[PostgreSQL DB]
        E <--> F
        C -- Serves RSS/Audio --> H
    end

    subgraph External
        E -- Fetches Video --> G[YouTube]
    end

    style C fill:#f9f,stroke:#333,stroke-width:2px
    style E fill:#ccf,stroke:#333,stroke-width:2px
```

This decoupled architecture ensures resilience. A failure in a worker process while downloading a large video will not impact the web server's ability to serve other users. Asynq's built-in retry mechanisms can automatically re-attempt the failed job, contributing to the overall robustness of the service.

## Data Model and Database Schema

The database schema is designed to be normalized and efficient, providing the foundation for all application logic. It consists of three primary tables: `users`, `subscriptions`, and `episodes`.

### `users`

This table stores essential information for each authenticated Telegram user. The Telegram User ID serves as the primary key, as it is a stable and unique identifier provided by the authentication mechanism. A crucial element is the `rss_uuid`, a randomly generated UUID that forms part of the user's unique RSS feed URL. This prevents enumeration of user feeds, as the UUID is non-sequential and difficult to guess.

```sql
CREATE TABLE users (
    id BIGINT PRIMARY KEY, -- Telegram User ID
    telegram_username VARCHAR(255),
    rss_uuid UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### `subscriptions`

This table represents the many-to-many relationship between users and YouTube channels. It links a `user_id` to a `youtube_channel_id`. A `UNIQUE` constraint on (`user_id`, `youtube_channel_id`) prevents a user from subscribing to the same channel multiple times. The `youtube_channel_title` is stored directly in this table as a performance optimization; this denormalization avoids the need to repeatedly fetch the channel's metadata just to display its name in the user interface.

```sql
CREATE TABLE subscriptions (
    id SERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    youtube_channel_id VARCHAR(255) NOT NULL,
    youtube_channel_title VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, youtube_channel_id) -- Prevent duplicate subscriptions
);
```

### `episodes`

This table stores the metadata for every YouTube video that has been processed or is queued for processing. It is the most detailed table and directly maps to the information required to generate an `<item>` in the RSS feed. The `youtube_video_id` is unique to prevent duplicate processing. The `audio_uuid` serves the same security purpose as the `rss_uuid` in the `users` table, obfuscating the direct path to the audio file. The `status` field is critical for state management within the asynchronous workflow, allowing the system to track whether an episode is `PENDING`, `PROCESSING`, `COMPLETED`, or has `FAILED`. This is vital for debugging, retries, and ensuring that incomplete episodes are not included in the final RSS feed. The schema also includes fields required by the RSS 2.0 and iTunes podcast specifications, such as `audio_size_bytes` and `duration_seconds`, which populate the `<enclosure>` and `<itunes:duration>` tags respectively.

```sql
CREATE TABLE episodes (
    id SERIAL PRIMARY KEY,
    subscription_id INTEGER NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
    youtube_video_id VARCHAR(255) NOT NULL UNIQUE,
    title TEXT,
    description TEXT,
    published_at TIMESTAMPTZ,
    audio_uuid UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    audio_path VARCHAR(1024),
    audio_size_bytes BIGINT,
    duration_seconds INTEGER,
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING', -- PENDING, PROCESSING, COMPLETED, FAILED
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

## Core Components Deep Dive

### Telegram Mini App Authentication

The authentication process is designed to be secure and frictionless, leveraging the native capabilities of the Telegram platform.

1.  **Client-Side Data Retrieval**: The htmx frontend, running within the Mini App context, uses the official Telegram SDK to retrieve the `initData` string.
2.  **Secure Transmission**: For every authenticated API request, the client includes this raw `initData` string in the `Authorization` HTTP header, prefixed with the scheme `tma`. This is a standard practice for token-based authentication and is explicitly recommended in the Telegram Mini App documentation. Example: `Authorization: tma <raw_initData_string>`.
3.  **Server-Side Middleware**: A Go middleware function is applied to all protected API routes. This middleware is responsible for intercepting and validating the `Authorization` header.
4.  **Validation**: The middleware uses the `initdata.Validate()` function from the `telegram-mini-apps/init-data-golang` library. This function performs a cryptographic verification of the `initData` signature against the bot's secret token and checks that the data has not expired, mitigating replay attacks.
5.  **User Provisioning**: Upon successful validation, the middleware parses the user information from `initData`. It then performs an "upsert" operation on the `users` table in the database: if a user with the given Telegram ID exists, their information is updated; otherwise, a new user record is created.
6.  **Context Injection**: The authenticated user's ID is injected into the request's context, making it available to all subsequent HTTP handlers in the chain without needing to re-validate or re-query the user. If validation fails at any step, the middleware immediately halts the request and returns a `401 Unauthorized` status code.

### Background Job Processing with Asynq

Asynq is central to the system's ability to handle long-running tasks without impacting user experience.

-   **Task Definitions**: Tasks are defined as strongly-typed Go structs, which are then serialized (e.g., to JSON) to be stored in Redis. This approach provides type safety and makes the task payloads self-documenting.

    ```go
    // tasks/payloads.go
    package tasks

    // CheckChannelPayload defines the data for a task that checks a YouTube channel for new videos.
    type CheckChannelPayload struct {
        SubscriptionID int
        ChannelID      string
    }

    // ProcessVideoPayload defines the data for a task that downloads and extracts audio from a video.
    type ProcessVideoPayload struct {
        SubscriptionID int
        VideoID        string
        VideoTitle     string
        PublishedAt    time.Time
    }
    ```

-   **Enqueuing Tasks**: The web server acts as an Asynq client. When a user performs an action that requires a long-running process (e.g., adding a new subscription), the corresponding HTTP handler immediately enqueues a task and returns a response to the user. For example, adding a new channel enqueues a `CheckChannelTask`.

-   **Worker Handlers**: The worker process defines handler functions for each task type. These handlers contain the actual business logic. For instance, the handler for `CheckChannelTask` will use `yt-dlp` to list recent videos and then enqueue multiple `ProcessVideoTask` jobs, one for each new video found. This separation of concerns—discovery vs. processing—is a key architectural pattern that enhances modularity.

-   **Scheduler**: A dedicated process or goroutine initializes an `asynq.Scheduler`. It is configured with cron-like expressions to periodically enqueue tasks. For example, it will register a job to run every hour, which queries the database for all active subscriptions and enqueues a `CheckChannelTask` for each one. This ensures that all user feeds are regularly and automatically updated.

### Audio Extraction Workflow

The audio extraction is performed within the `ProcessVideoTask` handler in the worker.

1.  **State Update**: The first step in the handler is to update the episode's status in the database to `PROCESSING`. This prevents other workers from picking up the same job and provides visibility into the system's state.
2.  **Secure Command Execution**: The `os/exec` package is used to invoke the `yt-dlp` command-line tool. To prevent command injection vulnerabilities, arguments are passed as a slice of strings to `exec.Command` rather than being concatenated into a single command string.
3.  **Command Construction**: The worker constructs and executes a command similar to the following:
    ```bash
    yt-dlp \
        -x \
        --audio-format m4a \
        -o "/path/to/audio/{audio_uuid}.%(ext)s" \
        "https://www.youtube.com/watch?v={video_id}"
    ```
    -   `-x` (`--extract-audio`): Instructs `yt-dlp` to download only the audio stream.
    -   `--audio-format m4a`: Specifies the desired output audio format. M4A (AAC) offers a good balance of quality and compatibility with podcast clients.
    -   `-o`: Defines the output filename template. Using the pre-generated `audio_uuid` ensures a unique, non-conflicting, and non-enumerable filename.
4.  **Metadata Update**: Upon successful execution of the command, the worker retrieves the final file size from the filesystem and updates the corresponding row in the `episodes` table. The status is set to `COMPLETED`, and the `audio_path` and `audio_size_bytes` fields are populated. If the command fails, the status is set to `FAILED`, and the error is logged for later inspection.

### RSS Feed Generation

The RSS feed is generated dynamically on request, ensuring it always reflects the most current state of a user's processed episodes.

-   **Endpoint**: The service exposes a `GET /rss/{user_rss_uuid}` endpoint.
-   **Data Fetching**: When a request is received, the handler extracts the `user_rss_uuid`, queries the `users` table to identify the user, and then fetches all episodes for that user with a status of `COMPLETED`, ordered by publication date.
-   **Feed Construction**: The `eduncan911/podcast` library is used to construct the feed in memory. This library provides a high-level API for creating RSS 2.0 feeds that are compliant with podcasting standards, including the iTunes namespace.
-   **Item Population**: The handler iterates through the fetched episode records. For each record, it creates a `podcast.Item` and populates its fields (Title, Description, PubDate, etc.) from the database columns.
-   **Enclosure Tag**: A critical step is calling `item.AddEnclosure()`. This method correctly formats the `<enclosure>` tag, which is mandatory for podcast clients to find and download the audio file. It requires the full public URL of the audio file (constructed using the `BASE_URL` and `audio_uuid`), the file size in bytes, and the MIME type.
-   **Response**: Finally, the handler sets the `Content-Type` header of the HTTP response to `application/rss+xml` and writes the serialized XML feed to the response body.

## API Endpoints & Frontend Interaction

The following table defines the contract between the Go backend and the htmx-powered frontend. It serves as a comprehensive map of the application's surface area, detailing the purpose of each route and its role in the user experience.

| Method | Path                      | Handler              | Description                                                                                                                                            |
| :----- | :------------------------ | :------------------- | :----------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GET`  | `/`                       | `serveWebApp`        | Serves the main `index.html` shell for the Telegram Mini App. This file includes the htmx library via CDN.                                                 |
| `POST` | `/auth`                   | `authenticateUser`   | A middleware endpoint that validates the `initData` passed in the `Authorization` header. All subsequent requests are protected by this.                   |
| `GET`  | `/subscriptions`          | `getSubscriptions`   | (HTMX) Fetches the user's current subscriptions from the DB and returns an HTML fragment containing the list of channels. Triggered on page load via `hx-get`. |
| `POST` | `/subscriptions`          | `addSubscription`    | (HTMX) Receives a YouTube channel URL from a form. Adds it to the DB, enqueues a `CheckChannelTask`, and returns the updated HTML fragment of the subscription list via `hx-swap`. |
| `DELETE`| `/subscriptions/{id}`   | `deleteSubscription` | (HTMX) Deletes a subscription by its ID. Returns an empty response (200 OK), and the frontend removes the corresponding element from the DOM via `hx-target="closest tr"`. |
| `GET`  | `/rss/{user_rss_uuid}`    | `serveRssFeed`       | Serves the generated XML RSS feed. This is the public URL the user will add to their podcast client.                                                     |
| `GET`  | `/audio/{audio_uuid}.m4a` | `serveAudioFile`     | Serves a specific audio file from the path specified in the `episodes` table, using `http.ServeFile`.                                                    |

## Security Considerations

-   **Input Validation**: All user-provided input, particularly YouTube channel URLs submitted via the frontend, must be rigorously validated on the server side. This includes checking for a valid URL format and potentially using a regex to ensure it conforms to YouTube's structure before it is passed to any internal logic or external tools.
-   **Command Injection Prevention**: As detailed in the Audio Extraction Workflow, the use of `os/exec.Command` with separate string arguments is mandatory. At no point should user-provided input be used to construct a command string via concatenation or formatting, as this would create a severe command injection vulnerability.
-   **Resource Enumeration Prevention**: The use of non-sequential, non-guessable UUIDs for both RSS feed URLs (`rss_uuid`) and audio file URLs (`audio_uuid`) is a critical security measure. This prevents malicious actors from discovering other users' content by simply incrementing a numerical ID in the URL.
-   **Resource Management and Abuse Prevention**: To ensure service stability and fairness, several controls must be implemented:
    -   **Rate Limiting**: Apply rate limiting to API endpoints, especially the `POST /subscriptions` endpoint, to prevent a single user from overwhelming the system with requests.
    -   **Subscription Limits**: Enforce a reasonable limit on the number of active subscriptions per user.
    -   **Process Timeouts**: When executing `yt-dlp`, use a context with a timeout (`context.WithTimeout`) to ensure that a hung or excessively long download process is killed, freeing up system resources.
    -   **Queue Prioritization**: If a tiered service model is ever introduced, Asynq's support for multiple queues can be used to process jobs from paying users with higher priority than those from free users.