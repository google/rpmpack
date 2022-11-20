load("@io_bazel_rules_docker//container:container.bzl", "container_image")

def _diff_test_impl(ctx):
    ctx.actions.expand_template(
        template = ctx.file._template,
        output = ctx.outputs.file,
        substitutions = {
            "{CMD}": ctx.executable.cmd.short_path,
            "{GOLDEN}": ctx.attr.golden,
        },
    )

diff_test_expand = rule(
    attrs = {
        "cmd": attr.label(
            mandatory = True,
            allow_single_file = True,
            executable = True,
            cfg = "exec",
        ),
        "golden": attr.string(
            mandatory = True,
        ),
        "_template": attr.label(
            default = "//:diff_test.sh",
            allow_single_file = True,
        ),
    },
    outputs = {"file": "%{name}.sh"},
    implementation = _diff_test_impl,
)

def docker_diff(name, base, cmd, golden):
    container_image(
        name = name,
        testonly = True,
        base = base,
        cmd = cmd,
        legacy_run_behavior = False,
    )
    diff_test_expand(
        name = name + "_diff",
        cmd = ":%s" % name,
        golden = golden,
        testonly = True,
    )
    native.sh_test(
        name = name + "_diff_test",
        srcs = [":%s_diff" % name],
        data = [":%s" % name],
    )
