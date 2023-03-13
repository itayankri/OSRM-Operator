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

ONE_HOUR_FROM_NOW=$(date -d "+1 hour")
HOUR=$(date -d "$ONE_HOUR_FROM_NOW" +"%H" | sed 's/^0*//')
DAY_OF_WEEK=$(expr $(date -d "$ONE_HOUR_FROM_NOW" +"%u") - 1)

PARTITION="day_of_week=$DAY_OF_WEEK/hour_of_day=$HOUR/"

OBJECTS=`curl -X GET "$URL/?prefix=$PARTITION" | jq -r '.items'`

for row in $(echo "$OBJECTS" | jq -r '.[] | @base64'); do
    _jq() {
     echo ${row} | base64 --decode | jq -r ${1}
    }

    if [[ $(_jq '.selfLink') == *.csv ]]
    then
      FULL_URL=$(_jq '.selfLink')

      echo "Downloading speed updates CSV from $FULL_URL"
      curl $FULL_URL -o speeds.csv

      echo "Customizing map data"
      osrm-customize $OSRM_FILE_NAME --segment-speed-file speeds.csv
    fi
done
