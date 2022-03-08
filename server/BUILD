load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "server_lib",
    srcs = ["server.go"],
    importpath = "github.com/vadimberezniker/l7-ilb-flakiness/server",
    visibility = ["//visibility:private"],
    deps = [
        "//proto:dummy_go_proto",
        "@org_golang_google_grpc//:go_default_library",
        "@org_golang_google_grpc//codes",
        "@org_golang_google_grpc//credentials",
        "@org_golang_google_grpc//peer",
        "@org_golang_google_grpc//status",
    ],
)

go_binary(
    name = "server",
    embed = [":server_lib"],
    pure = "on",
    static = "on",
    visibility = ["//visibility:public"],
)
