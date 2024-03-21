
def _docker_run_impl(ctx):
    ctx.actions.expand_template(
        template = ctx.file._template,
        output = ctx.outputs.file,
        is_executable = True,
        substitutions = {
            "{CMD}": ctx.attr.cmd,
            "{TAR}": ctx.file.tar.short_path,
            "{IMAGE}": ctx.attr.image,
        },
    )

docker_run = rule(
    attrs = {
        "tar": attr.label(
            mandatory = True,
            allow_single_file = True,
        ),
        "image": attr.string(
            mandatory = True,
        ),
        "cmd": attr.string(
            mandatory = True,
        ),
        "_template": attr.label(
            default = "//:docker_run.sh",
            allow_single_file = True,
        ),
    },
    outputs = {"file": "%{name}.sh"},
    implementation = _docker_run_impl,
)

def _diff_test_impl(ctx):
    ctx.actions.expand_template(
        template = ctx.file._template,
        output = ctx.outputs.file,
        substitutions = {
            "{CMD}": ctx.file.cmd.short_path,
            "{GOLDEN}": ctx.attr.golden,
        },
    )

diff_test_expand = rule(
    attrs = {
        "cmd": attr.label(
            mandatory = True,
            allow_single_file = True,
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

def docker_diff(name,cmd, golden, tar=":rpms", image="",  base=""):
    docker_run(
     name = name + "_run",
     testonly = True,
     tar = tar,
     image = image,
     cmd = cmd,
    )
    diff_test_expand(
        name = name + "_diff",
        cmd = ":%s_run" % name,
        golden = golden,
        testonly = True,
    )
    native.sh_test(
        name = name + "_diff_test",
        srcs = [":{}_diff".format(name)],
        data = [tar, ":{}_run".format(name)],
    )
