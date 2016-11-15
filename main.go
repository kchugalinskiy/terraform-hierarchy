package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"

	log "github.com/Sirupsen/logrus"
	"github.com/vharitonsky/iniflags"
)

var (
	rootDir         = flag.String("dir", ".", "start dir")
	descriptionPath = flag.String("desc", "", "terraform markdown description")
	outPath         = flag.String("out", "", "output result filepath")
)

type Line struct {
	Name        string `form:"Name" json:"Name" xml:"Name"`
	Optional    bool   `form:"Optional" json:"Optional" xml:"Optional"`
	Description string `form:"Description" json:"Description" xml:"Description"`
}

type ResourceArgument Line
type ResourceAttribute Line

type Resource struct {
	Name       string              `form:"Name" json:"Name" xml:"Name"`
	Arguments  []ResourceArgument  `form:"Arguments" json:"Arguments" xml:"Arguments"`
	Attributes []ResourceAttribute `form:"Attributes" json:"Attributes" xml:"Attributes"`
}

func main() {
	iniflags.Parse()
	log.SetLevel(log.InfoLevel)
	log.Debug("reading directory: ", *rootDir)

	awsResources, err := loadResources(*descriptionPath)
	if err != nil {
		log.Error("error loading aws resources: ", err)
		return
	}

	state := NewHierarchyState()

	err = loadModule(*rootDir, ".", awsResources, state)

	if err != nil {
		log.Errorf("error reading root module '%s' (SKIPPED): %v", *rootDir, err)
	}

	jsonState, err := json.Marshal(*state)
	if nil != err {
		log.Error(err)
	}
	if "" != *outPath {
		err = ioutil.WriteFile(*outPath, jsonState, 0755)
		if nil != err {
			log.Errorf("writing to file (%s) error: %v", *outPath, err)
		}
	} else {
		fmt.Print(string(jsonState))
	}
}
