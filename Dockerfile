# Use the official Golang image to build the binary
FROM golang:1.22 AS build

# Set the working directory inside the container
WORKDIR /app

# Copy the Go module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application source code
COPY . .

# Build the Go application for the target platform (buildx will handle architecture switching)
RUN go build -o informer cmd/informer/main.go

# Create a minimal runtime image using Ubuntu
FROM ubuntu:22.04

WORKDIR /app

# Install necessary dependencies and set timezone
RUN apt-get update && apt-get install -y ca-certificates libc6 tzdata && \
    ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Copy the Go binary from the build stage
COPY --from=build /app/informer /app/informer

# Ensure the binary has executable permissions
RUN chmod +x /app/informer

# Expose necessary ports (if any)
EXPOSE 8080

# Set the entrypoint command to run the application
CMD ["./informer"]
