package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "boss"
	app.Version = "8"
	app.Usage = "container tools for my setup"
	app.Description = `

                    ___           ___           ___     
     _____         /\  \         /\__\         /\__\    
    /::\  \       /::\  \       /:/ _/_       /:/ _/_   
   /:/\:\  \     /:/\:\  \     /:/ /\  \     /:/ /\  \  
  /:/ /::\__\   /:/  \:\  \   /:/ /::\  \   /:/ /::\  \ 
 /:/_/:/\:|__| /:/__/ \:\__\ /:/_/:/\:\__\ /:/_/:/\:\__\
 \:\/:/ /:/  / \:\  \ /:/  / \:\/:/ /:/  / \:\/:/ /:/  /
  \::/_/:/  /   \:\  /:/  /   \::/ /:/  /   \::/ /:/  / 
   \:\/:/  /     \:\/:/  /     \/_/:/  /     \/_/:/  /  
    \::/  /       \::/  /        /:/  /        /:/  /   
     \/__/         \/__/         \/__/         \/__/    

run containers like a boss`
	app.Commands = []cli.Command{
		buildCommand,
		createCommand,
		deleteCommand,
		initCommand,
		killCommand,
		listCommand,
		networkCommand,
		rollbackCommand,
		startCommand,
		stopCommand,
		systemdCommand,
		upgradeCommand,
	}
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
