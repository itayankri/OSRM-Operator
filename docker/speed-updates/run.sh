#!/bin/bash

echo "Evironment:"
printenv

if [[ -z "${ROOT_DIR}" ]]; then
  echo "ROOT_DIR environemnt variable must be provided"
  exit 1
fi

if [[ -z "${PARTITIONED_DATA_DIR}" ]]; then
  echo "PARTITIONED_DATA_DIR environemnt variable must be provided"
  exit 1
fi

if [[ -z "${CUSTOMIZED_DATA_DIR}" ]]; then
  echo "CUSTOMIZED_DATA_DIR environemnt variable must be provided"
  exit 1
fi

if [[ -z "${URL}" ]]; then
  echo "URL environemnt variable must be provided"
  exit 1
fi

if [[ -z "${OSRM_FILE_NAME}" ]]; then
  echo "OSRM_FILE_NAME environemnt variable must be provided"
  exit 1
fi

cd $ROOT_DIR/$CUSTOMIZED_DATA_DIR

echo "Deleting old customised map data"
rm -rf *

echo "Copying fresh partitioned map data"
cp -r ../$PARTITIONED_DATA_DIR/* .

MINUTES=$(date +"%M")
if [ "$MINUTES" -gt 55 ]; then
  DATE_HOUR=$(date -d "+1 hour")
  HOUR=$(date -d "$DATE_HOUR" +"%H")
else
  DATE_HOUR=$(date)
  HOUR=$(date +"%H")
fi

if [ "$HOUR" == "00" ]; then
  HOUR="0"
else
  HOUR=$(echo $HOUR | sed 's/^0*//')
fi

DAY_OF_WEEK=$(expr $(date -d "$DATE_HOUR" +"%u") - 1)
FULL_URL="$URL/$DAY_OF_WEEK/$HOUR.csv"

echo "Downloading speed updates CSV from $FULL_URL"
curl $FULL_URL -o speeds.csv

echo "Customizing map data"
osrm-customize $OSRM_FILE_NAME --segment-speed-file speeds.csv