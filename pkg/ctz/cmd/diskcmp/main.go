package main

import (
	"fmt"
	"os"
	"strconv"
)

func usage() {
	println("diskcmp: compare contents of two block devices or files")
	println("  usage: diskcmp <blocksize> <dev1> <dev2>")
	os.Exit(1)
}

func fatal(label string, err error) {
	println(label, err)
	os.Exit(1)
}

func main() {
	argv := os.Args
	argc := len(argv)
	if argc < 4 {
		usage()
	}
	bsRaw := argv[1]
	bs, err := strconv.ParseInt(bsRaw, 10, 64)
	if err != nil {
		fatal(fmt.Sprintf("Invalid block size: %v", bsRaw), err)
	}
	dev1 := argv[2]
	dev2 := argv[3]

	file1, err := os.OpenFile(dev1, os.O_RDONLY, 0)
	if err != nil {
		fatal("Error opening first device:", err)
	}
	file2, err := os.OpenFile(dev2, os.O_RDONLY, 0)
	if err != nil {
		fatal("Error opening second device:", err)
	}
	stat1, err := file1.Stat()
	if err != nil {
		fatal("Error stat()ing first device:", err)
	}
	size1 := stat1.Size()
	stat2, err := file2.Stat()
	if err != nil {
		fatal("Error stat()ing second device:", err)
	}
	size2 := stat2.Size()

	if size1 != size2 {
		fmt.Printf("size mismatch: %d vs %d (difference of %d)\n", size1, size2, size2-size1)
	}

	effectiveSize := min(size1, size2)

	runfailed := false

	for i := int64(0); i < effectiveSize; i += bs {
		ebs := min(bs, effectiveSize-i)
		out1 := make([]byte, ebs)
		out2 := make([]byte, ebs)
		n1, err1 := file1.ReadAt(out1, i)
		n2, err2 := file2.ReadAt(out2, i)

		fail := false
		if err1 != nil {
			fmt.Printf("Error reading disk1 at offset %d: %v\n", i, err1)
			fail = true
			runfailed = true
		}
		if err2 != nil {
			fmt.Printf("Error reading disk2 at offset %d: %v\n", i, err1)
			fail = true
			runfailed = true
		}
		if int64(n1) != ebs {
			fmt.Printf("diskcmp: could not read all data from disk1: %d vs %d\n", n1, ebs)
			runfailed = true
		}
		if int64(n2) != ebs {
			fmt.Printf("diskcmp: could not read all data from disk2: %d vs %d\n", n2, ebs)
			runfailed = true
		}
		if n1 != n2 {
			fail = true
			runfailed = true
		}
		if !fail {
			mismatch := 0
			for j := 0; j < n1; j++ {
				if out1[j] != out2[j] {
					mismatch++
				}
			}
			if mismatch > 0 {
				fmt.Printf("diskcmp: data mismatch at %d: %d bytes different\n", i, mismatch)
				runfailed = true
			}
		}
	}

	if runfailed {
		println("One or more checks failed")
		os.Exit(1)
	}
}
