#!/bin/bash

# CD to script directory
cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null

# Build the binary and output to the bin folder
go build -o ../bin/
