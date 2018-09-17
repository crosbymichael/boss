package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/crosbymichael/boss/api/v1"
	"github.com/urfave/cli"
)

var nodesCommand = cli.Command{
	Name:  "nodes",
	Usage: "list boss nodes",
	Action: func(clix *cli.Context) error {
		ctx := Context()
		agent, err := Agent(clix)
		if err != nil {
			return err
		}
		defer agent.Close()
		resp, err := agent.Nodes(ctx, &v1.NodesRequest{})
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(os.Stdout, 10, 1, 3, ' ', 0)
		const tfmt = "%s\t%s\t%s\n"
		fmt.Fprint(w, "ID\tADDRESS\tLABELS\n")
		for _, n := range resp.Nodes {
			fmt.Fprintf(w, tfmt,
				n.ID,
				n.Address,
				joinLabels(n.Labels),
			)
		}
		return w.Flush()
	},
}

func joinLabels(labels map[string]string) string {
	var s []string
	for k, v := range labels {
		s = append(s, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(s, ",")
}
