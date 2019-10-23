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
	"io"
	"log"
	"os"

	"github.com/google/rpmpack"
)

var (
	provides,
	obsoletes,
	suggests,
	recommends,
	requires,
	conflicts rpmpack.Relations
	name        = flag.String("name", "", "the package name")
	version     = flag.String("version", "", "the package version")
	release     = flag.String("release", "", "the rpm release")
	arch        = flag.String("arch", "noarch", "the rpm architecture")
	compressor  = flag.String("compressor", "gzip", "the rpm compressor")
	osName      = flag.String("os", "linux", "the rpm os")
	description = flag.String("description", "", "the rpm description")
	vendor      = flag.String("vendor", "", "the rpm vendor")
	packager    = flag.String("packager", "", "the rpm packager")
	group       = flag.String("group", "", "the rpm group")
	url         = flag.String("url", "", "the rpm url")
	licence     = flag.String("licence", "", "the rpm licence name")

	prein  = flag.String("prein", "", "prein scriptlet contents (not filename)")
	postin = flag.String("postin", "", "postin scriptlet contents (not filename)")
	preun  = flag.String("preun", "", "preun scriptlet contents (not filename)")
	postun = flag.String("postun", "", "postun scriptlet contents (not filename)")

	outputfile = flag.String("file", "", "write rpm to `FILE` instead of stdout")
)

func usage() {
	fmt.Fprintf(os.Stderr,
		`Usage:
  %s [OPTION] [FILE]
Options:
`, os.Args[0])
	flag.PrintDefaults()
}

func main() {
	flag.Var(&provides, "provides", "rpm provides values, can be just name or in the form of name=version (eg. bla=1.2.3)")
	flag.Var(&obsoletes, "obsoletes", "rpm obsoletes values, can be just name or in the form of name=version (eg. bla=1.2.3)")
	flag.Var(&suggests, "suggests", "rpm suggests values, can be just name or in the form of name=version (eg. bla=1.2.3)")
	flag.Var(&recommends, "recommends", "rpm recommends values, can be just name or in the form of name=version (eg. bla=1.2.3)")
	flag.Var(&requires, "requires", "rpm requires values, can be just name or in the form of name=version (eg. bla=1.2.3)")
	flag.Var(&conflicts, "conflicts", "rpm provides values, can be just name or in the form of name=version (eg. bla=1.2.3)")
	flag.Usage = usage
	flag.Parse()
	if *name == "" || *version == "" {
		fmt.Fprintln(os.Stderr, "name and version are required")
		flag.Usage()
		os.Exit(2)
	}

	var i io.Reader
	switch flag.NArg() {
	case 0:
		fmt.Fprintln(os.Stderr, "reading tar content from stdin.")
		i = os.Stdin
	case 1:
		f, err := os.Open(flag.Arg(0))
		if err != nil {
			log.Fatalf("Failed to open file %s for reading\n", flag.Arg(0))
		}
		i = f

	default:
		fmt.Fprintln(os.Stderr, "expecting 0 or 1 positional arguments")
		flag.Usage()
		os.Exit(2)
	}

	w := os.Stdout
	if *outputfile != "" {
		f, err := os.Create(*outputfile)
		if err != nil {
			log.Fatalf("Failed to open file %s for writing", *outputfile)
		}
		defer f.Close()
		w = f
	}
	r, err := rpmpack.FromTar(
		i,
		rpmpack.RPMMetaData{
			Name:        *name,
			Version:     *version,
			Release:     *release,
			Arch:        *arch,
			OS:          *osName,
			Vendor:      *vendor,
			Packager:    *packager,
			Group:       *group,
			URL:         *url,
			Licence:     *licence,
			Description: *description,
			Compressor:  *compressor,
			Provides:    provides,
			Obsoletes:   obsoletes,
			Suggests:    suggests,
			Recommends:  recommends,
			Requires:    requires,
			Conflicts:   conflicts,
		})
	r.AddPrein(*prein)
	r.AddPostin(*postin)
	r.AddPreun(*preun)
	r.AddPostun(*postun)

	if err != nil {
		fmt.Fprintf(os.Stderr, "tar2rpm error: %v\n", err)
		os.Exit(1)
	}
	if err := r.Write(w); err != nil {
		fmt.Fprintf(os.Stderr, "rpm write error: %v\n", err)
		os.Exit(1)
	}

}
