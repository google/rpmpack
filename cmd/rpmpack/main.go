// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/google/rpmpack"
)

var (
	name    = flag.String("name", "rpmsample", "the package name")
	version = flag.String("version", "0", "the package version")
	release = flag.String("release", "0", "the rpm release")

	outputfile = flag.String("file", "", "write rpm to `FILE` instead of stdout")

	owner    = flag.String("owner", "root", "use `NAME` as owner")
	group    = flag.String("group", "root", "use `NAME` as group")
	filemode = flag.String("filemode", "0644", "octal mode of files. Setting to 0 will read the permission bits from the files.")
	dirmode  = flag.String("dirmode", "0755", "octal mode of dirs. Setting to 0 will read the permission bits from the dirs.")
	mtime    = flag.Uint("mtime", 0, "change timestamp of files")
)

func usage() {
	fmt.Fprintf(os.Stderr,
		`Usage:
  %s [OPTION] [FILE]...
Options:
`, os.Args[0])
	flag.PrintDefaults()
}

func main() {
	flag.Usage = usage
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}
	fmode := parseOctFlag(*filemode)
	dmode := parseOctFlag(*dirmode)

	w := os.Stdout
	if *outputfile != "" {
		f, err := os.Create(*outputfile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open file %s for writing", *outputfile)
		}
		defer f.Close()
		w = f
	}
	r, err := rpmpack.FromFiles(
		flag.Args(),
		rpmpack.RPMMetaData{
			Name:    *name,
			Version: *version,
			Release: *release,
		},
		rpmpack.Opts{
			Owner:    *owner,
			Group:    *group,
			FileMode: fmode,
			DirMode:  dmode,
			Mtime:    *mtime,
		})
	if err != nil {
		fmt.Fprintf(os.Stderr, "rpmpack error: %v\n", err)
		os.Exit(1)
	}
	if err := r.Write(w); err != nil {
		fmt.Fprintf(os.Stderr, "rpm write error: %v\n", err)
		os.Exit(1)
	}

}
func parseOctFlag(v string) uint {
	var m uint
	if v != "" {
		m64, err := strconv.ParseInt(v, 8, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to parse mode %s as octal", v)
			flag.Usage()
			os.Exit(2)
		}
		m = uint(m64)
	}
	return m
}
