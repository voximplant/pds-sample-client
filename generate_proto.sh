#!/usr/bin/env bash

protoc -I . service.proto -I$GOPATH/pkg/mod/github.com/grpc-ecosystem/grpc-gateway@v1.5.1/third_party/googleapis --go_out=plugins=grpc:./service
protoc -I . service.proto -I$GOPATH/pkg/mod/github.com/grpc-ecosystem/grpc-gateway@v1.5.1/third_party/googleapis --grpc-gateway_out=logtostderr=true:./service
