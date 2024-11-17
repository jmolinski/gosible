package config

import "testing"

func TestSetConfigVariableFromIniFile(t *testing.T) {
	mgr := Manager()
	defer DestroyDefaultManager()

	if mgr.Settings.COLOR_DEBUG != "dark gray" {
		t.Errorf("Expected %v, got %v", "dark gray", mgr.Settings.COLOR_DEBUG)
	}

	err := mgr.TryLoadConfigFile("tests/assets/debug_color.cfg")
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}

	if mgr.Settings.COLOR_DEBUG != "test" {
		t.Errorf("Expected %v, got %v", "test", mgr.Settings.COLOR_DEBUG)
	}
	if mgr.BaseDefs.COLOR_DEBUG != "dark gray" {
		t.Errorf("Expected %v, got %v", "dark gray", mgr.BaseDefs.COLOR_DEBUG)
	}
}

func TestStripIniInlineComments(t *testing.T) {
	if stripCommentAndSpaces("; aaaa") != "" {
		t.Errorf("Should strip lines starting with ;")
	}
	if stripCommentAndSpaces("# aaaa") != "" {
		t.Errorf("Should strip lines starting with #")
	}

	if stripCommentAndSpaces("haha # aaaa") != "haha # aaaa" {
		t.Errorf("Shouldn't accept # as inline comment prefix")
	}
	if stripCommentAndSpaces("haha ; aaaa") != "haha" {
		t.Errorf("Should strip inline comments starting with ;")
	}
	if stripCommentAndSpaces("haha; aaaa") != "haha; aaaa" {
		t.Errorf("Should require space before inline comment prefix")
	}
}
