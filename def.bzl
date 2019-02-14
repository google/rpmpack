def _pkg_tar2rpm_impl(ctx):
    files = [ctx.file.data]
    args = ctx.actions.args()
    args.add("--name", ctx.attr.pkg_name)
    args.add("--version", ctx.attr.version)
    args.add("--release", ctx.attr.release)
    args.add("--file", ctx.outputs.out)
    args.add(ctx.file.data)
    ctx.actions.run(
        executable = ctx.executable.tar2rpm,
        arguments = [args],
        inputs = files,
        outputs = [ctx.outputs.out],
        mnemonic = "tar2rpm",
    )

# A rule for generating rpm files
pkg_tar2rpm = rule(
    implementation = _pkg_tar2rpm_impl,
    attrs = {
        "data": attr.label(mandatory = True, allow_single_file = [".tar"]),
        "pkg_name": attr.string(mandatory = True),
        "version": attr.string(mandatory = True),
        "release": attr.string(mandatory = True),
        "tar2rpm": attr.label(
            default = Label("//cmd/tar2rpm"),
            cfg = "host",
            executable = True,
        ),
    },
    outputs = {
        "out": "%{name}.rpm",
    },
)
