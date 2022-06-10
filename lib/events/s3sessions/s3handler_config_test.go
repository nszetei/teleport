package s3sessions

import (
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

func TestConfig_SetFromURL(t *testing.T) {

	// note that these configuration values and combinations do not necessarily make sense; should they be disallowed?

	parsedURL, err := url.Parse("s3://bucket/audit?insecure=true&disablesse=true&acl=private&use_fips_endpoint=true&sse_kms_key=abcdefg")
	require.NoError(t, err)

	cfg := &Config{}

	err = cfg.SetFromURL(parsedURL, "us-east-1")
	require.NoError(t, err)

	var (
		expectedBucket          = "bucket"
		expectedInsecure        = true
		expectedDisableSSE      = true
		expectedACL             = "private"
		expectedUseFipsEndpoint = true
		sseKMSKey               = "abcdefg"
	)
	require.EqualValues(t, expectedBucket, cfg.Bucket)
	require.EqualValues(t, expectedInsecure, cfg.Insecure)
	require.EqualValues(t, expectedDisableSSE, cfg.DisableServerSideEncryption)
	require.EqualValues(t, expectedACL, cfg.ACL)
	require.EqualValues(t, expectedUseFipsEndpoint, *cfg.UseFIPSEndpoint)
	require.EqualValues(t, sseKMSKey, cfg.SSEKMSKey)

	parsedURL, err = url.Parse("s3://bucket/audit?insecure=false&disablesse=false&use_fips_endpoint=false&endpoint=s3.example.com")
	require.NoError(t, err)

	err = cfg.SetFromURL(parsedURL, "us-east-1")
	require.NoError(t, err)

	expectedInsecure = false
	expectedDisableSSE = false
	expectedUseFipsEndpoint = false
	expectedEndpoint := "s3.example.com"

	require.EqualValues(t, expectedBucket, cfg.Bucket)
	require.EqualValues(t, expectedInsecure, cfg.Insecure)
	require.EqualValues(t, expectedDisableSSE, cfg.DisableServerSideEncryption)
	require.EqualValues(t, expectedACL, cfg.ACL)
	require.EqualValues(t, expectedUseFipsEndpoint, *cfg.UseFIPSEndpoint)
	require.EqualValues(t, expectedEndpoint, cfg.Endpoint)

	parsedURL, err = url.Parse("s3://bucket/audit")
	require.NoError(t, err)

	err = cfg.SetFromURL(parsedURL, "us-east-1")
	require.NoError(t, err)

	require.Nil(t, cfg.UseFIPSEndpoint)
}
