# Stage 1: Build
FROM python:3.9-slim AS build
WORKDIR /app
COPY requirements.txt .
RUN pip install -r requirements.txt
COPY . .
RUN pip install -r requirements.txt --no-deps

# Stage 2: Final
FROM python:3.9-slim
WORKDIR /app
COPY --from=build /app/ ./
CMD ["python", "app.py"]