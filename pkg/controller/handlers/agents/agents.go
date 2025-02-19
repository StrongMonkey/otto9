package agents

import (
	"context"
	"fmt"

	"github.com/otto8-ai/nah/pkg/name"
	"github.com/otto8-ai/nah/pkg/router"
	"github.com/otto8-ai/otto8/apiclient/types"
	"github.com/otto8-ai/otto8/pkg/create"
	v1 "github.com/otto8-ai/otto8/pkg/storage/apis/otto.otto8.ai/v1"
	"github.com/otto8-ai/otto8/pkg/system"
	"k8s.io/apimachinery/pkg/api/equality"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func createWorkspace(ctx context.Context, c kclient.Client, agent *v1.Agent) error {
	if agent.Status.WorkspaceName != "" {
		return nil
	}

	ws := &v1.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  agent.Namespace,
			Name:       name.SafeConcatName(system.WorkspacePrefix, agent.Name),
			Finalizers: []string{v1.WorkspaceFinalizer},
		},
		Spec: v1.WorkspaceSpec{
			AgentName: agent.Name,
		},
	}
	if err := create.OrGet(ctx, c, ws); err != nil {
		return err
	}

	agent.Status.WorkspaceName = ws.Name
	return c.Status().Update(ctx, agent)
}

func createKnowledgeSet(ctx context.Context, c kclient.Client, agent *v1.Agent) error {
	if len(agent.Status.KnowledgeSetNames) > 0 {
		return nil
	}

	ks := &v1.KnowledgeSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  agent.Namespace,
			Name:       name.SafeConcatName(system.KnowledgeSetPrefix, agent.Name),
			Finalizers: []string{v1.KnowledgeSetFinalizer},
		},
		Spec: v1.KnowledgeSetSpec{
			AgentName: agent.Name,
		},
	}
	if err := create.OrGet(ctx, c, ks); err != nil {
		return err
	}

	agent.Status.KnowledgeSetNames = append(agent.Status.KnowledgeSetNames, ks.Name)
	return c.Status().Update(ctx, agent)
}

func CreateWorkspaceAndKnowledgeSet(req router.Request, _ router.Response) error {
	agent := req.Object.(*v1.Agent)

	if err := createWorkspace(req.Ctx, req.Client, agent); err != nil {
		return err
	}

	return createKnowledgeSet(req.Ctx, req.Client, agent)
}

func BackPopulateAuthStatus(req router.Request, _ router.Response) error {
	var updateRequired bool
	agent := req.Object.(*v1.Agent)

	var logins v1.OAuthAppLoginList
	if err := req.List(&logins, &kclient.ListOptions{
		FieldSelector: fields.SelectorFromSet(map[string]string{"spec.credentialContext": agent.Name}),
		Namespace:     agent.Namespace,
	}); err != nil {
		return err
	}

	for _, login := range logins.Items {
		if login.Status.External.Authenticated || (login.Status.External.Required != nil && !*login.Status.External.Required) || login.Spec.ToolReference == "" {
			continue
		}

		var required *bool
		credentialTool, err := v1.CredentialTool(req.Ctx, req.Client, agent.Namespace, login.Spec.ToolReference)
		if err != nil {
			login.Status.External.Error = fmt.Sprintf("failed to get credential tool for knowledge source [%s]: %v", agent.Name, err)
		} else {
			required = &[]bool{credentialTool != ""}[0]
			updateRequired = updateRequired || login.Status.External.Required == nil || *login.Status.External.Required != *required
			login.Status.External.Required = required
		}

		if required != nil && *required {
			var oauthAppLogin v1.OAuthAppLogin
			if err = req.Get(&oauthAppLogin, agent.Namespace, system.OAuthAppLoginPrefix+agent.Name+login.Spec.ToolReference); apierror.IsNotFound(err) {
				updateRequired = updateRequired || login.Status.External.Error != ""
				login.Status.External.Error = ""
			} else if err != nil {
				login.Status.External.Error = fmt.Sprintf("failed to get oauth app login for agent [%s]: %v", agent.Name, err)
			} else {
				updateRequired = updateRequired || equality.Semantic.DeepEqual(login.Status.External, oauthAppLogin.Status.External)
				login.Status.External = oauthAppLogin.Status.External
			}
		}

		if agent.Status.External.AuthStatus == nil {
			agent.Status.External.AuthStatus = make(map[string]types.OAuthAppLoginAuthStatus)
		}
		agent.Status.External.AuthStatus[login.Spec.ToolReference] = login.Status.External
	}

	if updateRequired {
		return req.Client.Status().Update(req.Ctx, agent)
	}

	return nil
}
