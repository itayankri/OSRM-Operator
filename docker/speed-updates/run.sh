#!/bin/bash

# Validate required environment variables
REQUIRED_VARS=("ROOT_DIR" "PARTITIONED_DATA_DIR" "CUSTOMIZED_DATA_DIR" "URL" "OSRM_FILE_NAME")
for var in "${REQUIRED_VARS[@]}"; do
  if [[ -z "${!var}" ]]; then
    echo "${var} environment variable must be provided"
    exit 1
  fi
done

# Ensure the script can be safely interrupted
trap 'exit 1' SIGINT SIGTERM

# Define directories
ROOT_DIR_PATH="$ROOT_DIR"
PARTITIONED_DIR_PATH="$ROOT_DIR/$PARTITIONED_DATA_DIR"
CUSTOMIZED_DIR_PATH="$ROOT_DIR/$CUSTOMIZED_DATA_DIR"
TEMP_DIR_PATH="${CUSTOMIZED_DIR_PATH}_new"

# Create a temporary directory for new files
mkdir -p "$TEMP_DIR_PATH"

# Calculate date-specific URL
ONE_HOUR_FROM_NOW=$(date -d "+1 hour")
HOUR=$(date -d "$ONE_HOUR_FROM_NOW" +"%H")
HOUR=${HOUR#0}  # Remove leading zero
DAY_OF_WEEK=$(( $(date -d "$ONE_HOUR_FROM_NOW" +"%u") - 1 ))
FULL_URL="$URL/$DAY_OF_WEEK/$HOUR.csv"

echo "Downloading speed updates CSV from $FULL_URL"
if ! curl -f "$FULL_URL" -o "$TEMP_DIR_PATH/speeds.csv"; then
  echo "Failed to download speed updates"
  rmdir "$TEMP_DIR_PATH"
  exit 1
fi

# Copy partitioned data to temporary directory
echo "Copying fresh partitioned map data"
cp -r "$PARTITIONED_DIR_PATH"/* "$TEMP_DIR_PATH"

# Customize map data in the temporary directory
echo "Customizing map data"
if ! osrm-customize "$TEMP_DIR_PATH/$OSRM_FILE_NAME" --segment-speed-file "$TEMP_DIR_PATH/speeds.csv"; then
  echo "Customization failed"
  rm -rf "$TEMP_DIR_PATH"
  exit 1
fi

sleep 2

# Atomically replace the old directory
# This minimizes the time the directory is unavailable
echo "Swapping directories"
mv "$CUSTOMIZED_DIR_PATH" "${CUSTOMIZED_DIR_PATH}_old"

sleep 2

mv "$TEMP_DIR_PATH" "$CUSTOMIZED_DIR_PATH"

sleep 2

# Clean up the old directory
rm -rf "${CUSTOMIZED_DIR_PATH}_old"

echo "Customization complete"