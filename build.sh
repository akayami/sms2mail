#!/bin/bash

# Define the application name
APP_NAME="sms2mail"

# Define the output directory
OUTPUT_DIR="bin"

# Create the output directory if it doesn't exist
mkdir -p $OUTPUT_DIR

# Define the platforms to build for
# Format: "OS/ARCH"
PLATFORMS=(
    "darwin/amd64"
    "darwin/arm64"
    "linux/amd64"
    "linux/arm64"
    "windows/amd64"
    "windows/arm64"
)

echo "Building $APP_NAME..."

for PLATFORM in "${PLATFORMS[@]}"; do
    # Split the platform string into OS and ARCH
    IFS='/' read -r -a PARTS <<< "$PLATFORM"
    GOOS="${PARTS[0]}"
    GOARCH="${PARTS[1]}"

    # Define the output filename
    OUTPUT_NAME="$APP_NAME-$GOOS-$GOARCH"

    # Add .exe extension for Windows
    if [ "$GOOS" == "windows" ]; then
        OUTPUT_NAME+=".exe"
    fi

    echo "Building for $GOOS/$GOARCH..."
    
    # Build the application
    env GOOS=$GOOS GOARCH=$GOARCH go build -o "$OUTPUT_DIR/$OUTPUT_NAME"
    
    if [ $? -ne 0 ]; then
        echo "An error occurred while building for $GOOS/$GOARCH"
        exit 1
    fi
done

echo "Build complete! Binaries are located in the '$OUTPUT_DIR' directory."
