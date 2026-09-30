package engine

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestMetaString(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		value    *anypb.Any
		expected string
	}{
		{
			name:     "terragrunt version",
			value:    terragruntMeta(t, "v1.9.1"),
			expected: "v1.9.1",
		},
		{
			name:     "terragrunt latest",
			value:    terragruntMeta(t, "latest"),
			expected: "latest",
		},
		{
			name:     "terragrunt install dir",
			value:    terragruntMeta(t, "/opt/tofu"),
			expected: "/opt/tofu",
		},
		{
			name:     "terragrunt number",
			value:    terragruntMeta(t, 1.9),
			expected: "1.9",
		},
		{
			name:     "raw bytes",
			value:    &anypb.Any{Value: []byte("v1.9.1")},
			expected: "v1.9.1",
		},
		{
			name:     "missing",
			value:    nil,
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.expected, metaString(tc.value))
		})
	}
}

// terragruntMeta encodes a meta value the same way as ConvertMetaToProtobuf in Terragrunt.
func terragruntMeta(t *testing.T, value any) *anypb.Any {
	t.Helper()

	jsonData, err := json.Marshal(value)
	require.NoError(t, err)

	protoValue, err := structpb.NewValue(string(jsonData))
	require.NoError(t, err)

	encoded, err := anypb.New(protoValue)
	require.NoError(t, err)

	return encoded
}
