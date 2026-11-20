package launcher

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"my-quickstart-go/internal/model"
	"my-quickstart-go/internal/platform"
)

type entryDrag struct{ ID string }

func (a *App) filter() {
	a.results = a.results[:0]
	a.matches = make(map[string]int, len(a.state.Groups))
	q := strings.ToLower(strings.TrimSpace(a.query))
	firstMatch := ""
	for _, g := range a.state.Groups {
		if q == "" {
			a.matches[g.ID] = len(g.Apps)
		} else {
			for _, e := range g.Apps {
				if strings.Contains(strings.ToLower(e.Name), q) {
					a.matches[g.ID]++
				}
			}
		}
		if firstMatch == "" && a.matches[g.ID] > 0 {
			firstMatch = g.ID
		}
	}
	if q != "" {
		if a.filteredQuery == "" {
			a.browseGroupID = a.state.ActiveGroupID
		}
		if firstMatch != "" && (q != a.filteredQuery || a.matches[a.state.ActiveGroupID] == 0) {
			a.state.ActiveGroupID = firstMatch
			a.selected = 0
		}
	} else if a.filteredQuery != "" {
		if a.state.Group(a.browseGroupID) != nil {
			a.state.ActiveGroupID = a.browseGroupID
		}
		a.browseGroupID = ""
	}
	a.filteredQuery = q
	if g := a.state.Group(a.state.ActiveGroupID); g != nil {
		for _, e := range g.Apps {
			if q == "" || strings.Contains(strings.ToLower(e.Name), q) {
				a.results = append(a.results, e)
			}
		}
	}
	a.selected = max(0, min(a.selected, len(a.results)-1))
}

func (a *App) chooseGroup(id string) {
	if a.state.Group(id) == nil || (strings.TrimSpace(a.query) != "" && a.matches[id] == 0) {
		return
	}
	a.state.ActiveGroupID = id
	a.selected = 0
	a.changed()
}

func (a *App) addGroup() {
	count := 0
	for _, existing := range a.state.Groups {
		if strings.HasPrefix(existing.Name, "Group-") {
			count++
		}
	}
	g := model.Group{ID: model.ID(), Name: fmt.Sprint("Group-", count+1), Apps: []model.Entry{}}
	a.state.Groups = append(a.state.Groups, g)
	a.state.ActiveGroupID = g.ID
	a.changed()
}

func (a *App) removeGroup(id string) {
	if len(a.state.Groups) <= 1 {
		return
	}
	a.state.Groups = slices.DeleteFunc(a.state.Groups, func(g model.Group) bool { return g.ID == id })
	a.changed()
}

func (a *App) removeEntry(id string) {
	g, e := a.state.Entry(id)
	if e == nil {
		return
	}
	g.Apps = slices.DeleteFunc(g.Apps, func(e model.Entry) bool { return e.ID == id })
	a.changed()
}

func (a *App) moveEntry(id, groupID, beforeID string) {
	if id == beforeID {
		return
	}
	src, e := a.state.Entry(id)
	dst := a.state.Group(groupID)
	if src == nil || e == nil || dst == nil {
		return
	}
	entry := *e
	src.Apps = slices.DeleteFunc(src.Apps, func(e model.Entry) bool { return e.ID == id })
	dst = a.state.Group(groupID)
	at := len(dst.Apps)
	if beforeID != "" {
		for i, e := range dst.Apps {
			if e.ID == beforeID {
				at = i
				break
			}
		}
	}
	dst.Apps = slices.Insert(dst.Apps, at, entry)
	a.changed()
}

func (a *App) addEntries(groupID, beforeID string, entries []model.Entry) int {
	g := a.state.Group(groupID)
	if g == nil {
		return 0
	}
	at := 0
	if beforeID == "$end" {
		at = len(g.Apps)
	} else if beforeID != "" {
		at = len(g.Apps)
		for i, e := range g.Apps {
			if e.ID == beforeID {
				at = i
				break
			}
		}
	}
	exists := map[string]bool{}
	for _, e := range g.Apps {
		exists[e.Path+"\x00"+e.Args] = true
	}
	var added []model.Entry
	for _, e := range entries {
		key := e.Path + "\x00" + e.Args
		if exists[key] {
			continue
		}
		exists[key] = true
		if e.ID == "" {
			e.ID = model.ID()
		}
		added = append(added, e)
	}
	if len(added) > 0 {
		g.Apps = slices.Insert(g.Apps, at, added...)
		a.changed()
	}
	return len(added)
}

func (a *App) importPaths(paths []string, groupID, beforeID string) {
	if len(paths) == 0 {
		return
	}
	relative := a.state.Settings.UseRelativePath
	go func() {
		var entries []model.Entry
		var errs []error
		for _, path := range paths {
			e, err := platform.Resolve(path, relative)
			if err != nil {
				errs = append(errs, err)
			} else {
				entries = append(entries, e)
			}
		}
		a.window.Update(func() {
			n := a.addEntries(groupID, beforeID, entries)
			if len(errs) > 0 {
				a.report(errors.Join(errs...))
			} else {
				a.message = a.text("已添加 ", "Added ") + fmt.Sprint(n)
			}
		})
	}()
}
