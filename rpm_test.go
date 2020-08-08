package rpmpack

import (
	"io/ioutil"
	"testing"
)

func TestFileOwner(t *testing.T) {
	r, err := NewRPM(RPMMetaData{})
	if err != nil {
		t.Fatalf("NewRPM returned error %v", err)
	}
	group := "testGroup"
	user := "testUser"

	r.AddFile(RPMFile{
		Name:  "/usr/local/hello",
		Body:  []byte("content of the file"),
		Group: group,
		Owner: user,
	})

	if err := r.Write(ioutil.Discard); err != nil {
		t.Errorf("NewRPM returned error %v", err)
	}
	if r.fileowners[0] != user {
		t.Errorf("File owner shoud be %s but is %s", user, r.fileowners[0])
	}
	if r.filegroups[0] != group {
		t.Errorf("File owner shoud be %s but is %s", group, r.filegroups[0])
	}
}

// https://github.com/google/rpmpack/issues/49
func Test100644(t *testing.T) {
	r, err := NewRPM(RPMMetaData{})
	if err != nil {
		t.Errorf("NewRPM returned error %v", err)
		t.FailNow()
	}
	r.AddFile(RPMFile{
		Name: "/usr/local/hello",
		Body: []byte("content of the file"),
		Mode: 0100644,
	})

	if err := r.Write(ioutil.Discard); err != nil {
		t.Errorf("Write returned error %v", err)
	}
	if r.filemodes[0] != 0100644 {
		t.Errorf("file mode want 0100644, got %o", r.filemodes[0])
	}
	if r.filelinktos[0] != "" {
		t.Errorf("linktos want empty (not a symlink), got %q", r.filelinktos[0])
	}

}
