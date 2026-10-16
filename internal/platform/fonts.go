package platform

import "github.com/egoist/mygo/ui"

func Fonts() []string {
	families := ui.FontFamilies()
	names := make([]string, len(families))
	for i, family := range families {
		names[i] = family.Name
	}
	return names
}
