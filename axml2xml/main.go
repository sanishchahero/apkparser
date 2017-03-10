package main

import (
	"apkverifier"
	"binxml"
	"bufio"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/pprof"
	"shared"
	"strings"
)

func main() {
	isApk := flag.Bool("a", false, "The input file is an apk (default if INPUT is *.apk)")
	isManifest := flag.Bool("m", false, "The input file is an AndroidManifest.xml (default)")
	isResources := flag.Bool("r", false, "The input is resources.arsc file (default if INPUT is *.arsc)")
	verifyApk := flag.Bool("v", false, "Verify the file if it is an APK.")
	cpuProfile := flag.String("cpuprofile", "", "Write cpu profiling info")
	fileListPath := flag.String("l", "", "Process file list")

	flag.Parse()

	if *fileListPath == "" && len(flag.Args()) < 1 {
		fmt.Printf("%s INPUT\n", os.Args[0])
		os.Exit(1)
	}

	exitcode := 0
	defer func() {
		os.Exit(exitcode)
	}()

	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			exitcode = 1
			return
		}
		defer f.Close()

		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	if *fileListPath == "" {
		for _, input := range flag.Args() {
			if !processInput(input, *isApk, *isManifest, *isResources, *verifyApk) {
				exitcode = 1
			}
		}
	} else {
		f, err := os.Open(*fileListPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer f.Close()

		s := bufio.NewScanner(f)
		for s.Scan() {
			if !processInput(s.Text(), *isApk, *isManifest, *isResources, *verifyApk) {
				exitcode = 1
			}
		}
	}
}

func processInput(input string, isApk, isManifest, isResources, verifyApk bool) bool {
	var r io.Reader

	if !isApk && !isManifest && !isResources {
		if strings.HasSuffix(input, ".apk") {
			isApk = true
		} else if strings.HasSuffix(input, ".arsc") {
			isResources = true
		} else {
			isManifest = true
		}
	}

	if isApk {
		return processApk(input, verifyApk)
	} else {
		if input == "-" {
			r = os.Stdin
		} else {
			f, err := os.Open(input)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return false
			}
			defer f.Close()
			r = f
		}

		var err error
		if isManifest {
			enc := xml.NewEncoder(os.Stdout)
			enc.Indent("", "    ")

			err = binxml.ParseManifest(r, enc, nil)
		} else {
			_, err = binxml.ParseResourceTable(r)
		}

		fmt.Println()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return false
		}
	}
	return true
}

func processApk(input string, verify bool) bool {
	enc := xml.NewEncoder(os.Stdout)
	enc.Indent("", "    ")

	parser, err := shared.NewApkParser(0, input, enc)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return false
	}
	defer parser.Close()

	_, err = parser.ParseManifest()
	fmt.Println()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return false
	}

	if !verify {
		return true
	}

	fmt.Print("\n=====================================\n")

	res, err := apkverifier.Verify(input, parser.Zip())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return false
	}

	fmt.Println("Uses V2 signing scheme:", res.UsingSchemeV2)
	cinfo, cert := shared.PickBestApkCert(res.SignerCerts)
	if cinfo == nil {
		fmt.Fprintf(os.Stderr, "Failed to pick best cert from %v\n", res.SignerCerts)
		return false
	}

	fmt.Println()
	fmt.Println("validfrom:", cinfo.ValidFrom)
	fmt.Println("validto:", cinfo.ValidTo)
	fmt.Println("serialnumber:", cert.SerialNumber.Text(16))
	fmt.Println("thumbprint:", cinfo.Sha1)
	fmt.Println("thumbprint-sha256:", cinfo.Sha256)
	fmt.Printf("Subject:\n  %s\n", cinfo.Subject)
	fmt.Printf("Issuer:\n  %s\n", cinfo.Issuer)

	return true
}
