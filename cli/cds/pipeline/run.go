package pipeline

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/ovh/cds/sdk"
)

var cmdPipelineRunArguments []string
var batch bool
var env string
var parentInfo string
var parentBuildNumber int64

func pipelineRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "run",
		Short:   "cds pipeline run <projectKey> <appName> <pipelineName> <envName>",
		Long:    ``,
		Aliases: []string{"fire", "leeeeeeeeeeeeeroy"},
		Run:     runPipeline,
	}

	cmd.Flags().BoolVarP(&batch, "batch", "", false, "Do not stream logs")
	cmd.Flags().StringSliceVarP(&cmdPipelineRunArguments, "parameter", "p", nil, "Pipeline parameters")
	cmd.Flags().StringVarP(&parentInfo, "parent", "", "", "Parent build (format: app/pip[/env])")
	cmd.Flags().Int64VarP(&parentBuildNumber, "parent-build", "", 0, "Parent build number")

	return cmd
}

func runPipeline(cmd *cobra.Command, args []string) {
	var params []sdk.Parameter

	if len(args) < 3 {
		sdk.Exit("Wrong usage: see %s\n", cmd.Short)
	}

	projectKey := args[0]
	appName := args[1]
	name := args[2]

	var envName = ""
	if len(args) > 3 {
		envName = args[3]
	}

	pipelineArgs := cmdPipelineRunArguments
	stream := true
	if batch {
		stream = false
	}

	var ppipID, pappID, penvID int64
	// if parent info is provided, fetch ids
	if parentInfo != "" {
		penvID = 1
		t := strings.Split(parentInfo, "/")
		if len(t) < 2 || len(t) > 3 {
			sdk.Exit("Error: parent-info should be 'application/pipeline[/env]")
		}
		app, err := sdk.GetApplication(projectKey, t[0])
		if err != nil {
			sdk.Exit("Error: Application '%s' not found (%s)\n", t[0], err)
		}
		pappID = app.ID
		pip, err := sdk.GetPipeline(projectKey, t[1])
		if err != nil {
			sdk.Exit("Error: Pipeline '%s' not found (%s)\n", t[1], err)
		}
		ppipID = pip.ID
		// if environment is provided, fetch it
		penv := "NoEnv"
		if len(t) == 3 {
			env, err := sdk.GetEnvironment(projectKey, t[2])
			if err != nil {
				sdk.Exit("Error: Environment '%s' not found (%s)\n", t[2], err)
			}
			penvID = env.ID
			penv = env.Name
		}

		// OK now get parameters
		var found bool
		trs, err := sdk.GetTriggersAsSource(projectKey, t[0], t[1], penv)
		if err != nil {
			sdk.Exit("Error: Cannot fetch trigger parameters (%s)\n", err)
		}
		for _, t := range trs {
			if t.DestApplication.Name == appName && t.DestPipeline.Name == name && t.DestEnvironment.Name == envName {
				params = t.Parameters
				found = true
				break
			}
		}
		if !found {
			sdk.Exit("Error: link between parent and child not found\n")
		}

		// parent info provided, but not build number, fetch last build
		if parentBuildNumber == 0 {
			pb, err := sdk.GetPipelineBuildStatus(projectKey, t[0], t[1], penv, 0)
			if err != nil {
				sdk.Exit("Error: Cannot fetch last build number for %s/%s/%s[%s] (%s)\n",
					projectKey, t[0], t[1], penv, err)
			}
			parentBuildNumber = pb.BuildNumber
		}
	}

	for _, elt := range pipelineArgs {
		argSplitted := strings.SplitN(elt, "=", 2)
		if len(argSplitted) == 2 {
			p := sdk.Parameter{
				Name:  argSplitted[0],
				Value: argSplitted[1],
			}
			params = append(params, p)
		} else {
			sdk.Exit("Error: malformed parameter '%s' (must be format 'name=value')\n", elt)
		}
	}

	r := sdk.RunRequest{
		Params:              params,
		ParentBuildNumber:   parentBuildNumber,
		ParentPipelineID:    ppipID,
		ParentApplicationID: pappID,
		ParentEnvironmentID: penvID,
	}

	ch, err := sdk.RunPipeline(projectKey, appName, name, envName, stream, r, false)
	if err != nil {
		sdk.Exit("Error: %s\n", err)
	}

	if batch {
		return
	}

	streamResponse(ch)
}

func streamResponse(ch chan sdk.Log) {
	w := tabwriter.NewWriter(os.Stdout, 27, 1, 2, ' ', 0)
	titles := []string{"DATE", "ACTION", "LOG"}
	fmt.Fprintln(w, strings.Join(titles, "\t"))

	for l := range ch {
		fmt.Fprintf(w, "%s\t%s\t%s",
			[]byte(l.Timestamp.String())[:19],
			l.Step,
			l.Value,
		)
		w.Flush()

		// Exit 1 if pipeline fail
		if l.ID == 0 && strings.Contains(l.Value, "status: Fail") {
			sdk.Exit("")
		}
	}
}
