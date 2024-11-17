#!/bin/bash

unformatted_files=$(gofmt -l .)

if [ "$unformatted_files" != "" ]; then
  echo >&2 "Following files are not formatted, please run go fmt ./..."
  echo >&2 "$unformatted_files"
  exit 1
fi
