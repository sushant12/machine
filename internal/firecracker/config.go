package firecracker

import (
	firecracker "github.com/firecracker-microvm/firecracker-go-sdk"
	models "github.com/firecracker-microvm/firecracker-go-sdk/client/models"
)

type options struct {
	FcBinary        string
	FcKernelImage   string
	FcKernelCmdLine string
	FcRootDrivePath string
	FcNicConfig     []string
	FcLogFifo       string
	FcLogLevel      string
	FcMetricsFifo   string
	FcDisableSmt    bool
	FcCPUCount      int64
	FcCPUTemplate   string
	FcMemSz         int64
	FcFifoLogFile   string
	FcSocketPath    string
}

// Option is a function that configures an options instance
type Option func(*options)

// newOptions creates an options instance with default values
func newOptions(opts ...Option) *options {
	o := &options{
		FcBinary:        "bin/firecracker",
		FcKernelImage:   "bin//vmlinux",
		FcKernelCmdLine: "ro console=ttyS0 noapic reboot=k panic=1 pci=off nomodules",
		FcDisableSmt:    true,
		FcCPUCount:      1,
		FcMemSz:         256,
	}

	for _, opt := range opts {
		opt(o)
	}

	return o
}

func WithFcBinary(path string) Option {
	return func(o *options) {
		o.FcBinary = path
	}
}

func WithFcSocketPath(path string) Option {
	return func(o *options) {
		o.FcSocketPath = path
	}
}

// Converts options to a usable firecracker config
func (opts *options) getFirecrackerConfig() (firecracker.Config, error) {
	return firecracker.Config{
		SocketPath:      opts.FcSocketPath,
		LogFifo:         opts.FcLogFifo,
		LogLevel:        opts.FcLogLevel,
		MetricsFifo:     opts.FcMetricsFifo,
		KernelImagePath: opts.FcKernelImage,
		KernelArgs:      opts.FcKernelCmdLine,
		MachineCfg: models.MachineConfiguration{
			VcpuCount:  firecracker.Int64(opts.FcCPUCount),
			Smt:        firecracker.Bool(!opts.FcDisableSmt),
			MemSizeMib: firecracker.Int64(opts.FcMemSz),
		},
	}, nil
}
