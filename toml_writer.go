package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/pelletier/go-toml"
)

/////////////////////////////////////////////////////////////////////////////////////
// write
func WriteFile(state *HierarchyState) {
	if nil == state {
		log.Error("nil state")
		return
	}

	tomlTree := toml.TreeFromMap(make(map[string]interface{}))
	log.Infof("%+v", tomlTree)
}
