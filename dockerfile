# Build stage
FROM python:3.10-slim AS builder

WORKDIR /app

# Install dependencies first to leverage cache
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy source code
COPY . .

# Compile Python bytecode
RUN python -m compileall /app/src

# Final stage
FROM python:3.10-slim

WORKDIR /app

# Copy only necessary files from builder
COPY --from=builder /usr/local/lib/python3.10/site-packages/ /usr/local/lib/python3.10/site-packages/
COPY --from=builder /app/src /app/src

ENV PYTHONPATH=/app
EXPOSE 8000

CMD ["uvicorn", "src.main:app", "--host", "0.0.0.0", "--port", "8000"]