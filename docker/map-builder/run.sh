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
TEMP_DIR=$(date "+%F-%T")

cd $ROOT_DIR
mkdir -p $PARTITIONED_DATA_DIR $CUSTOMIZED_DATA_DIR $TEMP_DIR

cd $TEMP_DIR

echo "--> Downloading PBF file from $PBF_URL"
curl -O $PBF_URL

echo "--> Extracting PBF"
osrm-extract -p /opt/$PROFILE.lua $PBF_FILE_NAME && \

echo "--> Partitioning map data"
osrm-partition $OSRM_FILE_NAME

echo "--> Copying partitioned data to $PARTITIONED_DATA_DIR"
cp * ../$PARTITIONED_DATA_DIR

echo "--> Customizing map data"
osrm-customize $OSRM_FILE_NAME

echo "--> Copying customized ata to $CUSTOMIZED_DATA_DIR"
cp * ../$CUSTOMIZED_DATA_DIR

echo "--> Cleaning up temp dir $TEMP_DIR"
cd $ROOT_DIR
rm -rf $TEMP_DIR