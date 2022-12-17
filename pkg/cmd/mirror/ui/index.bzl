load("@build_bazel_rules_nodejs//:index.bzl", "js_library", "npm_package_bin", "nodejs_binary")

def yarn_lock_to_json(name, yarn_lock, **kwargs):
    """Runs @cockroachlabs/yarn-lock-to-json on the provided yarn_lock file,
       storing the produced output in "__${name}".
    """
    npm_package_bin(
        name = name,
        tool = ":yarn-lock-to-json",
        stdout = "__" + name,
        data = [
            yarn_lock,
        ],
        args = [
            "$(execpath {})".format(yarn_lock),
        ],
    )
