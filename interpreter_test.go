package rpmpack

import (
	"testing"
)

var (
	binTest  = "/bin/test"
	binother = "/bin/other"
	echo     = "echo"
	i1       = "i = 1"
)

func EnsureSetScriptletIs(
	t *testing.T,
	scriptlets explicitScriptlets,
	interpreter string,
	expectedInterpreter string,
	expectedContent string) int {

	haveInterpreter := scriptlets.scriptlets[interpreter].interpreter
	haveContent := scriptlets.scriptlets[interpreter].content

	if haveInterpreter != "" {
		if haveInterpreter != expectedInterpreter {
			t.Errorf("%s interpreter should be %q, got %q", interpreter, expectedInterpreter, haveInterpreter)
		}
		if expectedContent != haveContent {
			t.Errorf("%s content should be %q, got %q", interpreter, expectedContent, haveContent)
		}
		return 1
	} else {
		return 0
	}
}
func EnsureSetInterpretersAreAll(
	r *RPM,
	t *testing.T,
	expectedInterpreter string,
	expectedContent string,
	expectedTotal int) {

	scriptlets := r.implicitToExplicitScriptlets()
	total := 0
	for _, interpreter := range []string{
		"prein",
		"postin",
		"preun",
		"postun",
		"verifyscript",
	} {
		total += EnsureSetScriptletIs(t, scriptlets, interpreter, expectedInterpreter, expectedContent)
	}

	if total != expectedTotal {
		t.Errorf("Saw %d interpreters, but expected %d", total, expectedTotal)
	}
}

func EnsureSetLuaInterpretersAreAll(r *RPM, t *testing.T, expectedInterpreter string, expectedContent string, expectedTotal int) {
	scriptlets := r.implicitToExplicitScriptlets()
	total := 0
	for _, interpreter := range []string{
		"pretrans",
		"posttrans",
	} {
		total += EnsureSetScriptletIs(t, scriptlets, interpreter, expectedInterpreter, expectedContent)
	}
	if total != expectedTotal {
		t.Errorf("Saw %d interpreters, but expected %d", total, expectedTotal)
	}
}

func TestDefault(t *testing.T) {
	r, err := NewRPM(RPMMetaData{})
	if err != nil {
		t.Fatalf("NewRPM returned error %v", err)
	}

	EnsureSetInterpretersAreAll(r, t, DefaultScriptletInterpreter, "", 0)
	EnsureSetLuaInterpretersAreAll(r, t, MagicLuaMarker, "", 0)
}

func TestDefaultScriptletInterpreterWithoutContent(t *testing.T) {
	r, err := NewRPM(RPMMetaData{})
	if err != nil {
		t.Fatalf("NewRPM returned error %v", err)
	}

	r.SetDefaultScriptletInterpreter(binTest)
	EnsureSetInterpretersAreAll(r, t, binTest, "", 0)
	EnsureSetLuaInterpretersAreAll(r, t, "", "", 0)
}

func TestAllAdds(t *testing.T) {
	r, err := NewRPM(RPMMetaData{})
	if err != nil {
		t.Fatalf("NewRPM returned error %v", err)
	}

	r.AddPretrans(i1)
	r.AddPrein(echo)
	r.AddPostin(echo)
	r.AddPreun(echo)
	r.AddPostun(echo)
	r.AddPosttrans(i1)
	r.AddVerifyScript(echo)

	EnsureSetInterpretersAreAll(r, t, DefaultScriptletInterpreter, echo, 5)
	EnsureSetLuaInterpretersAreAll(r, t, MagicLuaMarker, i1, 2)
	r.SetDefaultScriptletInterpreter(binTest)
	EnsureSetInterpretersAreAll(r, t, binTest, echo, 5)
}

func TestAllSetInterpreterFor(t *testing.T) {
	r, err := NewRPM(RPMMetaData{})
	if err != nil {
		t.Fatalf("NewRPM returned error %v", err)
	}

	r.AddPretrans(i1)
	r.AddPrein(echo)
	r.AddPostin(echo)
	r.AddPreun(echo)
	r.AddPostun(echo)
	r.AddPosttrans(i1)
	r.AddVerifyScript(echo)

	for _, name := range []string{
		"pretrans",
		"prein",
		"postin",
		"preun",
		"postun",
		"posttrans",
		"verifyscript",
	} {
		r.SetScriptletInterpreterFor(name, binTest)
	}

	EnsureSetInterpretersAreAll(r, t, binTest, echo, 5)
	EnsureSetLuaInterpretersAreAll(r, t, binTest, i1, 2)
}

