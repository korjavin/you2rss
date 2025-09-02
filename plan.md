This document outlines a phased implementation plan for the YT-Podcaster service. The plan is structured into seven distinct milestones, designed to build the system incrementally, starting with a solid foundation and progressively adding features. This approach allows for testing and validation at each stage of development.

Milestone 1: Project Scaffolding and Core Backend Setup
This foundational milestone establishes the project structure, database schema, and basic server functionality.

Task 1.1: Initialize Go Module: Create the project directory and initialize it as a Go module using go mod init yt-podcaster.   

Task 1.2: Establish Project Directory Structure: Create a conventional Go project layout to organize code logically (e.g., /cmd for main applications, /internal for private application logic, /pkg for public libraries).

Task 1.3: Set Up Basic HTTP Server: In cmd/server/main.go, implement a minimal web server using the standard net/http package. It should listen on a configured port and respond to a root request, confirming the basic server is operational.   

Task 1.4: Implement Database Schema and Migrations: Define the SQL CREATE TABLE statements for the users, subscriptions, and episodes tables as specified in the architecture document. Use a migration tool like golang-migrate/migrate to version-control the schema and apply it to the database.

Task 1.5: Implement Configuration Management: Integrate a library such as viper or godotenv to load application settings (database URL, Redis address, etc.) from a .env file, keeping configuration separate from code.

Milestone 2: User Authentication and Management
This milestone focuses on implementing the security layer. All subsequent feature development will depend on a reliable way to identify and authorize users. Building this component first ensures that the application is secure by default, rather than attempting to add security as an afterthought.

Task 2.1: Create Authentication Middleware: Implement a Go http.Handler middleware. This middleware will extract the Authorization: tma... header from incoming requests and use the init-data-golang library to validate the initData string against the TELEGRAM_BOT_TOKEN.   

Task 2.2: Implement User Upsert Logic: Within the middleware, upon successful validation, parse the user data. Implement the database logic to find a user by their Telegram ID or create a new entry if one does not exist (an "upsert" operation).

Task 2.3: Create Initial Frontend Shell: Develop a basic index.html file. This file will serve as the entry point for the Telegram Mini App and must include the script tag for the htmx library, loaded from a CDN.   

Task 2.4: Implement Root Handler: Create the Go handler for the / route that serves the index.html file and is protected by the authentication middleware created in Task 2.1.

Milestone 3: Subscription Management UI and API
This milestone builds the core user-facing functionality: the ability to manage YouTube channel subscriptions.

Task 3.1: Develop Subscription Form: In index.html, add an HTML form for submitting a new YouTube channel URL. Use htmx attributes to specify the backend endpoint and behavior: hx-post="/subscriptions" will send the form data, and hx-target="#subscription-list" will define where the server's response (the updated list) should be rendered.   

Task 3.2: Develop Subscription List View: Add a div with id="subscription-list" to index.html. Use hx-get="/subscriptions" and hx-trigger="load" to instruct htmx to immediately fetch the initial list of subscriptions from the server when the page loads.

Task 3.3: Implement POST /subscriptions Handler: Create the Go handler for adding a new subscription. This handler will parse the channel URL from the form, validate it, save the new subscription to the database for the authenticated user, and then render and return an HTML fragment containing the complete, updated list of subscriptions.

Task 3.4: Implement GET /subscriptions Handler: Create the Go handler that fetches the current user's subscriptions from the database and renders them into the corresponding HTML list fragment.

Task 3.5: Add Delete Functionality: In the HTML template for a subscription item, add a "Delete" button. Use htmx attributes hx-delete="/subscriptions/{{.ID}}" and hx-target="closest tr" to send a DELETE request and remove the table row from the DOM upon a successful response.   

Task 3.6: Implement DELETE /subscriptions/{id} Handler: Create the Go handler to delete a subscription from the database based on its ID.

Milestone 4: Background Worker and Audio Processing
This milestone involves setting up the asynchronous processing backend.

Task 4.1: Configure Asynq Client and Server: In the web server application (cmd/server), initialize an asynq.Client to enqueue tasks. In a new worker application (cmd/worker), initialize an asynq.Server to process tasks from the queues.   

Task 4.2: Define Task Payloads: In a shared package, define the Go structs for task payloads (e.g., ProcessVideoPayload, CheckChannelPayload) to ensure type safety between the client and server.

Task 4.3: Implement ProcessVideoTask Handler: Create the handler function for the video processing task. This function will contain the core logic:

Update the episode's database status to PROCESSING.

Securely invoke yt-dlp using os/exec with a timeout context.   

Handle both successful and failed outcomes by updating the episode's status to COMPLETED or FAILED and recording relevant metadata like file size or error messages.

Task 4.4: Integrate Task Enqueuing: Modify the POST /subscriptions handler. After successfully adding a new subscription, it should immediately enqueue a CheckChannelTask to begin fetching content for the new channel.

Milestone 5: Periodic Job Scheduling
This milestone automates the process of keeping feeds up-to-date.

Task 5.1: Set Up Asynq Scheduler: In a new cmd/scheduler application (or within the worker), initialize an asynq.Scheduler instance connected to the same Redis server.   

Task 5.2: Register Periodic "Check All" Task: Register a task to run on a recurring schedule (e.g., hourly, using a cron string like @every 1h). This task will be responsible for initiating the channel update process for all users.

Task 5.3: Implement Channel Update Logic: The handler for the periodic task will query the subscriptions table for all active subscriptions. For each subscription, it will execute yt-dlp --flat-playlist -j {channel_url}. This command efficiently fetches a JSON list of recent video metadata without downloading the full videos.

Task 5.4: Enqueue Processing Tasks: The handler will compare the list of video IDs from yt-dlp against the youtube_video_id column in the episodes table. For any video ID that does not yet have a corresponding entry, it will create a PENDING episode record in the database and enqueue a ProcessVideoTask.

Milestone 6: RSS Feed and Audio Serving
This final functional milestone exposes the generated content to the user's podcast client.

Task 6.1: Implement RSS Feed Handler: Create the Go handler for GET /rss/{user_rss_uuid}. This handler will perform the database queries to find the user and their COMPLETED episodes.

Task 6.2: Generate RSS XML: Use the eduncan911/podcast library to construct the RSS feed in memory. Iterate over the episode records to create podcast.Item entries, paying close attention to correctly populating the <enclosure> tag with the public audio URL, file size, and MIME type.   

Task 6.3: Implement Audio Serving Handler: Create the handler for GET /audio/{audio_uuid}.m4a. This handler will use http.ServeFile to efficiently and securely serve the static audio file from the configured AUDIO_STORAGE_PATH, letting the standard library handle headers like Content-Type, Content-Length, and byte-range requests.

Milestone 7: Finalization and Deployment
This milestone prepares the project for production use.

Task 7.1: Write Unit Tests: Develop unit tests for critical business logic, such as URL validation, database queries, and task payload serialization, using Go's built-in testing package.

Task 7.2: Write Integration Tests: Create integration tests for the API endpoints to verify the full request-response cycle, including middleware authentication.

Task 7.3: Create Dockerfile: Write a multi-stage Dockerfile to build a minimal, optimized, and secure container image for the Go application.

Task 7.4: Create Docker Compose Configuration: Develop a docker-compose.yml file to orchestrate the entire stack for local development and testing. This file will define services for the Go web server, the Go worker, Redis, and PostgreSQL, linking them together.

Task 7.5: Write Deployment Guide: Document the steps required to deploy the application to a production environment, such as a cloud virtual machine or a Kubernetes cluster. This should include notes on setting up the prerequisites, configuring environment variables, and managing the application processes.