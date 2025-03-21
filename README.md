# Machine

Spin Firecracker VMs

## Prerequisites

- Firecracker
- Go (for building the project)
- Enable `vhost/vsock` in Ubuntu:

    ```sh
    sudo modprobe vhost_vsock
    sudo modprobe vsock
    ```

    To check if it is enabled

    ```sh
    lsmod | grep vsock
    lsmod | grep vhost
    ls -l /dev/vsock /dev/vhost-vsock
    ```

   To ensure it is loaded on every reboot, add `vhost_vsock` to `/etc/modules`:

    ```sh
    echo vhost_vsock | sudo tee -a /etc/modules
    echo vsock | sudo tee -a /etc/modules
    ```

## Installation

1. Clone the repository:

    ```sh
    git clone https://github.com/sushant12/machine.git
    cd machine
    ```

2. Build the project:

    ```sh
    make build
    ```

## Configuring `sudo` to Allow Running Firecracker Without a Password

1. Open the `sudoers` file using `visudo`:

    ```sh
    sudo visudo
    ```

2. Add the following lines to allow your user to run the Firecracker binary and mkext4 without a password:

    ```sh
    yourusername ALL=(ALL) NOPASSWD: /path/to/bin/firecracker
    yourusername ALL=(ALL) NOPASSWD: /path/to/bin/mkext4
    ```

    Replace `yourusername` with your actual username, `/path/to/bin/firecracker` with the full path to the Firecracker binary, and `/path/to/bin/mkext4` with the full path to the mkext4 binary.

3. Save and close the `sudoers` file.

## Usage

1. Start the server:

    ```sh
    ./machine
    ```

2. Send a POST request to start a VM:

    ```sh
    curl -X POST -H "Content-Type: application/json" -d '{
        "config": {
            "init": {
                "exec": ["/bin/sleep", "inf"]
            },
            "auto_destroy": true,
            "image": "alpine:latest",
            "files": [
                {
                    "guest_path": "/main.sh",
                    "raw_value": "example-base64-encoded-value"
                }
            ],
            "guest": {
                "cpus": 2,
                "memory_mb": 2048
            }
        }
    }' http://localhost:8080/start-vm
    ```

## API Documentation

The API documentation is available through Swagger UI. After starting the server, you can access the documentation at:

- Swagger UI: `http://localhost:8080/swagger/index.html`

This provides an interactive interface to explore and test all available API endpoints.

## Makefile Targets

- `make all`: Run tests and build the project.
- `make test`: Run tests.
- `make build`: Build the project.
- `make release`: Clean and build the project.
- `make clean`: Remove the built binary.
- `make fmt`: Format the code.

## Testing

To run the tests:

```sh
make test
```

## License

This project is licensed under the MIT License.

