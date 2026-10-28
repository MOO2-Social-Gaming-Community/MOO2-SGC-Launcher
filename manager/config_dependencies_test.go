package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestConfigStatementsPreserveReferences(t *testing.T) {
	got, err := configStatements([]byte("# ignored include \\EXTRACT.CFG;\ninclude \"\\150\\build\\custom.txt\"; pregame_script = \"first.lua;second.lua\"; value = \"caret ^\" quote\";"))
	want := [][]string{{"include", `\150\build\custom.txt`}, {"pregame_script", "=", "first.lua;second.lua"}, {"value", "=", `caret " quote`}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, %v; want %#v", got, err, want)
	}
	for _, input := range []string{`include "unfinished;`, "include unfinished^"} {
		if _, err := configStatements([]byte(input)); err == nil {
			t.Fatalf("accepted unterminated input %q", input)
		}
	}
}

func TestConfigDependenciesFollowNestedNonCFGIncludes(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{}
	write := func(name, contents string) {
		t.Helper()
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
		files[strings.ToUpper(name)] = file
	}
	write("150/USER.CFG", `include "\150\build\EXTRA.TXT";`)
	write("150/build/EXTRA.TXT", `include \150\USER.CFG; # A cycle must terminate.`)
	if err := validateConfigDependencies(files); err != nil {
		t.Fatal(err)
	}
	write("150/build/EXTRA.TXT", `include \ORION2.LOG;`)
	if err := validateConfigDependencies(files); err == nil {
		t.Fatal("indirect excluded runtime output accepted")
	}
	write("150/build/EXTRA.TXT", `pregame_script = "safe.lua;\SAVE1.GAM";`)
	if err := validateConfigDependencies(files); err == nil {
		t.Fatal("excluded input inside quoted script list accepted")
	}
	write("150/build/EXTRA.TXT", strings.Repeat("#", (4<<20)+1))
	if err := validateConfigDependencies(files); err == nil {
		t.Fatal("oversized included input accepted")
	}
}

func TestConfigReferencesExpandKnownMacros(t *testing.T) {
	got, err := configReferenceCandidates("150/USER.CFG", `\SAVE$LANG_ID$.GAM`)
	if err != nil || len(got) != 5 {
		t.Fatal(got, err)
	}
	for _, name := range got {
		if !excludedConfigInput(name) {
			t.Fatalf("expanded save not recognized: %s", name)
		}
	}
	if _, err := configReferenceCandidates("150/USER.CFG", `C:\private.cfg`); err == nil {
		t.Fatal("drive path accepted")
	}
	for _, value := range []string{"", "0"} {
		if got, err := configReferenceCandidates("150/USER.CFG", value); err != nil || len(got) != 0 {
			t.Fatal(got, err)
		}
	}
}
