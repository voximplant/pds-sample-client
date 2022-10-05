#!/usr/bin/env bash

protoc -I . service.proto --go_out=./service --go-grpc_out=./service
