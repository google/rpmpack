// rpmsample creates an rpm file with some known files, which
// can be used to test rpmpack's output against other rpm implementations.
// It is also an instructive example for using rpmpack.
package main

import (
	"os"

	"github.com/google/rpmpack"
)

func main() {

	r := rpmpack.NewRPM(rpmpack.RPMMetaData{
		Name:    "rpmsample",
		Version: "0.1",
		Release: "A",
	})
	r.AddFile(
		rpmpack.RPMFile{
			Name: "./var/lib/rpmpack/sample.txt",
			Body: []byte("testsample\n"),
		})
	r.Write(os.Stdout)

}
