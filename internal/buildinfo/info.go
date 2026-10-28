package buildinfo

import "runtime/debug"

const Version = "1.0.1"

type Dependency struct {
	Path    string
	Version string
}

func Dependencies() []Dependency {
	info, _ := debug.ReadBuildInfo()
	return dependencies(info)
}

func dependencies(info *debug.BuildInfo) []Dependency {
	if info == nil {
		return nil
	}
	deps := make([]Dependency, 0, len(info.Deps))
	for _, mod := range info.Deps {
		path, version := mod.Path, mod.Version
		if mod.Replace != nil {
			path += " => " + mod.Replace.Path
			version = mod.Replace.Version
			if version == "" {
				version = "(devel)"
			}
		}
		deps = append(deps, Dependency{Path: path, Version: version})
	}
	return deps
}
