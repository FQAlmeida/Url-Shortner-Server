# Use the official Go image as the base image
FROM golang

# Set the working directory inside the container
WORKDIR /app

# Copy the source code into the container
COPY src/ .

# Build the Go application
RUN go build -o server .

# Expose the port on which the API will run
EXPOSE 8080

# Set the command to run the Go application
CMD ["./server"]
