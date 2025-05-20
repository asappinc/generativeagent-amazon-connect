package quickstart

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/asappinc/generativeagent-amazon-connect/pkg/config"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsconnect"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awselasticache"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambdanodejs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3deployment"
	"github.com/aws/aws-cdk-go/awscdk/v2/customresources"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/connect"
	"github.com/aws/aws-sdk-go-v2/service/connect/types"
	"github.com/aws/jsii-runtime-go"

	"github.com/iancoleman/orderedmap"

	"github.com/aws/constructs-go/constructs/v10"
)

type AmazonConnectDemoCdkStackProps struct {
	awscdk.StackProps
	EnvName *string
}

const (
	promptsPath = "../../flow-modules/prompts"

	// Lambda paths
	engageLambdaDir                           = "staging/lambdas/engage"
	engageLambdaIndexPath                     = engageLambdaDir + "/index.mjs"
	engageLambdaAttributeToInputVariablesPath = engageLambdaDir + "/attributesToInputVariables.mjs"
	engageLambdaLockPath                      = engageLambdaDir + "/package-lock.json"
	pullActionLambdaDir                       = "staging/lambdas/pullaction"
	pullActionLambdaIndexPath                 = pullActionLambdaDir + "/index.mjs"
	pullActionLambdaLockPath                  = pullActionLambdaDir + "/package-lock.json"
	pullActionSSMLConversionsPath             = pullActionLambdaDir + "/ssmlConversions.mjs"
	pushActionLambdaDir                       = "staging/lambdas/pushaction"
	pushActionLambdaIndexPath                 = pushActionLambdaDir + "/index.mjs"
	pushActionLambdaLockPath                  = pushActionLambdaDir + "/package-lock.json"

	// Template paths
	contactFlowModulePath = "../../flow-modules/template/ASAPPGenerativeAgent.json"

	lambdaFunctionAlias = "prod"
)

