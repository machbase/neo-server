# Machbase Neo gRPC API Guide

gRPC is the first-class-api of machbase, any program language that support gRPC can utilize it's full functionalities.

**Warning**: Since gRPC interface provides low level api, it is very critical that client program should properly use them. In any cases of mis-uses, it may lead machbase not to work properly.

## Proto File

The latest version of .proto file is hosted in github. Please [find it from github](https://github.com/machbase/neo-server/tree/main/api/proto/machrpc.proto).