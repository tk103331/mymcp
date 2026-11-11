package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestLanguageSelectAppliesImmediately(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	a := newTestApp()
	a.settings.Language = "system"
	a.router.Transition = ui.TransitionNone
	a.router.Push("/settings")
	tt := ui.NewTester(a.view, 1000, 800)
	tt.SetLocale("zh-Hans")
	a.languageChoice, a.themeChoice = "", ""
	tt.Frame()
	if err := tt.Click("跟随系统"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("English"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.settings.Language != "en" {
		t.Fatalf("language = %q, want en", a.settings.Language)
	}
	if !tt.HasText("English") {
		t.Fatalf("select did not show English: %q", tt.Texts())
	}
}
