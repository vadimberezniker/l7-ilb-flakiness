load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "client_lib",
    srcs = ["client.go"],
    importpath = "github.com/vadimberezniker/l7-ilb-flakiness/client",
    visibility = ["//visibility:private"],
    deps = [
        "//proto:dummy_go_proto",
        "@org_golang_google_grpc//:go_default_library",
        "@org_golang_google_grpc//credentials/google",
        "@org_golang_x_sync//errgroup",
    ],
)

go_binary(
    name = "client",
    embed = [":client_lib"],
    pure = "on",
    static = "on",
    visibility = ["//visibility:public"],
)
