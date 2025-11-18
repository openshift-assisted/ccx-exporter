#!/bin/bash

# sync.sh runs the following steps
# - format secrets representing S3 credentials into rclone configuration
# - run rclone copy between the two s3 buckets
# At the end, if the src bucket has an object as s3://src-bucket/prefix1/path/obj.json,
# it should be copied into s3://dst-bucket/prefix2/path/obj.json

# -E Set error traps to be inherited by function
# -e Exits immediately if a command exits with non-zero
# -u Treat unset variables as an error when substituting(null is a valid value)
# -o pipefail The return value of a pipeline is the status of the last command to exit with a non-zero status
set -Eeuo pipefail

# Constants

REMOTE_SRC=src
REMOTE_DST=dst

# Helpers

function log { echo "[$(date '+%Y-%m-%d %H:%M:%S')] [$1] $2" >&2; }
function logInfo { log INFO "$1"; }
function logWarn { log WARN "$1"; }
function logError { log ERROR "$1"; }

function usage {
    cat >&2 <<EOF
Usage: $0 --src-secret DIR --src-prefix PREFIX --dst-secret DIR --dst-prefix PREFIX

Options:
  --src-secret DIR    Directory containing source S3 credentials
  --src-prefix PREFIX Source bucket prefix (e.g., "prefix1")
  --dst-secret DIR    Directory containing destination S3 credentials
  --dst-prefix PREFIX Destination bucket prefix (e.g., "prefix2")

Required files in each secret directory:
  - endpoint
  - aws_access_key_id
  - aws_secret_access_key
  - aws_region
  - bucket
EOF
    exit 1
}

# Arguments parsing

SRC_PREFIX=""
DST_PREFIX=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --src-secret)
            SRC_SECRET="$2"
            shift
            shift
            ;;
        --src-prefix)
            SRC_PREFIX="$2"
            shift
            shift
            ;;
        --dst-secret)
            DST_SECRET="$2"
            shift
            shift
            ;;
        --dst-prefix)
            DST_PREFIX="$2"
            shift
            shift
            ;;
        --help|-h)
            usage
            ;;
        *)
            logError "Unknown option $1"
            usage
            ;;
    esac
done

# Arguments validation

if [ -z "${SRC_SECRET+x}" ]; then
    logError "Source secret must be defined"
    usage
fi

if [ ! -d "${SRC_SECRET}" ]; then
    logError "Source secret (${SRC_SECRET}) is not a directory"
    exit 1
fi

if [ -z "${DST_SECRET+x}" ]; then
    logError "Destination secret must be defined"
    usage
fi

if [ ! -d "${DST_SECRET}" ]; then
    logError "Destination secret (${DST_SECRET}) is not a directory"
    exit 1
fi

if [ ! ${SRC_PREFIX} == "" ] && [[ ! ${SRC_PREFIX} != */ ]]; then
    logWarn "Source prefix (${SRC_PREFIX}) does not end with a slash, adding one"
    
    SRC_PREFIX="${SRC_PREFIX}/"
fi

if [ ! ${DST_PREFIX} == "" ] && [[ ! ${DST_PREFIX} != */ ]]; then
    logWarn "Destination prefix (${DST_PREFIX}) does not end with a slash, adding one"

    DST_PREFIX="${DST_PREFIX}/"
fi

# Functions

function readSecret {
    local path=$1
    local -n outputVar=$2

    if [ ! -f "${path}" ]; then
        logError "Required file does not exist: ${path}"
        exit 1
    fi

    local temp
    if ! temp=$(< "${path}"); then
        logError "Failed to read required file: ${path}"
        exit 1
    fi

    outputVar="${temp}"
}

function generateConfig {
    local remote=$1
    local secretPath=$2
    local config=$3

    local endpoint
    readSecret "${secretPath}/endpoint" endpoint
    if [[ ! "${endpoint}" == http* ]]; then
        endpoint="https://${endpoint}"
    fi

    local access_key_id
    readSecret "${secretPath}/aws_access_key_id" access_key_id

    local secret_access_key
    readSecret "${secretPath}/aws_secret_access_key" secret_access_key

    local region
    readSecret "${secretPath}/aws_region" region

    cat >>"${config}" <<endofconfig
[${remote}]
type = s3
provider = AWS
env_auth = false
access_key_id = ${access_key_id}
secret_access_key = ${secret_access_key}
region = ${region}
endpoint = ${endpoint}

endofconfig
}

# Main

if ! command -v rclone &> /dev/null; then
    logError "rclone is not installed or not in PATH"
    exit 1
fi

config=$(mktemp) 
trap "rm -f ${config}" EXIT

logInfo "Generating rclone configuration at ${config}"

generateConfig "${REMOTE_SRC}" "${SRC_SECRET}" "${config}"
generateConfig "${REMOTE_DST}" "${DST_SECRET}" "${config}"

srcBucket=""
readSecret "${SRC_SECRET}/bucket" srcBucket

dstBucket=""
readSecret "${DST_SECRET}/bucket" dstBucket


rclone copy "${REMOTE_SRC}:${srcBucket}/${SRC_PREFIX}" "${REMOTE_DST}:${dstBucket}/${DST_PREFIX}" \
    --config "${config}" \
    --size-only \
    --s3-no-head \
    --ignore-checksum \
    --s3-disable-checksum \
    --progress \
    --checkers 8 \
    --transfers 8

logInfo "rclone copy finished successfully."
