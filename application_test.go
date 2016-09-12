package fire

import "testing"

func TestApplication(t *testing.T) {
	app := New("", "")

	p := DefaultPolicy()
	p.AllowMethodOverriding = true
	p.OverrideXFrameOptions = "DENY"

	app.EnableDevMode()
	app.SetPolicy(p)
	app.prepare()
}
