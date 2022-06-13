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

package tbot

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/tbot/testhelpers"
	"github.com/gravitational/teleport/lib/utils"
	libutils "github.com/gravitational/teleport/lib/utils"
	"github.com/stretchr/testify/require"
)

func rotate(
	ctx context.Context, t *testing.T, client auth.ClientI, phase string,
) {
	t.Helper()
	err := client.RotateCertAuthority(ctx, auth.RotateRequest{
		Mode:        "manual",
		TargetPhase: phase,
	})
	require.NoError(t, err)
}

func setupServerForCARotationTest(ctx context.Context, t *testing.T, wg *sync.WaitGroup) (func() auth.ClientI, *config.FileConfig) {
	fc, fds := testhelpers.DefaultConfig(t)

	// Patch until https://github.com/gravitational/teleport/issues/13443
	// is resolved.
	fc.Databases.EnabledFlag = "false"

	cfg := service.MakeDefaultConfig()
	require.NoError(t, config.ApplyFileConfig(fc, cfg))
	cfg.FileDescriptors = fds
	cfg.Log = utils.NewLoggerForTests()

	cfg.CachePolicy.Enabled = false
	cfg.Proxy.DisableWebInterface = true

	svcC := make(chan *service.TeleportProcess, 20)
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := service.Run(ctx, *cfg, func(cfg *service.Config) (service.Process, error) {
			svc, err := service.NewTeleport(cfg)
			if err == nil {
				svcC <- svc
			}
			return svc, err
		})
		require.NoError(t, err)
	}()

	var svc *service.TeleportProcess
	select {
	case <-time.After(10 * time.Second):
		t.Fatal("teleport process did not instantiate in 10 seconds")
	case svc = <-svcC:
	}

	eventCh := make(chan service.Event, 1)
	svc.WaitForEvent(svc.ExitContext(), service.TeleportReadyEvent, eventCh)
	select {
	case <-eventCh:
	case <-time.After(30 * time.Second):
		// in reality, the auth server should start *much* sooner than this.  we use a very large
		// timeout here because this isn't the kind of problem that this test is meant to catch.
		t.Fatal("auth server didn't start after 30s")
	}

	return func() auth.ClientI {
		return testhelpers.MakeDefaultAuthClient(t, fc)
	}, fc
}

// TestCARotation is a heavy integration test that through a rotation, the bot
// receives credentials for a new CA.
func TestBot_Run_CARotation(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("test skipped when -short provided")
	}

	// wg and context manage the cancellation of long running processes e.g
	// teleport and tbot in the test.
	wg := &sync.WaitGroup{}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		t.Log("Shutting down long running test processes..")
		cancel()
		wg.Wait()
	})

	client, fc := setupServerForCARotationTest(ctx, t, wg)

	// Make and join a new bot instance.
	botParams := testhelpers.MakeBot(t, client(), "test", "access")
	botConfig := testhelpers.MakeMemoryBotConfig(t, fc, botParams)
	b := New(botConfig, libutils.NewLoggerForTests(), make(chan struct{}))

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := b.Run(ctx)
		require.NoError(t, err)
	}()
	// Allow time for bot to start running and watching for CA rotations
	// TODO: We should modify the bot to emit events that may be useful...
	time.Sleep(10 * time.Second)

	// fetch initial host cert
	require.Len(t, b.ident().TLSCACertsBytes, 2)
	initialCAs := [][]byte{}
	copy(initialCAs, b.ident().TLSCACertsBytes)

	// Fully rotate
	rotate(ctx, t, client(), "init")
	// TODO: These sleeps allow the client time to rotate. They could be
	// replaced if tbot emitted a CA rotation/renewal event.
	time.Sleep(time.Second * 30)
	_, err := b.client().Ping(ctx)
	require.NoError(t, err)

	rotate(ctx, t, client(), "update_clients")
	time.Sleep(time.Second * 30)
	// Ensure both sets of CA certificates are now available locally
	require.Len(t, b.ident().TLSCACertsBytes, 4)
	_, err = b.client().Ping(ctx)
	require.NoError(t, err)

	rotate(ctx, t, client(), "update_servers")
	time.Sleep(time.Second * 30)
	_, err = b.client().Ping(ctx)
	require.NoError(t, err)

	rotate(ctx, t, client(), "standby")
	time.Sleep(time.Second * 30)
	_, err = b.client().Ping(ctx)
	require.NoError(t, err)

	require.Len(t, b.ident().TLSCACertsBytes, 2)
	finalCAs := b.ident().TLSCACertsBytes
	require.NotEqual(t, initialCAs, finalCAs)
}
