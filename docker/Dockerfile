# Start from Go base image
FROM golang:1.24-alpine

# Set working directory
WORKDIR /app

# Copy go mod and source
COPY ../go.mod ./
COPY .. ./

# Build the app
RUN go build -o pickems

# Command to run the app
CMD ["./pickems"]
