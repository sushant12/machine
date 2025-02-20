# Machine

Spin Firecracker VMs

## Prerequisites

- Docker
- Firecracker
- Go (for building the project)

## Installation

1. Clone the repository:

    ```sh
    git clone https://github.com/yourusername/machine.git
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

2. Add the following line to allow your user to run the Firecracker binary without a password:

    ```sh
    yourusername ALL=(ALL) NOPASSWD: /path/to/bin/firecracker
    ```

    Replace `yourusername` with your actual username and `/path/to/bin/firecracker` with the full path to the Firecracker binary.

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
                "cpu_kind": "shared",
                "cpus": 2,
                "memory_mb": 2048
            }
        }
    }' http://localhost:8080/start-vm
    ```

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

