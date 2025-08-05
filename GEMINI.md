# Gemini Project Context

## Project Overview

- **Project Type**: Python FastAPI application.
- **Main Functionality**: A LINE Bot designed to search for and provide information about medical personnel in Taiwan.
- **Core Module**: `src/linebot` contains the primary business logic, including handlers, services, and routing for the LINE Bot.
- **Dependencies**: FastAPI, SQLAlchemy, line-bot-sdk, psycopg2.

## Development Conventions

- **Testing Framework**: Not specified.
- **Code Style**: Not specified.
- **Dependency Management**: `requirements.txt`

## Key Commands

- **Run Linter**: Not specified.
- **Run Tests**: Not specified.
- **Run Application**: `uvicorn src.main:app --host 0.0.0.0 --port 8000`

## Architectural Decisions & History

- The application is built using FastAPI and follows a modular structure.
- The `src/linebot` module encapsulates all LINE Bot-related functionality.
- Database interactions are handled by SQLAlchemy.
- The application is designed to be containerized, as indicated by the `dockerfile`.

## TODO

- Write unit and integration tests for the `linebot` module.
- Establish a CI pipeline in `.github/workflows/ci.yaml` to automate linting and testing.
- Define and enforce a consistent code style using a linter/formatter.
