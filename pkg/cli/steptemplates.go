package cli

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/otto8-ai/otto8/apiclient"
	"github.com/otto8-ai/otto8/apiclient/types"
	"github.com/spf13/cobra"
)

type StepTemplates struct {
	root  *Otto
	Quiet bool `usage:"Only print IDs of tools" short:"q"`
}

func (l *StepTemplates) Customize(cmd *cobra.Command) {
	cmd.Use = "step-templates [flags]"
	cmd.Args = cobra.NoArgs
	cmd.Aliases = []string{"st", "steptemplate", "steptemplates", "step-template"}
}

func (l *StepTemplates) Run(cmd *cobra.Command, args []string) error {
	toolRefs, err := l.root.Client.ListToolReferences(cmd.Context(), apiclient.ListToolReferencesOptions{
		ToolType: types.ToolReferenceTypeStepTemplate,
	})
	if err != nil {
		return err
	}

	if l.Quiet {
		for _, toolRef := range toolRefs.Items {
			fmt.Println(toolRef.ID)
		}
		return nil
	}

	w := newTable("ID", "NAME", "DESCRIPTION", "PARAMS")
	for _, toolRef := range toolRefs.Items {
		desc := toolRef.Description
		if toolRef.Error != "" {
			desc = toolRef.Error
		}
		w.WriteRow(toolRef.ID, toolRef.Name, desc, strings.Join(slices.Sorted(maps.Keys(toolRef.Params)), ", "))
	}

	return w.Err()
}
