FROM golang:1.19

# Set the working directory within the container
WORKDIR /app

# Copy go.mod and go.sum files to the container's working directory
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code to the container's working directory
COPY . .

# make the exec.sh file executable
RUN chmod +x ./docker/exec.sh

# Build the Go application
RUN go build -o qovery

# Add the /app directory to the PATH environment variable
ENV PATH="/app:${PATH}"

ENTRYPOINT ["sh", "./docker/exec.sh"]
