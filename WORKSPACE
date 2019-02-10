load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "492c3ac68ed9dcf527a07e6a1b2dcbf199c6bf8b35517951467ac32e421c06c1",
    urls = ["https://github.com/bazelbuild/rules_go/releases/download/0.17.0/rules_go-0.17.0.tar.gz"],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "7949fc6cc17b5b191103e97481cf8889217263acf52e00b560683413af204fcb",
    urls = ["https://github.com/bazelbuild/bazel-gazelle/releases/download/0.16.0/bazel-gazelle-0.16.0.tar.gz"],
)

load("@io_bazel_rules_go//go:deps.bzl", "go_rules_dependencies", "go_register_toolchains")

go_rules_dependencies()

go_register_toolchains()

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies", "go_repository")

gazelle_dependencies()

go_repository(
    name = "com_github_pkg_errors",
    commit = "ffb6e22f01932bf7ac35e0bad9be11f01d1c8685",
    importpath = "github.com/pkg/errors",
)

go_repository(
    name = "com_github_google_go_cmp",
    commit = "2248b49eaa8e1c8c0963ee77b40841adbc19d4ca",
    importpath = "github.com/google/go-cmp",
)

go_repository(
    name = "com_github_cavaliercoder_go_cpio",
    commit = "925f9528c45e5b74f52963bd11f1988ad99a95a5",
    importpath = "github.com/cavaliercoder/go-cpio",
)
