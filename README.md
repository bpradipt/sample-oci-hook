# Introduction
Sample OCI runtime hook for Raksh.

More details on hooks are available [here.](https://github.com/opencontainers/runtime-spec/blob/master/config.md#posix-platform-hooks)

# Installation
The hook config json needs to be placed in the platform hook config directory. Example configuration paths:
- /usr/share/containers/oci/hooks.d
- /etc/containers/oci/hooks.d


# Building

```sh
go build -o bin/hook hook.go
```

# Using it with Kata Containers

1. Ensure `guest_hook_path` is set to `/usr/share/oci/hooks` in kata containers `configuration.toml` file
2. Copy the `hook` binary to the Kata agent initrd under the following location `${ROOTFS_DIR}/usr/share/oci/hooks/prestart`

    Instructions to build a custom Kata agent is described [here](https://github.com/kata-containers/documentation/blob/master/Developer-Guide.md#create-and-install-rootfs-and-initrd-image)






