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

if [[ -z "${PBF_URL}" ]]; then
  echo "PBF_URL environemnt variable must be provided"
  exit 1
fi

if [[ -z "${PROFILE}" ]]; then
  echo "PROFILE environemnt variable must be provided"
  exit 1
fi

PBF_FILE_NAME=$(basename $PBF_URL)
OSRM_FILE_NAME="${PBF_FILE_NAME/osm.pbf/osrm}"

cd $ROOT_DIR
mkdir $PARTITIONED_DATA_DIR $CUSTOMIZED_DATA_DIR
cd $PARTITIONED_DATA_DIR

echo "Downloading PBF file from $PBF_URL"
curl -O $PBF_URL

echo "Extracting PBF"
osrm-extract -p /opt/$PROFILE.lua $PBF_FILE_NAME $EXTRACT_OPTIONS && \

echo "Partitioning map data"
osrm-partition $OSRM_FILE_NAME $PARTITION_OPTIONS

cp * ../$CUSTOMIZED_DATA_DIR
cd ../$CUSTOMIZED_DATA_DIR

echo "Customizing map data"
osrm-customize $OSRM_FILE_NAME $CUSTOMIZE_OPTIONS --time-zone-file ./timezone-file.json