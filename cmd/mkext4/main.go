package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/sushant12/machine/pkg/rootfs"
)

func main() {
	inputDir := flag.String("input", "", "Input directory containing the root filesystem")
	outputImage := flag.String("output", "rootfs.img", "Output ext4 image path")
	size := flag.Int("size", 0, "Size of the image in MB (optional, will be calculated if not specified)")
	flag.Parse()

	if *inputDir == "" {
		fmt.Println("Error: input directory is required")
		flag.Usage()
		os.Exit(1)
	}

	if err := rootfs.CreateExt4Image(*inputDir, *outputImage, *size); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
