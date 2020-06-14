package rpmpack

import (
	"io/ioutil"
	"testing"
)

func TestFileOwner(t *testing.T) {
	r, err := NewRPM(RPMMetaData{})
	if err != nil {
		t.Errorf("NewRpm returned error %v", err)
		t.FailNow()
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
		t.Errorf("NewRpm returned error %v", err)
	}
	if r.fileowners[0] != user {
		t.Errorf("File owner shoud be %s but is %s", user, r.fileowners[0])
	}
	if r.filegroups[0] != group {
		t.Errorf("File owner shoud be %s but is %s", group, r.filegroups[0])
	}
}
