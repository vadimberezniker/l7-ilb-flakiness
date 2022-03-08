load("@rules_proto//proto:defs.bzl", "proto_library")
load("@io_bazel_rules_go//proto:def.bzl", "go_proto_library")

package(default_visibility = ["//visibility:public"])

proto_library(
    name = "dummy_proto",
    srcs = ["dummy.proto"],
    deps = [
    ],
)

go_proto_library(
    name = "dummy_go_proto",
    compilers = ["@io_bazel_rules_go//proto:go_grpc"],
    importpath = "github.com/vadimberezniker/l7-ilb-flakiness/proto/dummy",
    proto = ":dummy_proto",
)