func NewQuickStartGenerativeAgentStack(scope constructs.Construct, id string, props *AmazonConnectDemoCdkStackProps, cfg *config.Config) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	awslogs.NewLogGroup(stack, generateObjectName(cfg, "stack-log-group"), &awslogs.LogGroupProps{
		LogGroupName:  generateObjectName(cfg, "stack-log-group"),
		Retention:     awslogs.RetentionDays_THREE_DAYS,
		RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
	})

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(cfg.Region))
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	// Check if there is any Kinesis Video Stream configuration
	svc := connect.NewFromConfig(awsCfg)
	instanceStorageConfigsInput := &connect.ListInstanceStorageConfigsInput{
		InstanceId:   jsii.String(cfg.ConnectInstanceArn),
		ResourceType: types.InstanceStorageResourceTypeMediaStreams,
	}
	result, err := svc.ListInstanceStorageConfigs(context.Background(), instanceStorageConfigsInput)
	if err != nil {
		fmt.Println("Error:", err)
	}

	// custom resource IAM role/policy for CDK created Lambda functions to call AWS SDK
	customResourceRole := awsiam.NewRole(stack, generateObjectName(cfg, "custom-resource-role"), &awsiam.RoleProps{
		RoleName:  generateObjectName(cfg, "custom-resource-role"),
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("lambda.amazonaws.com"), nil),
	})

	customResourcesPolicy := awsiam.NewPolicy(stack, generateObjectName(cfg, "custom-resource-policy"), &awsiam.PolicyProps{
		PolicyName: generateObjectName(cfg, "custom-resource-policy"),
		Statements: &[]awsiam.PolicyStatement{
			// Access Amazon Connect settings to create/disassociate the Kinesis Video Stream, give access to S3 bucket and read KMS key
			awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
				Actions: jsii.Strings(
					"connect:AssociateInstanceStorageConfig",
					"connect:DisassociateInstanceStorageConfig",
					"ds:DescribeDirectories",
					"iam:PutRolePolicy",
					"kms:DescribeKey",
				),
				Resources: jsii.Strings("*"),
			}),
			// Access Amazon Connect to create/delete prompts and read S3 bucket where prompt files are stored
			awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
				Actions: jsii.Strings(
					"connect:CreatePrompt",
					"connect:DeletePrompt",
					"s3:GetObject",
					"s3:ListBucket",
				),
				Resources: jsii.Strings("*"),
			}),
			// Access Amazon Connect to associate/disassociate Lambda functions and add resource permission to Lambda function
			awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
				Actions: jsii.Strings(
					"connect:AssociateLambdaFunction",
					"connect:DisassociateLambdaFunction",
					"lambda:AddPermission",
				),
				Resources: jsii.Strings("*"),
			}),
		},
	})

	customResourceRole.AttachInlinePolicy(customResourcesPolicy)

	kinesisPrefixExists := false
	kinesisVideoStreamConfigPrefix := ""
	kinesisVideoKMSKeyArn := ""
	if len(result.StorageConfigs) > 0 {
		for _, storageConfig := range result.StorageConfigs {
			if storageConfig.StorageType == types.StorageTypeKinesisVideoStream {
				fmt.Printf("Kinesis Video Stream Config found: %v\n", *storageConfig.KinesisVideoStreamConfig.Prefix)
				kinesisVideoStreamConfigPrefix = *storageConfig.KinesisVideoStreamConfig.Prefix
				kinesisVideoKMSKeyArn = *storageConfig.KinesisVideoStreamConfig.EncryptionConfig.KeyId
				kinesisPrefixExists = true
			}
		}
	}

	if !kinesisPrefixExists { // If the Kinesis prefix doesn't exist, create it using a AWS Custom Resource call
		kinesisVideoStreamConfigPrefix = cfg.ObjectPrefix
		kinesisPrefixResource := customresources.NewAwsCustomResource(stack, jsii.String("EnableKinesisPrefix"), &customresources.AwsCustomResourceProps{
			OnCreate: &customresources.AwsSdkCall{
				Service: jsii.String("Connect"),
				Action:  jsii.String("AssociateInstanceStorageConfig"),
				Parameters: map[string]interface{}{
					"InstanceId":   cfg.ConnectInstanceArn,
					"ResourceType": "MEDIA_STREAMS",
					"StorageConfig": map[string]interface{}{
						"StorageType": "KINESIS_VIDEO_STREAM",
						"KinesisVideoStreamConfig": map[string]interface{}{
							"Prefix": kinesisVideoStreamConfigPrefix,
							"EncryptionConfig": map[string]interface{}{
								"EncryptionType": "KMS",
								"KeyId":          "alias/aws/kinesisvideo",
							},
							"RetentionPeriodHours": 24,
						},
					},
				},
				PhysicalResourceId: customresources.PhysicalResourceId_FromResponse(jsii.String("AssociationId")),
			},
			OnDelete: &customresources.AwsSdkCall{
				Service: jsii.String("Connect"),
				Action:  jsii.String("DisassociateInstanceStorageConfig"),
				Parameters: map[string]interface{}{
					"InstanceId":    cfg.ConnectInstanceArn,
					"ResourceType":  "MEDIA_STREAMS",
					"AssociationId": customresources.NewPhysicalResourceIdReference()},
			},
			Role: customResourceRole,
		})
		kinesisPrefixResource.Node().AddDependency(customResourceRole, customResourcesPolicy)
	}

	// -- Setup the Prompts --
	// Create an S3 Bucket to store the audio files
	s3Bucket := awss3.NewBucket(stack, generateObjectName(cfg, "bucket"), &awss3.BucketProps{
		Versioned:         jsii.Bool(false),
		RemovalPolicy:     awscdk.RemovalPolicy_DESTROY,
		AutoDeleteObjects: jsii.Bool(true),
	})

	// Grant the custom Role read access to the S3 Bucket
	customInstanceRole := awsiam.NewRole(stack, generateObjectName(cfg, "custom-instance-role"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("connect.amazonaws.com"), nil),
	})
	s3Bucket.GrantRead(customInstanceRole, "*")

	// Upload the audio files to the S3 Bucket
	s3BucketDeployment := awss3deployment.NewBucketDeployment(stack, generateObjectName(cfg, "bucket-deployment"), &awss3deployment.BucketDeploymentProps{
		Sources: &[]awss3deployment.ISource{
			awss3deployment.Source_Asset(jsii.String(promptsPath), nil),
		},
		DestinationBucket: s3Bucket,
	})

	// Create an S3 URL for the uploaded audio file
	beepbopUrl := s3Bucket.S3UrlForObject(jsii.String("asappBeepBop.wav"))
	silence1secondUrl := s3Bucket.S3UrlForObject(jsii.String("asappSilence1second.wav"))
	silence400msUrl := s3Bucket.S3UrlForObject(jsii.String("asappSilence400ms.wav"))

	// Create the Prompts
	createBeepbopShortPrompt := customresources.NewAwsCustomResource(stack, generateObjectName(cfg, "create-prompt-asappBeepBop"),
		&customresources.AwsCustomResourceProps{
			OnCreate: &customresources.AwsSdkCall{
				Service: jsii.String("Connect"),
				Action:  jsii.String("CreatePrompt"),
				Parameters: map[string]interface{}{
					"InstanceId":  jsii.String(cfg.ConnectInstanceArn),
					"Name":        generateObjectName(cfg, "asappBeepBop"),
					"S3Uri":       beepbopUrl,
					"Description": jsii.String("Short BeepBop no silence"),
				},
				PhysicalResourceId: customresources.PhysicalResourceId_FromResponse(jsii.String("PromptId")),
			},
			OnDelete: &customresources.AwsSdkCall{
				Service: jsii.String("Connect"),
				Action:  jsii.String("DeletePrompt"),
				Parameters: map[string]interface{}{
					"InstanceId": jsii.String(cfg.ConnectInstanceArn),
					"PromptId":   customresources.NewPhysicalResourceIdReference(),
				},
			},
			Role: customResourceRole,
		})

	createSilence1secondPrompt := customresources.NewAwsCustomResource(stack, generateObjectName(cfg, "create-prompt-asappSilence1second"),
		&customresources.AwsCustomResourceProps{
			OnCreate: &customresources.AwsSdkCall{
				Service: jsii.String("Connect"),
				Action:  jsii.String("CreatePrompt"),
				Parameters: map[string]interface{}{
					"InstanceId":  jsii.String(cfg.ConnectInstanceArn),
					"Name":        generateObjectName(cfg, "asappSilence1second"),
					"S3Uri":       silence1secondUrl,
					"Description": jsii.String("One second silence"),
				},
				PhysicalResourceId: customresources.PhysicalResourceId_FromResponse(jsii.String("PromptId")),
			},
			OnDelete: &customresources.AwsSdkCall{
				Service: jsii.String("Connect"),
				Action:  jsii.String("DeletePrompt"),
				Parameters: map[string]interface{}{
					"InstanceId": jsii.String(cfg.ConnectInstanceArn),
					"PromptId":   customresources.NewPhysicalResourceIdReference(),
				},
			},
			Role: customResourceRole,
		})

	createSilence400msPrompt := customresources.NewAwsCustomResource(stack, generateObjectName(cfg, "create-prompt-asappSilence400ms"),
		&customresources.AwsCustomResourceProps{
			OnCreate: &customresources.AwsSdkCall{
				Service: jsii.String("Connect"),
				Action:  jsii.String("CreatePrompt"),
				Parameters: map[string]interface{}{
					"InstanceId":  jsii.String(cfg.ConnectInstanceArn),
					"Name":        generateObjectName(cfg, "asappSilence400ms"),
					"S3Uri":       silence400msUrl,
					"Description": jsii.String("Silence for 400ms"),
				},
				PhysicalResourceId: customresources.PhysicalResourceId_FromResponse(jsii.String("PromptId")),
			},
			OnDelete: &customresources.AwsSdkCall{
				Service: jsii.String("Connect"),
				Action:  jsii.String("DeletePrompt"),
				Parameters: map[string]interface{}{
					"InstanceId": jsii.String(cfg.ConnectInstanceArn),
					"PromptId":   customresources.NewPhysicalResourceIdReference(),
				},
			},
			Role: customResourceRole,
		})

	// Wait for the IAM role/policy to be created and the audio files to be uploaded before creating the Prompts
	createBeepbopShortPrompt.Node().AddDependency(customResourceRole, customResourcesPolicy, s3BucketDeployment)
	createSilence1secondPrompt.Node().AddDependency(customResourceRole, customResourcesPolicy, s3BucketDeployment)
	createSilence400msPrompt.Node().AddDependency(customResourceRole, customResourcesPolicy, s3BucketDeployment)

	beepBopShortPromptId := createBeepbopShortPrompt.GetResponseField(jsii.String("PromptId"))
	silence1secondPromptId := createSilence1secondPrompt.GetResponseField(jsii.String("PromptId"))
	silence400msPromptId := createSilence400msPrompt.GetResponseField(jsii.String("PromptId"))

	// VPC configuration
	var vpc awsec2.IVpc

	if cfg.UseExistingVpcId != "" {
		// -- Lookup the VPC --
		vpc = awsec2.Vpc_FromLookup(stack, generateObjectName(cfg, "vpc"), &awsec2.VpcLookupOptions{
			VpcId: jsii.String(cfg.UseExistingVpcId),
		})
		if vpc == nil {
			log.Fatalf("Failed to find VPC with ID: %s", cfg.UseExistingVpcId)
			return nil
		}

	} else {
		// -- Create the VPC --
		vpc = awsec2.NewVpc(stack, generateObjectName(cfg, "vpc"), &awsec2.VpcProps{
			MaxAzs: aws.Float64(2), // Two availability zones.
			SubnetConfiguration: &[]*awsec2.SubnetConfiguration{
				{
					Name:       jsii.String("PrivateIsolatedSubnet"),
					SubnetType: awsec2.SubnetType_PRIVATE_ISOLATED,
				},
			},
		})
	}

	vpcPrivateIsolatedSubnets := *vpc.IsolatedSubnets()

	// -- Create the Security Groups --
	valkeySecurityGroup := awsec2.NewSecurityGroup(stack, generateObjectName(cfg, "valkey-security-group"), &awsec2.SecurityGroupProps{
		Vpc:               vpc,
		SecurityGroupName: generateObjectName(cfg, "valkey-security-group"),
		AllowAllOutbound:  jsii.Bool(false),
	})
	pullActionLambdaSecurityGroup := awsec2.NewSecurityGroup(stack, generateObjectName(cfg, "lambda-pullaction-security-group"), &awsec2.SecurityGroupProps{
		Vpc:               vpc,
		SecurityGroupName: generateObjectName(cfg, "lambda-pullaction-security-group"),
		AllowAllOutbound:  jsii.Bool(false),
	})
	pushActionLambdaSecurityGroup := awsec2.NewSecurityGroup(stack, generateObjectName(cfg, "lambda-pushaction-security-group"), &awsec2.SecurityGroupProps{
		Vpc:               vpc,
		SecurityGroupName: generateObjectName(cfg, "lambda-pushaction-security-group"),
		AllowAllOutbound:  jsii.Bool(false),
	})

	valkeySecurityGroup.AddIngressRule(
		pullActionLambdaSecurityGroup,
		awsec2.Port_Tcp(aws.Float64(6379)),
		jsii.String(fmt.Sprintf("Allow inbound TCP traffic only from %slambda-pullaction-security-group into the Reddis port", cfg.ObjectPrefix)),
		jsii.Bool(false),
	)

	valkeySecurityGroup.AddIngressRule(
		pushActionLambdaSecurityGroup,
		awsec2.Port_Tcp(aws.Float64(6379)),
		jsii.String(fmt.Sprintf("Allow inbound TCP traffic only from %slambda-pushaction-security-group into the Reddis port", cfg.ObjectPrefix)),
		jsii.Bool(false),
	)

	pullActionLambdaSecurityGroup.AddEgressRule(
		valkeySecurityGroup,
		awsec2.Port_Tcp(aws.Float64(6379)),
		jsii.String("Allow outbound TCP traffic only to Valkey security group and port"),
		jsii.Bool(false),
	)

	pushActionLambdaSecurityGroup.AddEgressRule(
		valkeySecurityGroup,
		awsec2.Port_Tcp(aws.Float64(6379)),
		jsii.String("Allow outbound TCP traffic only to Valkey security group and port"),
		jsii.Bool(false),
	)

	var valkeySubnetGroupSubnetIds []*string
	for _, subnet := range vpcPrivateIsolatedSubnets {
		valkeySubnetGroupSubnetIds = append(valkeySubnetGroupSubnetIds, subnet.SubnetId())
	}

	// -- Create the Valkey replication group --
	valkeySubnetGroup := awselasticache.NewCfnSubnetGroup(stack, generateObjectName(cfg, "valkey-subnet-group"), &awselasticache.CfnSubnetGroupProps{
		Description: jsii.String("Subnet group for ASAPP Valkey"),
		SubnetIds:   &valkeySubnetGroupSubnetIds,
	})
	valkeyReplicationGroupId := *generateObjectName(cfg, "valkey")
	if len(valkeyReplicationGroupId) > 40 {
		valkeyReplicationGroupId = valkeyReplicationGroupId[:40]
	}
	valkeyReplicationGroup := awselasticache.NewCfnReplicationGroup(stack, generateObjectName(cfg, "valkey-repl-group"), &awselasticache.CfnReplicationGroupProps{
		ReplicationGroupId:          jsii.String(valkeyReplicationGroupId),
		ReplicationGroupDescription: jsii.String("ASAPP Valkey replication group"),
		CacheNodeType:               jsii.String(cfg.ValkeyParameters.CacheNodeType),
		Engine:                      jsii.String("valkey"),
		NumNodeGroups:               jsii.Number(1),
		ReplicasPerNodeGroup:        jsii.Number(cfg.ValkeyParameters.ReplicaNodesCount),
		CacheSubnetGroupName:        valkeySubnetGroup.Ref(),
		SecurityGroupIds:            &[]*string{valkeySecurityGroup.SecurityGroupId()},
		AutomaticFailoverEnabled:    jsii.Bool(true),
		TransitEncryptionEnabled:    jsii.Bool(false),
		MultiAzEnabled:              jsii.Bool(true),
	})

	attributeToVarsFile, err := os.Create(engageLambdaAttributeToInputVariablesPath)
	if err != nil {
		log.Fatalf("Failed to create attributeToVarsFile: %v", err)
		return nil
	}
	defer attributeToVarsFile.Close()
	err = writeAttributesToInputVariablesFile(attributeToVarsFile, cfg.AttributesToInputVariablesMap)
	if err != nil {
		log.Fatalf("Failed to write to attributeToVarsFile: %v", err)
		return nil

	}

	/// -- Create the Lambda functions and associate them to the Connect Instance --
	// Engage: this function only talks to Internet endpoints and is not attached to a VPC.
	engageLambdaFunction := awslambdanodejs.NewNodejsFunction(stack, generateObjectName(cfg, "lambda-genagent-engage"), &awslambdanodejs.NodejsFunctionProps{
		FunctionName:     generateObjectName(cfg, "lambda-genagent-engage"),
		Entry:            jsii.String(engageLambdaIndexPath),
		Handler:          jsii.String("handler"),
		Runtime:          awslambda.Runtime_NODEJS_22_X(),
		Timeout:          awscdk.Duration_Seconds(jsii.Number(15)),
		DepsLockFilePath: jsii.String(engageLambdaLockPath),
		Bundling: &awslambdanodejs.BundlingOptions{
			Format: awslambdanodejs.OutputFormat_ESM,
			ExternalModules: &[]*string{
				jsii.String("aws-sdk"), // aws-sdk is already included in Lambda environment
			},
			NodeModules: &[]*string{
				jsii.String("axios"),
			},
			ForceDockerBundling: jsii.Bool(true),
		},
		Environment: &map[string]*string{
			"ASAPP_API_HOST":   jsii.String(cfg.Asapp.ApiHost),
			"ASAPP_API_ID":     jsii.String(cfg.Asapp.ApiId),
			"ASAPP_API_SECRET": jsii.String(cfg.Asapp.ApiSecret),
		},
	})
	engageLambdaFunction.AddPermission(jsii.String("AmazonConnectInvokePermission"), &awslambda.Permission{
		Principal: awsiam.NewServicePrincipal(jsii.String("connect.amazonaws.com"), nil),
		SourceArn: jsii.String(cfg.ConnectInstanceArn),
		Action:    jsii.String("lambda:InvokeFunction"),
	})

	associateEngageLambdaWithConnect := customresources.NewAwsCustomResource(stack, jsii.String("AssociateEngageLambdaWithConnect"), &customresources.AwsCustomResourceProps{
		OnCreate: &customresources.AwsSdkCall{
			Service: jsii.String("Connect"),
			Action:  jsii.String("AssociateLambdaFunction"),
			Parameters: map[string]interface{}{
				"InstanceId":  jsii.String(cfg.ConnectInstanceArn),
				"FunctionArn": engageLambdaFunction.FunctionArn(),
			},
			PhysicalResourceId: customresources.PhysicalResourceId_Of(jsii.String("AssociateEngageLambda")),
		},
		OnDelete: &customresources.AwsSdkCall{
			Service: jsii.String("Connect"),
			Action:  jsii.String("DisassociateLambdaFunction"),
			Parameters: map[string]interface{}{
				"InstanceId":  jsii.String(cfg.ConnectInstanceArn),
				"FunctionArn": engageLambdaFunction.FunctionArn(),
			},
		},
		Role: customResourceRole,
	})

	engageLambdaVersion := engageLambdaFunction.CurrentVersion()

	engageLambdaAliasProps := &awslambda.AliasProps{
		AliasName:   jsii.String(lambdaFunctionAlias),
		Version:     engageLambdaVersion,
		Description: jsii.String("Production alias called by Connect"),
	}

	if cfg.LambdaProvisionedConcurrency.EngageProvisionedConcurrency > 0 {
		engageLambdaAliasProps.ProvisionedConcurrentExecutions = jsii.Number(float64(cfg.LambdaProvisionedConcurrency.EngageProvisionedConcurrency))
	}

	engageLambdaAlias := awslambda.NewAlias(stack, jsii.String("EngageLambdaAlias"), engageLambdaAliasProps)
	engageLambdaAlias.AddPermission(jsii.String("AmazonConnectInvokePermission"), &awslambda.Permission{
		Principal: awsiam.NewServicePrincipal(jsii.String("connect.amazonaws.com"), nil),
		SourceArn: jsii.String(cfg.ConnectInstanceArn),
		Action:    jsii.String("lambda:InvokeFunction"),
	})

	associateEngageLambdaWithConnect.Node().AddDependency(engageLambdaFunction, engageLambdaAlias, customResourceRole, customResourcesPolicy)

	ssmlConversionsFile, err := os.Create(pullActionSSMLConversionsPath)
	if err != nil {
		log.Fatalf("Failed to create ssmlConversionsFile: %v", err)
		return nil
	}
	defer ssmlConversionsFile.Close()
	err = writeSSMLConversionsFile(ssmlConversionsFile, cfg.SSMLConversions)
	if err != nil {
		log.Fatalf("Failed to write to ssmlConversionsFile: %v", err)
		return nil

	}

	// PullAction: this function needs access to Valkey, so it needs to be on the same VPC.
	pullActionLambdaFunction := awslambdanodejs.NewNodejsFunction(stack, generateObjectName(cfg, "lambda-pullaction"), &awslambdanodejs.NodejsFunctionProps{
		FunctionName:     generateObjectName(cfg, "lambda-pullaction"),
		Entry:            jsii.String(pullActionLambdaIndexPath),
		Handler:          jsii.String("handler"),
		Runtime:          awslambda.Runtime_NODEJS_22_X(),
		DepsLockFilePath: jsii.String(pullActionLambdaLockPath),
		Bundling: &awslambdanodejs.BundlingOptions{
			Format: awslambdanodejs.OutputFormat_ESM,
			ExternalModules: &[]*string{
				jsii.String("aws-sdk"), // aws-sdk is already included in Lambda environment
			},
			NodeModules: &[]*string{
				jsii.String("@valkey/valkey-glide"),
			},
			ForceDockerBundling: jsii.Bool(true),
		},
		Vpc: vpc,
		VpcSubnets: &awsec2.SubnetSelection{
			Subnets: &vpcPrivateIsolatedSubnets,
		},
		SecurityGroups: &[]awsec2.ISecurityGroup{pullActionLambdaSecurityGroup},
		Environment: &map[string]*string{
			"VALKEY_HOST": valkeyReplicationGroup.AttrPrimaryEndPointAddress(),
			"VALKEY_PORT": valkeyReplicationGroup.AttrPrimaryEndPointPort(),
		},
	})
	pullActionLambdaFunction.AddPermission(jsii.String("AmazonConnectInvokePermission"), &awslambda.Permission{
		Principal: awsiam.NewServicePrincipal(jsii.String("connect.amazonaws.com"), nil),
		SourceArn: jsii.String(cfg.ConnectInstanceArn),
		Action:    jsii.String("lambda:InvokeFunction"),
	})

	associatePullActionLambdaWithConnect := customresources.NewAwsCustomResource(stack, jsii.String("AssociatePullActionLambdaWithConnect"), &customresources.AwsCustomResourceProps{
		OnCreate: &customresources.AwsSdkCall{
			Service: jsii.String("Connect"),
			Action:  jsii.String("AssociateLambdaFunction"),
			Parameters: map[string]interface{}{
				"InstanceId":  jsii.String(cfg.ConnectInstanceArn),
				"FunctionArn": pullActionLambdaFunction.FunctionArn(),
			},
			PhysicalResourceId: customresources.PhysicalResourceId_Of(jsii.String("AssociatePullActionLambda")),
		},
		OnDelete: &customresources.AwsSdkCall{
			Service: jsii.String("Connect"),
			Action:  jsii.String("DisassociateLambdaFunction"),
			Parameters: map[string]interface{}{
				"InstanceId":  jsii.String(cfg.ConnectInstanceArn),
				"FunctionArn": pullActionLambdaFunction.FunctionArn(),
			},
		},
		Role: customResourceRole,
	})

	pullActionLambdaVersion := pullActionLambdaFunction.CurrentVersion()
	pullActionLambdaAliasProps := &awslambda.AliasProps{
		AliasName:   jsii.String(lambdaFunctionAlias),
		Version:     pullActionLambdaVersion,
		Description: jsii.String("Production alias called by Connect"),
	}
	if cfg.LambdaProvisionedConcurrency.PullActionProvisionedConcurrency > 0 {
		pullActionLambdaAliasProps.ProvisionedConcurrentExecutions = jsii.Number(float64(cfg.LambdaProvisionedConcurrency.PullActionProvisionedConcurrency))
	}

	pullActionLambdaAlias := awslambda.NewAlias(stack, jsii.String("PullActionLambdaAlias"), pullActionLambdaAliasProps)

	pullActionLambdaAlias.AddPermission(jsii.String("AmazonConnectInvokePermission"), &awslambda.Permission{
		Principal: awsiam.NewServicePrincipal(jsii.String("connect.amazonaws.com"), nil),
		SourceArn: jsii.String(cfg.ConnectInstanceArn),
		Action:    jsii.String("lambda:InvokeFunction"),
	})

	associatePullActionLambdaWithConnect.Node().AddDependency(pullActionLambdaFunction, pullActionLambdaAlias, customResourceRole, customResourcesPolicy)

	// PushAction: this function needs access to Valkey, so it needs to be on the same VPC.
	pushActionLambdaFunction := awslambdanodejs.NewNodejsFunction(stack, generateObjectName(cfg, "lambda-pushaction"), &awslambdanodejs.NodejsFunctionProps{
		FunctionName:     generateObjectName(cfg, "lambda-pushaction"),
		Entry:            jsii.String(pushActionLambdaIndexPath),
		Handler:          jsii.String("handler"),
		Runtime:          awslambda.Runtime_NODEJS_22_X(),
		DepsLockFilePath: jsii.String(pushActionLambdaLockPath),
		Bundling: &awslambdanodejs.BundlingOptions{
			Format: awslambdanodejs.OutputFormat_ESM,
			ExternalModules: &[]*string{
				jsii.String("aws-sdk"), // aws-sdk is already included in Lambda environment
			},
			NodeModules: &[]*string{
				jsii.String("@valkey/valkey-glide"),
			},
			ForceDockerBundling: jsii.Bool(true),
		},
		Vpc: vpc,
		VpcSubnets: &awsec2.SubnetSelection{
			Subnets: &vpcPrivateIsolatedSubnets,
		},
		SecurityGroups: &[]awsec2.ISecurityGroup{pushActionLambdaSecurityGroup},
		Environment: &map[string]*string{
			"VALKEY_HOST": valkeyReplicationGroup.AttrPrimaryEndPointAddress(),
			"VALKEY_PORT": valkeyReplicationGroup.AttrPrimaryEndPointPort(),
		},
	})

	pushActionLambdaVersion := pushActionLambdaFunction.CurrentVersion()
	pushActionLambdaAliasProps := &awslambda.AliasProps{
		AliasName:   jsii.String(lambdaFunctionAlias),
		Version:     pushActionLambdaVersion,
		Description: jsii.String("Production alias called by ASAPP"),
	}
	if cfg.LambdaProvisionedConcurrency.PushActionProvisionedConcurrency > 0 {
		pushActionLambdaAliasProps.ProvisionedConcurrentExecutions = jsii.Number(float64(cfg.LambdaProvisionedConcurrency.PushActionProvisionedConcurrency))
	}
	pushActionLambdaAlias := awslambda.NewAlias(stack, jsii.String("PushActionLambdaAlias"), pushActionLambdaAliasProps)

	// Read Contact Flow Module
	contactFlowModuleContent, err := os.ReadFile(contactFlowModulePath)
	if err != nil {
		log.Fatalf("Failed to read Contact Flow Module file: %v", err)
		return nil
	}

	// Unmarshal the JSON data into a map
	var contactFlowModuleContentMap orderedmap.OrderedMap
	if err := json.Unmarshal(contactFlowModuleContent, &contactFlowModuleContentMap); err != nil {
		fmt.Printf("Failed to unmarshal JSON: %v\n", err)
		return nil
	}

	// Setup a map with the newly created Prompts ARNs to be replaced in the Contact Flow Module
	parsedInstanceIdArn, err := arn.Parse(cfg.ConnectInstanceArn)
	if err != nil {
		log.Fatalf("Failed to parse ConnectInstanceArn: %v", err)
	}
	resourceArnSection := strings.Split(parsedInstanceIdArn.Resource, "/")

	var connectInstanceId string
	if len(resourceArnSection) == 2 && resourceArnSection[0] == "instance" { // Check if the resource section has "instance" and get the instance ID
		connectInstanceId = resourceArnSection[1]
	}
	promptArnsMap := map[string]string{
		"Wait1sPrompt":     fmt.Sprintf("arn:aws:connect:%s:%s:instance/%s/prompt/%s", cfg.Region, cfg.AccountId, connectInstanceId, *silence1secondPromptId),
		"Wait400msPrompt":  fmt.Sprintf("arn:aws:connect:%s:%s:instance/%s/prompt/%s", cfg.Region, cfg.AccountId, connectInstanceId, *silence400msPromptId),
		"PlayBeepBopShort": fmt.Sprintf("arn:aws:connect:%s:%s:instance/%s/prompt/%s", cfg.Region, cfg.AccountId, connectInstanceId, *beepBopShortPromptId),
	}

	// Setup a map with the newly created Lambda Function ARNs to be replaced in the Contact Flow Module
	lambdaFunctionsArnMap := map[string]string{
		"Engage":     *engageLambdaAlias.FunctionArn(),
		"PullAction": *pullActionLambdaAlias.FunctionArn(),
	}

	// Update the referenced resources (Prompts and Lambda functions), then Marshal the content into the same variable.
	UpdateResourcesARN(&contactFlowModuleContentMap, cfg.Region, cfg.AccountId, cfg.ConnectInstanceArn, promptArnsMap, lambdaFunctionsArnMap)

	// Update Output Variables
	UpdateExtractOutputVariables(&contactFlowModuleContentMap, cfg.OutputVariablesToAttributesMap)

	// Update SpeakResponse in module if SSML conversions are provided
	if len(cfg.SSMLConversions) != 0 {
		UpdateSpeakResponseToSSML(&contactFlowModuleContentMap)

	}

	contactFlowModuleContent, err = json.MarshalIndent(contactFlowModuleContentMap, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal updated JSON: %v\n", err)
	}

	// Create the Contact Flow Module
	connectModule := awsconnect.NewCfnContactFlowModule(stack, generateObjectName(cfg, "contact-flow-module"), &awsconnect.CfnContactFlowModuleProps{
		InstanceArn: jsii.String(cfg.ConnectInstanceArn),
		Name:        generateObjectName(cfg, "contact-flow-module"),
		Content:     jsii.String(string(contactFlowModuleContent)),
	})

	// Wait for the Prompts to be ready before proceeding to create the Contact Flow Module
	connectModule.Node().AddDependency(createBeepbopShortPrompt, createSilence1secondPrompt, createSilence400msPrompt)

	// -- Create the Role: generativeagent-quickstart-access-role --
	asappGenagentAccessRole := awsiam.NewRole(stack, generateObjectName(cfg, "access-role"), &awsiam.RoleProps{
		RoleName: generateObjectName(cfg, "access-role"),
		AssumedBy: awsiam.NewCompositePrincipal(
			awsiam.NewArnPrincipal(jsii.String(cfg.Asapp.AssumingRoleArn)), // TrustASAPPRole
		)})

	var sbAsappKinesisAccessPolicy strings.Builder
	sbAsappKinesisAccessPolicy.WriteString("arn:aws:kinesisvideo:*:")
	sbAsappKinesisAccessPolicy.WriteString(cfg.AccountId)
	sbAsappKinesisAccessPolicy.WriteString(":stream/")
	sbAsappKinesisAccessPolicy.WriteString(kinesisVideoStreamConfigPrefix)
	sbAsappKinesisAccessPolicy.WriteString("*/*")
	asappKinesisAccessPolicy := awsiam.NewPolicy(stack, generateObjectName(cfg, "kinesis-access"), &awsiam.PolicyProps{
		PolicyName: generateObjectName(cfg, "kinesis-access"),
		Statements: &[]awsiam.PolicyStatement{
			// First Statement: ReadAmazonConnectStreams
			awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					jsii.String("kinesisvideo:GetDataEndpoint"),
					jsii.String("kinesisvideo:GetMedia"),
					jsii.String("kinesisvideo:DescribeStream"),
				},
				Resources: &[]*string{
					jsii.String(sbAsappKinesisAccessPolicy.String()),
				},
			}),
			// Second Statement: ListAllStreams
			awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					jsii.String("kinesisvideo:ListStreams"),
				},
				Resources: &[]*string{
					jsii.String("*"),
				},
			}),
		},
	})

	// if kinesisVideoKMSKeyArn is not empty, add a statement to allow decrypting the KMS key, this is not needed if the KMS key is aws/kinesisvideo
	if kinesisVideoKMSKeyArn != "" {
		asappKinesisAccessPolicy.AddStatements(
			awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					jsii.String("kms:Decrypt"),
				},
				Resources: &[]*string{
					jsii.String(kinesisVideoKMSKeyArn),
				},
			}),
		)
	}

	asappGenagentAccessRole.AttachInlinePolicy(asappKinesisAccessPolicy)

	asappInvokePushActionLambdaPolicy := awsiam.NewPolicy(stack, generateObjectName(cfg, "pushaction-lambda-access"), &awsiam.PolicyProps{
		PolicyName: generateObjectName(cfg, "pushaction-lambda-access"),
		Statements: &[]awsiam.PolicyStatement{
			awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
				Effect: awsiam.Effect_ALLOW,
				Actions: &[]*string{
					jsii.String("lambda:InvokeFunction"),
				},
				Resources: &[]*string{
					pushActionLambdaFunction.FunctionArn(),
					pushActionLambdaAlias.FunctionArn(),
				},
			}),
		},
	})
	asappGenagentAccessRole.AttachInlinePolicy(asappInvokePushActionLambdaPolicy)

	// Output the ARN of the Role
	awscdk.NewCfnOutput(stack, jsii.String("iamrolearn"), &awscdk.CfnOutputProps{
		Value: asappGenagentAccessRole.RoleArn(),
	})

	// Output the ARN of the pushaction lambda
	awscdk.NewCfnOutput(stack, jsii.String("pushactionlambdaarn"), &awscdk.CfnOutputProps{
		Value: pushActionLambdaAlias.FunctionArn(),
	})

	return stack
}

func generateObjectName(cfg *config.Config, name string) *string {
	val := fmt.Sprintf("%s%s", cfg.ObjectPrefix, name)
	return &val
}
