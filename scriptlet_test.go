package rpmpack

import (
	"bytes"
	"testing"
)

func TestScriptletProgram(t *testing.T) {
	testCases := []struct {
		name        string
		scriptlet   string
		interpreter string
		ok          bool
	}{{
		name:        "interpreter only",
		scriptlet:   "",
		interpreter: "/bin/bash",
		ok:          true,
	}, {
		name:        "interpreter and scriptlet",
		scriptlet:   "-p/bin/bash\necho test",
		interpreter: "/bin/bash",
		ok:          true,
	}, {
		name:        "scriplet 1",
		scriptlet:   "-p/bin/bash",
		interpreter: "",
		ok:          true,
	}, {
		name:        "scriplet 2",
		scriptlet:   "-p  /bin/bash  ",
		interpreter: "",
		ok:          true,
	}, {
		name:        "scriplet 3",
		scriptlet:   "-p  /bin/bash   \necho foo ",
		interpreter: "",
		ok:          true,
	}, {
		name:        "whitespace before -p",
		scriptlet:   " -p /bin/bash",
		interpreter: "",
		ok:          true,
	},
		// expect to fail:
		{
			name:        "no -p argument",
			scriptlet:   " -p   ",
			interpreter: "",
			ok:          false,
		}, {
			name:        "whitespace",
			scriptlet:   "-p /bin /bash",
			interpreter: "",
			ok:          false,
		}, {
			name:        "CR",
			scriptlet:   "-p/bin/bash\recho this",
			interpreter: "",
			ok:          false,
		}, {
			name:        "newline escape",
			scriptlet:   "-p/bin/bash \\n echo this",
			interpreter: "",
			ok:          false,
		}, {
			name:        "no slash 1",
			scriptlet:   "-pbin/bash",
			interpreter: "",
			ok:          false,
		}, {
			name:        "no slash 2",
			scriptlet:   "-p bin/bash",
			interpreter: "",
			ok:          false,
		}}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			r, err := NewRPM(RPMMetaData{
				Name:        "test",
				Version:     "1.0",
				Release:     "1",
				Summary:     "test summary",
				Description: "test description",
				Licence:     "test license",
			})
			if err != nil {
				t.Fatalf("NewRPM returned error %v", err)
			}
			if tc.interpreter != "" {
				r.SetScriptletProgram(tc.interpreter)
			}
			if tc.scriptlet != "" {
				r.AddPrein(tc.scriptlet)
			}

			var b bytes.Buffer
			err = r.Write(&b)
			if tc.ok && err != nil {
				t.Errorf("Write returned error %v", err)
			}
			if !tc.ok && err == nil {
				t.Errorf("Write should have returned an error")
			}
		})
	}
}
