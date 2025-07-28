def _docker_run_impl(ctx):
    out = ctx.actions.declare_file(ctx.label.name)
    args = ctx.actions.args()
    args.add(out)
    args.add(ctx.attr.cmd)
    args.add(ctx.file.tar)
    args.add(ctx.attr.image)
    ctx.actions.run(
        outputs = [out],
        inputs = [ctx.file.tar],
        executable = ctx.file._script,
        arguments = [args],
        mnemonic = "DockerRun",
    )
    return DefaultInfo(files = depset([out]))

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
        "_script": attr.label(
            default = "//:docker_run.sh",
            allow_single_file = True,
        ),
    },
    implementation = _docker_run_impl,
)

def _diff_test_impl(ctx):
    ctx.actions.expand_template(
        template = ctx.file._template,
        output = ctx.outputs.file,
        substitutions = {
            "{RESULT}": ctx.file.result.short_path,
            "{GOLDEN}": ctx.attr.golden,
        },
    )
    return DefaultInfo(runfiles = ctx.runfiles(files = ctx.files.result))

diff_test_expand = rule(
    attrs = {
        "result": attr.label(
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

def docker_diff(name, cmd, golden, tar = ":rpms", image = "", base = ""):
    docker_run(
        name = name + "_run",
        testonly = True,
        tar = tar,
        image = image,
        cmd = cmd,
    )
    diff_test_expand(
        name = name + "_diff",
        result = ":%s_run" % name,
        golden = golden,
        testonly = True,
    )
    native.sh_test(
        name = name + "_diff_test",
        srcs = [":{}_diff".format(name)],
        data = [tar, ":{}_run".format(name)],
    )
