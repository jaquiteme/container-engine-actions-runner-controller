#!/usr/bin/env bash
# Description: Logger

# Config
LOG_LEVEL="${LOG_LEVEL:-notice}"   # Niveau par défaut
LOG_USE_COLOR="${LOG_USE_COLOR:-true}"

# --- Log levels ---
# debug < info < notice < warn < error < fatal
declare -A LOG_LEVELS=(
  [debug]=0
  [info]=1
  [notice]=2
  [warn]=3
  [error]=4
  [fatal]=5
)

# --- Colors ---
if [ "$LOG_USE_COLOR" = "true" ]; then
  COLOR_DEBUG="\033[36m"   # Cyan
  COLOR_INFO="\033[32m"    # Green
  COLOR_NOTICE="\033[34m"  # Blue
  COLOR_WARN="\033[33m"    # Yellow
  COLOR_ERROR="\033[31m"   # Red
  COLOR_FATAL="\033[1;31m" # Dark Red
  COLOR_RESET="\033[0m"
else
  COLOR_DEBUG=""
  COLOR_INFO=""
  COLOR_NOTICE=""
  COLOR_WARN=""
  COLOR_ERROR=""
  COLOR_FATAL=""
  COLOR_RESET=""
fi

# --- Internal functions ---
_log() {
  local level="$1"
  shift
  local msg="$*"

  # Check log level
  if [ "${LOG_LEVELS[$level]}" -lt "${LOG_LEVELS[$LOG_LEVEL]}" ]; then
    return
  fi

  # Select a color
  local color_var="COLOR_${level^^}"
  local color="${!color_var}"

  # Timestamp
  local ts
  ts="$(date '+%Y-%m-%d %H:%M:%S')"

  # Print
  printf "%b[%s] %-7s%b %s\n" \
    "$color" "$ts" "$level" "$COLOR_RESET" "$msg"
}

# --- Public functions ---
log.debug()  { _log debug  "$@"; }
log.info()   { _log info   "$@"; }
log.notice() { _log notice "$@"; }
log.warn()   { _log warn   "$@"; }
log.error()  { _log error  "$@"; }
log.fatal()  { _log fatal  "$@"; exit 1; }