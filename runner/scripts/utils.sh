#!/usr/bin/env bash

# remove apt cache files
remove_apt_cache() {
    rm -rf rm -rf /var/lib/apt/lists/*
    rm -rf /tmp/*
}