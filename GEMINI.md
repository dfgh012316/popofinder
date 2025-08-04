# Gemini Project Context

This file helps Gemini understand the project's context, conventions, and commands to provide more accurate and efficient assistance.

## Project Overview

- **Project Type**: Python FastAPI application.
- **Main Functionality**: A LINE Bot designed to search for and provide information about medical personnel.
- **Core Module**: `src/linebot` contains the primary business logic, including handlers, services, and routing for the LINE Bot.

## Development Conventions

- **Testing Framework**: `pytest` is the designated framework for testing.
- **Code Style**: (To be defined, e.g., Black, Ruff)
- **Dependency Management**: `requirements.txt`

## Key Commands

- **Run Linter**: (To be defined, e.g., `ruff check .`)
- **Run Tests**: (To be defined, e.g., `pytest`)

## Architectural Decisions & History

- **2025-08-04**: Refactored the project to simplify its structure.
  - The `popo` module, which contained search logic, was merged into the `linebot` module as it was the sole consumer.
  - The `infra` module, which only contained a logger, was also merged into the `linebot` module.
  - This was done to reduce unnecessary modularization and make the project easier to navigate.

## TODO

- Write unit and integration tests for the `linebot` module.
- Establish a CI pipeline in `.github/workflows/ci.yaml` to automate linting and testing.
- Define and enforce a consistent code style using a linter/formatter.
