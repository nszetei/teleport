/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resources

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/types"
	resourcesv5 "github.com/gravitational/teleport/operator/apis/resources/v5"
	"github.com/gravitational/trace"
)

func TestRoleCreation(t *testing.T) {
	ctx := context.Background()

	teleportServer, operatorName := defaultTeleportServiceConfig(t)

	require.NoError(t, teleportServer.Start())

	tClient := clientForTeleport(t, teleportServer, operatorName)
	k8sClient := startKubernetesOperator(t, tClient)

	ns := createNamespaceForTest(t, k8sClient)
	roleName := validRandomResourceName("role-")

	// The role is created in K8S
	k8sCreateDummyRole(ctx, t, k8sClient, ns.Name, roleName)

	fastEventually(t, func() bool {
		tRole, err := tClient.GetRole(ctx, roleName)
		if trace.IsNotFound(err) {
			return false
		}
		require.NoError(t, err)

		require.Equal(t, tRole.GetName(), roleName)

		require.Contains(t, tRole.GetMetadata().Labels, types.OriginLabel)
		require.Equal(t, tRole.GetMetadata().Labels[types.OriginLabel], types.OriginCloud)

		return true
	})

	// The role is deleted in K8S
	k8sDeleteRole(ctx, t, k8sClient, roleName, ns.Name)

	fastEventually(t, func() bool {
		_, err := tClient.GetRole(ctx, roleName)
		return trace.IsNotFound(err)
	})
}

func TestRoleUpdate(t *testing.T) {
	ctx := context.Background()

	teleportServer, operatorName := defaultTeleportServiceConfig(t)

	require.NoError(t, teleportServer.Start())

	tClient := clientForTeleport(t, teleportServer, operatorName)
	k8sClient := startKubernetesOperator(t, tClient)

	ns := createNamespaceForTest(t, k8sClient)
	roleName := validRandomResourceName("role-")

	// The role does not exist in K8S
	var r resourcesv5.Role
	err := k8sClient.Get(ctx, kclient.ObjectKey{
		Namespace: ns.Name,
		Name:      roleName,
	}, &r)
	require.True(t, kerrors.IsNotFound(err))

	// The role is created in Teleport
	tRole, err := types.NewRole(roleName, types.RoleSpecV5{
		Allow: types.RoleConditions{
			Logins: []string{"a", "b"},
		},
	})
	require.NoError(t, err)

	err = tClient.UpsertRole(ctx, tRole)
	require.NoError(t, err)

	// The role is created in K8S
	k8sRole := resourcesv5.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: ns.Name,
		},
		Spec: resourcesv5.RoleSpec{
			Allow: types.RoleConditions{
				Logins: []string{"x", "z"},
			},
		},
	}
	k8sCreateRole(ctx, t, k8sClient, &k8sRole)

	// The role is updated in Teleport
	fastEventually(t, func() bool {
		tRole, err := tClient.GetRole(ctx, roleName)
		require.NoError(t, err)

		// Role was updated with new roles
		if !assert.ElementsMatch(t, tRole.GetLogins(types.Allow), []string{"x", "z"}) {
			return false
		}

		// Role does not have the Origin Label
		require.NotEqual(t, tRole.GetMetadata().Labels[types.OriginLabel], types.OriginCloud)
		return true
	})

	// Updating the role in K8S
	var k8sRoleNewVersion resourcesv5.Role
	err = k8sClient.Get(ctx, kclient.ObjectKey{
		Namespace: ns.Name,
		Name:      roleName,
	}, &k8sRoleNewVersion)
	require.NoError(t, err)

	k8sRoleNewVersion.Spec.Allow.Logins = append(k8sRoleNewVersion.Spec.Allow.Logins, "admin", "root")
	err = k8sClient.Update(ctx, &k8sRoleNewVersion)
	require.NoError(t, err)

	// Updates the role in Teleport
	fastEventually(t, func() bool {
		tRole, err := tClient.GetRole(ctx, roleName)
		require.NoError(t, err)

		// Role updated with new roles
		return assert.ElementsMatch(t, tRole.GetLogins(types.Allow), []string{"x", "z", "admin", "root"})
	})
}

func k8sCreateDummyRole(ctx context.Context, t *testing.T, kc kclient.Client, namespace, roleName string) {
	role := resourcesv5.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: namespace,
		},
		Spec: resourcesv5.RoleSpec{
			Allow: types.RoleConditions{
				Logins: []string{"a", "b"},
			},
		},
	}
	k8sCreateRole(ctx, t, kc, &role)
}

func k8sDeleteRole(ctx context.Context, t *testing.T, kc kclient.Client, roleName, namespace string) {
	role := resourcesv5.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: namespace,
		},
	}
	err := kc.Delete(ctx, &role)
	require.NoError(t, err)
}

func k8sCreateRole(ctx context.Context, t *testing.T, kc kclient.Client, role *resourcesv5.Role) {
	err := kc.Create(ctx, role)
	require.NoError(t, err)
}
