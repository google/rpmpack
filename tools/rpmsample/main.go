// rpmsample creates an rpm file with some known files, which
// can be used to test rpmpack's output against other rpm implementations.
// It is also an instructive example for using rpmpack.
package main

import (
	"log"
	"os"

	"github.com/google/rpmpack"
)

func main() {

	r, err := rpmpack.NewRPM(rpmpack.RPMMetaData{
		Name:    "rpmsample",
		Version: "0.1",
		Release: "A",
	})
	if err != nil {
		log.Fatal(err)
	}
	r.AddFile(
		rpmpack.RPMFile{
			Name: "./var/lib/rpmpack/sample.txt",
			Body: []byte("testsample\n"),
			Mode: 0600,
		})
	r.AddFile(
		rpmpack.RPMFile{
			Name: "./var/lib/rpmpack/sample2.txt",
			Body: []byte("testsample2\n"),
			Mode: 0600,
		})
	r.Write(os.Stdout)

}
