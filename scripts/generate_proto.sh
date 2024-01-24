#!/usr/bin/env bash

protoc -I . service.proto --go_out=./ --go-grpc_out=./
