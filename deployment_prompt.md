
# Prompt for AI Agent: Create a Complete Deployment Setup

## Objective

Your task is to create a complete CI/CD pipeline for a web application using Docker and GitHub Actions. This pipeline will automatically build a Docker image, push it to the GitHub Container Registry (GHCR), and trigger a redeployment using a webhook.

## Context

The project is a standard web application with a frontend and a backend. The goal is to containerize the application and automate the deployment process. The deployment strategy involves updating a `docker-compose.yml` file in a separate `deploy` branch, which then triggers a redeployment.

## Core Components

Here are the templates for the core components of the deployment setup.

### 1. Dockerfile

This `Dockerfile` is for a simple web server (Nginx) that serves static files. It includes a build argument to embed the commit SHA into the application for traceability.

```dockerfile
# Dockerfile Template
FROM nginx:alpine

# Add build argument for commit SHA
ARG COMMIT_SHA=unknown

# Copy all static files from the build output directory
# Replace `build` with the actual directory of your built application
COPY build/ /usr/share/nginx/html

# Replace the placeholder in the main HTML file with the commit SHA
# This allows you to see which version of the code is deployed
RUN sed -i "s/__COMMIT_SHA__/${COMMIT_SHA}/g" /usr/share/nginx/html/index.html
```

### 2. docker-compose.yml

This `docker-compose.yml` file defines the application service. It uses Traefik for reverse proxying and specifies an external network. The image tag will be updated automatically by the GitHub Actions workflow.

```yaml
# docker-compose.yml Template
version: "3.8"

services:
  my-app:
    # The image tag will be replaced by the GitHub Actions workflow
    image: ghcr.io/YOUR_GITHUB_USERNAME/YOUR_REPOSITORY_NAME:latest
    container_name: my-app
    networks:
      - my-network # Replace with your network name
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.my-app.rule=Host(`app.your-domain.com`)" # Replace with your domain
      - "traefik.http.routers.my-app.entrypoints=websecure"
      - "traefik.http.routers.my-app.tls.certresolver=myresolver"
    restart: unless-stopped

networks:
  my-network: # Replace with your network name
    external: true
```

### 3. GitHub Actions Workflow

This workflow, located at `.github/workflows/deploy.yml`, automates the entire deployment process.

```yaml
# .github/workflows/deploy.yml Template
name: Deploy Application

on:
  push:
    branches:
      - main
  workflow_dispatch:

jobs:
  build-and-push:
    runs-on: ubuntu-latest
    permissions:
      contents: write
      packages: write

    steps:
      - name: Checkout repository
        uses: actions/checkout@v4

      - name: Log in to GitHub Container Registry
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build and push Docker image
        id: build-and-push
        uses: docker/build-push-action@v5
        with:
          push: true
          tags: ghcr.io/${{ github.repository_owner }}/${{ github.event.repository.name }}:${{ github.sha }}
          build-args: |
            COMMIT_SHA=${{ github.sha }}

      - name: Update and commit docker-compose.yml
        run: |
          git config user.name "GitHub Actions Bot"
          git config user.email "actions@github.com"
          git checkout -B deploy
          sed -i "s|image: ghcr.io/${{ github.repository_owner }}/${{ github.event.repository.name }}:.*|image: ghcr.io/${{ github.repository_owner }}/${{ github.event.repository.name }}:${{ github.sha }}|g" docker-compose.yml
          git add docker-compose.yml
          git commit -m "ci: Update image tag to ${{ github.sha }}"
          git push origin deploy --force

      - name: Trigger Portainer Redeploy Webhook
        uses: distributhor/workflow-webhook@v3
        with:
          webhook_url: ${{ secrets.PORTAINER_REDEPLOY_HOOK }}
          webhook_secret: "trigger"
```

## Instructions

1.  **Create the files**: Create the `Dockerfile`, `docker-compose.yml`, and `.github/workflows/deploy.yml` files in the root of your project.
2.  **Customize the templates**: Replace the placeholders in the templates with the appropriate values for your project.
3.  **Add secrets**: Add the `PORTAINER_REDEPLOY_HOOK` secret to your GitHub repository settings. This secret should contain the URL of your Portainer webhook.
4.  **Push to `main`**: Push your changes to the `main` branch to trigger the workflow.

## Placeholders

Here is a list of placeholders you need to replace in the templates:

*   `build/`: In the `Dockerfile`, replace this with the path to your application's build output directory.
*   `YOUR_GITHUB_USERNAME`: In `docker-compose.yml`, replace this with your GitHub username or organization name.
*   `YOUR_REPOSITORY_NAME`: In `docker-compose.yml`, replace this with your repository name.
*   `my-app`: In `docker-compose.yml`, replace this with the name of your application.
*   `my-network`: In `docker-compose.yml`, replace this with the name of your Docker network.
*   `app.your-domain.com`: In `docker-compose.yml`, replace this with the domain name for your application.
*   `PORTAINER_REDEPLOY_HOOK`: In `.github/workflows/deploy.yml`, this is a secret that you need to add to your repository settings.

By following these instructions, you will have a fully automated CI/CD pipeline for your application.
