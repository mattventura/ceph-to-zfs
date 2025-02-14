#!/bin/sh

set -e

go build -o ./ctz ./pkg/ctz/cmd

echo "Done: built ./ctz"
