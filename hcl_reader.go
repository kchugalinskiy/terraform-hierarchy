package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"

	log "github.com/Sirupsen/logrus"
	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
)

func loadModule(terraformRoot string, moduleRoot string, awsResources []Resource, state *HierarchyState) (*Module, error) {
	modulePath := filepath.Join(terraformRoot, moduleRoot)
	log.Debug("loading module: ", modulePath)

	module := state.NewModule(moduleRoot)

	files, err := ioutil.ReadDir(modulePath)
	if err != nil {
		return nil, fmt.Errorf("error reading directory: ", err)
	}

	for _, file := range files {

		if file.IsDir() {
			log.Debug(file.Name())

			log.Info("load module = ", filepath.Join(moduleRoot, file.Name()))
			childModule, err := loadModule(*rootDir, filepath.Join(moduleRoot, file.Name()), awsResources, state)

			if err != nil {
				log.Errorf("error reading file '%s' (SKIPPED): %v", file.Name(), err)
			}

			module.Submodules = append(module.Submodules, childModule)
		} else {
			moduleFile := filepath.Join(modulePath, file.Name())
			log.Info("moduleFile = ", moduleFile)
			_, err = loadModuleFile(module, moduleFile, awsResources, state)
			if err != nil {
				log.Errorf("error reading file '%s' (SKIPPED): %v", moduleFile, err)
			}
		}
	}
	return module, nil
}

func loadModuleFile(module *Module, filePath string, awsResources []Resource, state *HierarchyState) (*HierarchyState, error) {
	re := regexp.MustCompile(".*\\.tf")
	if !re.MatchString(filePath) {
		return state, nil
	}

	log.Debug("module file loading: ", filePath)

	bytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("module file loading (%s): %v", filePath, err)
	}

	hclFile, err := hcl.Parse(string(bytes))
	if err != nil {
		return nil, fmt.Errorf("module file loading (%s): unmarshalling from hcl: %v", filePath, err)
	}

	objects := hclFile.Node.(*ast.ObjectList)

	for _, objItem := range objects.Items {
		_, err = processModuleObject(module, objItem, awsResources, state)
		if nil != err {
			log.Warningf("module file loading (%s): error processing module object: %v", filePath, err)
		}
	}

	log.Debugf("module file loading (%s): loaded module: %+v", filePath, module)

	return state, nil
}

func processModuleObject(module *Module, object *ast.ObjectItem, awsResources []Resource, state *HierarchyState) (*HierarchyState, error) {
	var strKeys []string

	for _, key := range object.Keys {
		strKeys = append(strKeys, key.Token.Text)
	}

	if len(strKeys) < 2 {
		return nil, fmt.Errorf("process module object: wrong number of object keys (expected at least 2)")
	}

	switch strKeys[0] {
	case "variable":
		moduleInput := state.NewInput(module, unquote(strKeys[1]))
		moduleInput.IsLoaded = true
		log.Debug(moduleInput)
	case "output":
		moduleOutput := state.NewOutput(module, unquote(strKeys[1]))
		moduleOutput.IsLoaded = true
		log.Debug(moduleOutput)
	case "resource":
		processResource(module, object.Val.(*ast.ObjectType), Map(strKeys[1:], unquote), awsResources, state)
	case "module":
		//module instantiation
	default:
		log.Warning("process module object: unknown item type: ", strKeys[0])
	}

	return state, nil
}

func processResource(module *Module, object *ast.ObjectType, resourceName []string, awsResources []Resource, state *HierarchyState) {
	log.Debug("resource name = ", resourceName)

	if nil != object.List && nil != object.List.Items {
		for _, i := range object.List.Items {
			if len(i.Keys) != 1 {
				log.Error("process resource: wrong number of keys, expected 1")
				return
			}
			fieldResourceName := resourceName
			for _, k := range i.Keys {
				fieldResourceName = append(fieldResourceName, k.Token.Text)
			}

			switch value := i.Val.(type) {
			case *ast.LiteralType:
				log.Debug("Value = ", value.Token.Text)
				re := regexp.MustCompile("\\${var\\.([-a-zA-Z_]*)}")

				attributesMatched := re.FindStringSubmatch(value.Token.Text)
				if len(attributesMatched) != 2 {
					log.Debugf("process resource: wrong number of matches of regexp. match: %v", attributesMatched)
					continue
				}

				variableName := attributesMatched[1]

				if "" != variableName {
					awsArgument := getArgumentByName([]string{fieldResourceName[0], fieldResourceName[2]}, awsResources)

					state.ConnectInputToArgument(module, variableName, fieldResourceName, awsArgument)
				}
			default:
				log.Warningf("process resource: unsupported value type for resourceName: %v value: %+v", fieldResourceName, value)
			}
		}
	}
}

func loadResources(path string) ([]Resource, error) {
	var resources []Resource
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("resource loading: %v", err)
	}

	err = json.Unmarshal(bytes, &resources)
	if err != nil {
		return nil, fmt.Errorf("resource loading: error unmarshalling resources: %v", err)
	}

	return resources, nil
}

func getArgumentByName(argName []string, awsResources []Resource) *ResourceArgument {
	if len(argName) < 2 {
		log.Error("get argument by name: too short name")
		return nil
	}
	log.Debug("argName = ", argName)

	s1 := unquote(argName[0])
	s2 := unquote(argName[1])

	for _, res := range awsResources {
		if res.Name == s1 {
			for _, arg := range res.Arguments {
				if arg.Name == s2 {
					return &arg
				}
			}
			break
		}
	}
	return nil
}