func TestDefaultScriptletInterpreter(t *testing.T) {
	r, err := NewRPM(RPMMetaData{})
	if err != nil {
		t.Fatalf("NewRPM returned error %v", err)
	}

	r.SetDefaultScriptletInterpreter(binTest)
	r.AddPostun(echo)
	r.AddPrein(echo)
	r.AddPretrans(i1)
	// Only set for non-lua scriptlets
	EnsureSetInterpretersAreAll(r, t, binTest, echo, 2)
	EnsureSetLuaInterpretersAreAll(r, t, "<lua>", i1, 1)
}

func TestDefaultScriptletInterpreterDoesNotResetSetScriptletInterpreterFor(t *testing.T) {
	r, err := NewRPM(RPMMetaData{})
	if err != nil {
		t.Fatalf("NewRPM returned error %v", err)
	}

	r.SetDefaultScriptletInterpreter(binTest)
	r.AddPrein(echo)
	r.SetScriptletInterpreterFor("prein", binother)
	r.SetDefaultScriptletInterpreter(binTest)
	r.AddPosttrans(i1)
	// The SetDefaultScriptletInterpreter does not undo the more specific SetScriptletInterpreterFor
	EnsureSetScriptletIs(t, r.implicitToExplicitScriptlets(), "prein", binother, echo)
	EnsureSetScriptletIs(t, r.implicitToExplicitScriptlets(), "posttrans", MagicLuaMarker, i1)
}

func TestOverrideLuaInterpreter(t *testing.T) {
	r, err := NewRPM(RPMMetaData{})
	if err != nil {
		t.Fatalf("NewRPM returned error %v", err)
	}

	r.AddPosttrans(i1)
	EnsureSetScriptletIs(t, r.implicitToExplicitScriptlets(), "posttrans", MagicLuaMarker, i1)
	r.SetScriptletInterpreterFor("posttrans", binTest)
	// Explicit setting of the interpreter for a lua scriptlet:
	EnsureSetScriptletIs(t, r.implicitToExplicitScriptlets(), "posttrans", binTest, i1)
	r.SetDefaultScriptletInterpreter("/foo/bar")
	// But not changed again by setting the default interpreter:
	EnsureSetScriptletIs(t, r.implicitToExplicitScriptlets(), "posttrans", binTest, i1)
}

func TestResetToDefaultInterpreter(t *testing.T) {
	r, err := NewRPM(RPMMetaData{})
	if err != nil {
		t.Fatalf("NewRPM returned error %v", err)
	}

	// verify: change this around
	r.AddVerifyScript(echo)
	// prein: only modify via SetDefaultScriptletInterpreter. Check that it obeys these settings.
	r.AddPrein(echo)
	r.SetDefaultScriptletInterpreter(binTest)
	EnsureSetScriptletIs(t, r.implicitToExplicitScriptlets(), "verifyscript", binTest, echo)
	EnsureSetScriptletIs(t, r.implicitToExplicitScriptlets(), "prein", binTest, echo)
	r.SetDefaultScriptletInterpreter("")
	EnsureSetScriptletIs(t, r.implicitToExplicitScriptlets(), "verifyscript", DefaultScriptletInterpreter, echo)
	EnsureSetScriptletIs(t, r.implicitToExplicitScriptlets(), "prein", DefaultScriptletInterpreter, echo)
	r.SetScriptletInterpreterFor("verifyscript", binTest)
	EnsureSetScriptletIs(t, r.implicitToExplicitScriptlets(), "verifyscript", binTest, echo)
	EnsureSetScriptletIs(t, r.implicitToExplicitScriptlets(), "prein", DefaultScriptletInterpreter, echo)
	r.SetDefaultScriptletInterpreter("")
	EnsureSetScriptletIs(t, r.implicitToExplicitScriptlets(), "verifyscript", binTest, echo)
	EnsureSetScriptletIs(t, r.implicitToExplicitScriptlets(), "prein", DefaultScriptletInterpreter, echo)
}

func TestInvalidInterpreter(t *testing.T) {
	r, err := NewRPM(RPMMetaData{})
	if err != nil {
		t.Fatalf("NewRPM returned error %v", err)
	}

	if err := r.SetScriptletInterpreterFor("mistake", binTest); err == nil {
		t.Fatalf("SetScriptletInterpreterFor with invalid name should return an error")
	}
}
