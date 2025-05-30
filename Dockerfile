# --- Stage 1: Builder ---
    FROM golang:1.24-alpine AS builder

    WORKDIR /app
    
    # Install required packages
    RUN apk add --no-cache git ca-certificates
    
    # Copy and download dependencies
    COPY go.mod go.sum ./
    RUN go mod tidy
    
    # Copy the source
    COPY . .
    
    # Build the binary
    RUN go build -o main .
    
    # --- Stage 2: Minimal Runtime Image ---
    FROM alpine:latest
    
    WORKDIR /root/
    
    # Install SSL certificates
    RUN apk --no-cache add ca-certificates
    
    # Copy binary from builder stage
    COPY --from=builder /app/main .
    COPY --from=builder /app/templates ./templates
    
    # Expose port
    EXPOSE 8080

    # Set env default values (optional, can be overridden)
    ENV PORT=8080
    
    CMD ["./main"]
    