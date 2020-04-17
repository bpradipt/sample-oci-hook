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

1. Ensure `guest_hook_path` is set to `/usr/share/oci/hooks` in kata containers `configuration.toml` file.
   Additionally also set `kernel_params = "agent.debug_console"` which will allow access to the hook logs inside the Kata VM
2. Copy the `hook` binary to the Kata agent initrd under the following location `${ROOTFS_DIR}/usr/share/oci/hooks/prestart`

    Instructions to build a custom Kata agent is described [here](https://github.com/kata-containers/documentation/blob/master/Developer-Guide.md#create-and-install-rootfs-and-initrd-image)

    For quick experimentation, you can find a sample kata initrd for amd64 in the following container image `bpradipt/kata-initrd-hook`

    In order to extract the initrd you can use the following steps:

    ```sh
    CID=`podman create bpradipt/kata-initrd-hook bash`
    podman cp $CID:kata-containers-initrd-hook.img /usr/share/kata-containers/kata-containers-initrd-hook.img
    ```
    Update the `initrd` location in `configuration.toml` file


3. Deploy container with encrypted data. Location for decrypted data being `/etc/raksh`.

    ```sh
    kubectl apply -f examples/sample-nginx.yaml
    ```

4. Exec a shell inside the container and check the mount points

    ```sh
    kubectl exec -it nginx

    root@nginx:~# mount
    [snip]

    tmpfs on /etc/raksh type tmpfs (rw,relatime)

    [snip]

    root@nginx:~# ls -l /etc/raksh

    -rw-r--r-- 1 root root 200 Apr 17 11:28 raksh.properties

    ```

5. Access the hook logs

    Get the console.sock file path for the Kata VM. It's part of the Qemu argument

    ```sh
    ps aux | grep qemu
    ```
    Look for the console.sock entry which will be of the following format: `/run/vc/vm/<UUID>/console.sock`

    Connect to the console
    ```sh
    socat stdin,raw,echo=0,escape=0x11 unix-connect:"<path_to_console.sock>"
    ```

    Log files are under `/tmp`


