package testimpl_test

import (
	"context"
	"encoding/json"
	"go.arcalot.io/assert"
	"go.arcalot.io/log/v2"
	"go.flow.arcalot.io/pluginsdk/atp"
	"go.flow.arcalot.io/pluginsdk/schema"
	"go.flow.arcalot.io/testdeployer"
	"testing"
)

func TestSimpleInOut(t *testing.T) {
	configJSON := `{"deploy_time": 2}`
	var config any
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		t.Fatal(err)
	}

	factory := testimpl.NewFactory()
	schema := factory.ConfigurationSchema()
	unserializedConfig, err := schema.UnserializeType(config)
	assert.NoError(t, err)
	assert.NoError(t, unserializedConfig.Validate())
	connector, err := factory.Create(unserializedConfig, log.NewTestLogger(t))
	assert.NoError(t, err)

	container, err := connector.Deploy(context.Background(), "quay.io/joconnel/io-test-script")
	assert.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, container.Close())
	})
}

// TestE2E tests running a single wait step by using the ATP server.
func TestE2E(t *testing.T) {
	// Inputs and parameters
	image := "image-dummy"
	stepID := "wait"
	input := map[string]any{"wait_time_ms": 2}

	// Sets up the factory
	d := testimpl.NewFactory()
	configSchema := d.ConfigurationSchema()
	defaultConfig, err := configSchema.UnserializeType(map[string]any{})
	assert.NoError(t, err)

	// Creates the connector, which gives us the testimpl's deployer
	connector, err := d.Create(defaultConfig, log.New(log.Config{
		Level:       log.LevelDebug,
		Destination: log.DestinationStdout,
	}))
	assert.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Fake deploys the plugin
	plugin, err := connector.Deploy(ctx, image)
	assert.NoError(t, err)
	defer func() {
		err := plugin.Close()
		assert.NoError(t, err)
	}()

	// Connects to the plugin, then reads its schema
	atpClient := atp.NewClient(plugin)
	pluginSchema, err := atpClient.ReadSchema()
	assert.NoError(t, err)

	// Gets the schema for the step
	steps := pluginSchema.Steps()
	step, ok := steps[stepID]
	if !ok {
		t.Fatalf("no such step: %s", stepID)
	}

	assert.NoError(t, err)

	_, err = step.Input().Unserialize(input)
	assert.NoError(t, err)
	receivedSignalsChan := make(chan schema.Input)
	emittedSignalsChan := make(chan schema.Input)

	// Executes the step and validates that the output is correct.
	outputID, outputData, _ := atpClient.Execute(
		schema.Input{ID: stepID, InputData: input},
		receivedSignalsChan,
		emittedSignalsChan,
	)
	assert.Equals(t, outputID, "success")
	assert.Equals(t,
		outputData.(map[interface{}]interface{}),
		map[interface{}]interface{}{"message": "Plugin slept for 2 ms."})
}
