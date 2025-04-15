package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/asappinc/generativeagent-amazon-connect/pkg/config"
	"github.com/asappinc/generativeagent-amazon-connect/pkg/quickstart"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/jsii-runtime-go"
	"github.com/heetch/confita"
	"github.com/heetch/confita/backend/file"
)

func main() {
	app := awscdk.NewApp(nil)
	envName := app.Node().TryGetContext(jsii.String("envName")).(string)
	cfg, err := loadConfiguration(envName)

	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}
	jsonConfig, _ := json.MarshalIndent(cfg, "", "  ")
	fmt.Printf("Loaded configuration:\n%+v\n", string(jsonConfig))

	prepareStagingDirectory()

	quickstart.NewQuickStartGenerativeAgentStack(app, fmt.Sprintf("%sstack", cfg.ObjectPrefix), &quickstart.AmazonConnectDemoCdkStackProps{
		StackProps: awscdk.StackProps{
			Env: env(cfg.AccountId, cfg.Region),
		},
	}, cfg)

	app.Synth(nil)
}

// env determines the AWS environment (account+region) in which our stack is to
// be deployed. For more information see: https://docs.aws.amazon.com/cdk/latest/guide/environments.html
func env(accountId, region string) *awscdk.Environment {

	// If unspecified, this stack will be "environment-agnostic".
	// Account/Region-dependent features and context lookups will not work, but a
	// single synthesized template can be deployed anywhere.
	//---------------------------------------------------------------------------
	// return nil

	// Uncomment if you know exactly what account and region you want to deploy
	// the stack to. This is the recommendation for production stacks.
	//---------------------------------------------------------------------------
	return &awscdk.Environment{
		Account: jsii.String(accountId),
		Region:  jsii.String(region),
	}

	// Uncomment to specialize this stack for the AWS Account and Region that are
	// implied by the current CLI configuration. This is recommended for dev
	// stacks.
	//---------------------------------------------------------------------------
	// return &awscdk.Environment{
	//  Account: jsii.String(os.Getenv("CDK_DEFAULT_ACCOUNT")),
	//  Region:  jsii.String(os.Getenv("CDK_DEFAULT_REGION")),
	// }
}

func loadConfiguration(envName string) (*config.Config, error) {
	// Load configuration
	cfgFilename := fmt.Sprintf("config.%s.json", envName)
	loader := confita.NewLoader(file.NewBackend(cfgFilename))
	cfg := config.Config{
		ObjectPrefix: "generativeagent-quickstart-", // this is default value that will get overwrritten by the config file if present
	}
	err := loader.Load(context.Background(), &cfg)
	return &cfg, err
}

func prepareStagingDirectory() {
	// Create the staging directory if it doesn't exist
	stagingDir := "staging"

	// Delete directory if it exists
	if _, err := os.Stat(stagingDir); !os.IsNotExist(err) {
		err := os.RemoveAll(stagingDir)
		if err != nil {
			log.Fatalf("Failed to delete existing staging directory: %v", err)
		}
	}
	// Create the staging directory
	err := os.Mkdir(stagingDir, 0755)
	if err != nil {
		log.Fatalf("Failed to create staging directory: %v", err)
	}

	err = os.MkdirAll(stagingDir+"/lambdas", 0755)
	if err != nil {
		log.Fatalf("Failed to create lambdas directory: %v", err)
	}

	// Copy all directories from ../../lambdas into staging directory
	lambdasSourceDir := "../../lambdas"
	// Check if source directory exists
	if _, err := os.Stat(lambdasSourceDir); os.IsNotExist(err) {
		log.Fatalf("Source directory %s does not exist: %v", lambdasSourceDir, err)
	}

	lambdaDir := os.DirFS(lambdasSourceDir)
	err = os.CopyFS(stagingDir+"/lambdas", lambdaDir)
	if err != nil {
		log.Fatalf("Failed to copy lambda directories: %v", err)
	}

}
