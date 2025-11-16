FROM golang:1.22.4-alpine

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Set the working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN go build -o hotel-reservation .

# Expose the port
EXPOSE 5000

# Run the application
CMD ["./hotel-reservation"]


