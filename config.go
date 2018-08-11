package main

import "github.com/crosbymichael/boss/config"

// this is the dumb version that requires the task to be bumped when it changes
func writeConfigs(id string, files map[string]config.File) error {
	for name, file := range files {
		uri, err := file.Source.Parse()
		if err != nil {
			return err
		}
	}
}
