FROM golang:1.25-alpine

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy dependency files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build
RUN go build -o main .

# Expose port
EXPOSE 8080

# Run
CMD ["./main"]