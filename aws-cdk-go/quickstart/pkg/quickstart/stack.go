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
	engageLambdaDir                           = "../../lambdas/engage"
	engageLambdaIndexPath                     = engageLambdaDir + "/index.mjs"
	engageLambdaAttributeToInputVariablesPath = engageLambdaDir + "/attributesToInputVariables.mjs"
	engageLambdaLockPath                      = engageLambdaDir + "/package-lock.json"
	pullActionLambdaDir                       = "../../lambdas/pullaction"
	pullActionLambdaIndexPath                 = pullActionLambdaDir + "/index.mjs"
	pullActionLambdaLockPath                  = pullActionLambdaDir + "/package-lock.json"
	pushActionLambdaDir                       = "../../lambdas/pushaction"
	pushActionLambdaIndexPath                 = pushActionLambdaDir + "/index.mjs"
	pushActionLambdaLockPath                  = pushActionLambdaDir + "/package-lock.json"

	// Template paths
	contactFlowModulePath = "../../flow-modules/template/ASAPPGenerativeAgent.json"
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
		customresources.NewAwsCustomResource(stack, jsii.String("EnableKinesisPrefix"), &customresources.AwsCustomResourceProps{
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
			Policy: customresources.AwsCustomResourcePolicy_FromStatements(&[]awsiam.PolicyStatement{
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
			}),
		})
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
					"Name":        jsii.String("asappBeepBop2"),
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
			Policy: customresources.AwsCustomResourcePolicy_FromStatements(&[]awsiam.PolicyStatement{
				awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
					Actions: jsii.Strings(
						"connect:CreatePrompt",
						"connect:DeletePrompt",
						"s3:GetObject",
						"s3:ListBucket",
					),
					Resources: jsii.Strings("*"),
				}),
			}),
		})

	createSilence1secondPrompt := customresources.NewAwsCustomResource(stack, generateObjectName(cfg, "create-prompt-asappSilence1second"),
		&customresources.AwsCustomResourceProps{
			OnCreate: &customresources.AwsSdkCall{
				Service: jsii.String("Connect"),
				Action:  jsii.String("CreatePrompt"),
				Parameters: map[string]interface{}{
					"InstanceId":  jsii.String(cfg.ConnectInstanceArn),
					"Name":        jsii.String("asappSilence1second2"),
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
			Policy: customresources.AwsCustomResourcePolicy_FromStatements(&[]awsiam.PolicyStatement{
				awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
					Actions: jsii.Strings(
						"connect:CreatePrompt",
						"connect:DeletePrompt",
						"s3:GetObject",
						"s3:ListBucket",
					),
					Resources: jsii.Strings("*"),
				}),
			}),
		})

	createSilence400msPrompt := customresources.NewAwsCustomResource(stack, generateObjectName(cfg, "create-prompt-asappSilence400ms"),
		&customresources.AwsCustomResourceProps{
			OnCreate: &customresources.AwsSdkCall{
				Service: jsii.String("Connect"),
				Action:  jsii.String("CreatePrompt"),
				Parameters: map[string]interface{}{
					"InstanceId":  jsii.String(cfg.ConnectInstanceArn),
					"Name":        jsii.String("asappSilence400ms2"),
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
			Policy: customresources.AwsCustomResourcePolicy_FromStatements(&[]awsiam.PolicyStatement{
				awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
					Actions: jsii.Strings(
						"connect:CreatePrompt",
						"connect:DeletePrompt",
						"s3:GetObject",
						"s3:ListBucket",
					),
					Resources: jsii.Strings("*"),
				}),
			}),
		})

	// Wait for the audio files to be uploaded before creating the Prompts
	createBeepbopShortPrompt.Node().AddDependency(s3BucketDeployment)
	createSilence1secondPrompt.Node().AddDependency(s3BucketDeployment)
	createSilence400msPrompt.Node().AddDependency(s3BucketDeployment)

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
	redisSecurityGroup := awsec2.NewSecurityGroup(stack, generateObjectName(cfg, "redis-security-group"), &awsec2.SecurityGroupProps{
		Vpc:               vpc,
		SecurityGroupName: generateObjectName(cfg, "redis-security-group"),
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

	redisSecurityGroup.AddIngressRule(
		pullActionLambdaSecurityGroup,
		awsec2.Port_Tcp(aws.Float64(6379)),
		jsii.String(fmt.Sprintf("Allow inbound TCP traffic only from %slambda-pullaction-security-group into the Reddis port", cfg.ObjectPrefix)),
		jsii.Bool(false),
	)

	redisSecurityGroup.AddIngressRule(
		pushActionLambdaSecurityGroup,
		awsec2.Port_Tcp(aws.Float64(6379)),
		jsii.String(fmt.Sprintf("Allow inbound TCP traffic only from %slambda-pushaction-security-group into the Reddis port", cfg.ObjectPrefix)),
		jsii.Bool(false),
	)

	pullActionLambdaSecurityGroup.AddEgressRule(
		redisSecurityGroup,
		awsec2.Port_Tcp(aws.Float64(6379)),
		jsii.String("Allow outbound TCP traffic only to Redis security group and port"),
		jsii.Bool(false),
	)

	pushActionLambdaSecurityGroup.AddEgressRule(
		redisSecurityGroup,
		awsec2.Port_Tcp(aws.Float64(6379)),
		jsii.String("Allow outbound TCP traffic only to Redis security group and port"),
		jsii.Bool(false),
	)

	var redisSubnetGroupSubnetIds []*string
	for _, subnet := range vpcPrivateIsolatedSubnets {
		redisSubnetGroupSubnetIds = append(redisSubnetGroupSubnetIds, subnet.SubnetId())
	}

	// -- Create the Redis cluster --
	redisSubnetGroup := awselasticache.NewCfnSubnetGroup(stack, generateObjectName(cfg, "redis-subnet-group"), &awselasticache.CfnSubnetGroupProps{
		Description: jsii.String("Subnet group for ASAPP Redis"),
		SubnetIds:   &redisSubnetGroupSubnetIds,
	})

	redisCluster := awselasticache.NewCfnCacheCluster(stack, generateObjectName(cfg, "redis-cluster"), &awselasticache.CfnCacheClusterProps{
		ClusterName:          generateObjectName(cfg, "redis-cluster"),
		CacheNodeType:        jsii.String("cache.t4g.micro"),
		Engine:               jsii.String("redis"),
		NumCacheNodes:        aws.Float64(1),
		CacheSubnetGroupName: redisSubnetGroup.Ref(),
		VpcSecurityGroupIds:  &[]*string{redisSecurityGroup.SecurityGroupId()},
	})

	redisUrl := awscdk.Fn_Join(jsii.String(""), &[]*string{
		jsii.String("redis://"),
		redisCluster.AttrRedisEndpointAddress(),
		jsii.String(":"),
		redisCluster.AttrRedisEndpointPort(),
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
		Policy: customresources.AwsCustomResourcePolicy_FromStatements(&[]awsiam.PolicyStatement{
			awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
				Actions: jsii.Strings(
					"connect:AssociateLambdaFunction",
					"connect:DisassociateLambdaFunction",
					"lambda:AddPermission",
				),
				Resources: jsii.Strings("*"),
			}),
		}),
	})

	associateEngageLambdaWithConnect.Node().AddDependency(engageLambdaFunction)

	// PullAction: this function needs access to the Redis Cluster, so it needs to be on the same VPC.
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
				jsii.String("redis"),
			},
		},
		Vpc: vpc,
		VpcSubnets: &awsec2.SubnetSelection{
			Subnets: &vpcPrivateIsolatedSubnets,
		},
		SecurityGroups: &[]awsec2.ISecurityGroup{pullActionLambdaSecurityGroup},
		Environment: &map[string]*string{
			"REDIS_URL": redisUrl,
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
		Policy: customresources.AwsCustomResourcePolicy_FromStatements(&[]awsiam.PolicyStatement{
			awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
				Actions: jsii.Strings(
					"connect:AssociateLambdaFunction",
					"connect:DisassociateLambdaFunction",
					"lambda:AddPermission",
				),
				Resources: jsii.Strings("*"),
			}),
		}),
	})

	associatePullActionLambdaWithConnect.Node().AddDependency(pullActionLambdaFunction)

	// PushAction: this function needs access to the Redis Cluster, so it needs to be on the same VPC.
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
				jsii.String("redis"),
			},
		},
		Vpc: vpc,
		VpcSubnets: &awsec2.SubnetSelection{
			Subnets: &vpcPrivateIsolatedSubnets,
		},
		SecurityGroups: &[]awsec2.ISecurityGroup{pushActionLambdaSecurityGroup},
		Environment: &map[string]*string{
			"REDIS_URL": redisUrl,
		},
	})

	associatePushActionLambdaWithConnect := customresources.NewAwsCustomResource(stack, jsii.String("AssociatePushActionLambdaWithConnect"), &customresources.AwsCustomResourceProps{
		OnCreate: &customresources.AwsSdkCall{
			Service: jsii.String("Connect"),
			Action:  jsii.String("AssociateLambdaFunction"),
			Parameters: map[string]interface{}{
				"InstanceId":  jsii.String(cfg.ConnectInstanceArn),
				"FunctionArn": pushActionLambdaFunction.FunctionArn(),
			},
			PhysicalResourceId: customresources.PhysicalResourceId_Of(jsii.String("AssociatePushActionLambda")),
		},
		OnDelete: &customresources.AwsSdkCall{
			Service: jsii.String("Connect"),
			Action:  jsii.String("DisassociateLambdaFunction"),
			Parameters: map[string]interface{}{
				"InstanceId":  jsii.String(cfg.ConnectInstanceArn),
				"FunctionArn": pushActionLambdaFunction.FunctionArn(),
			},
		},
		Policy: customresources.AwsCustomResourcePolicy_FromStatements(&[]awsiam.PolicyStatement{
			awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
				Actions: jsii.Strings(
					"connect:AssociateLambdaFunction",
					"connect:DisassociateLambdaFunction",
					"lambda:AddPermission",
				),
				Resources: jsii.Strings("*"),
			}),
		}),
	})

	associatePushActionLambdaWithConnect.Node().AddDependency(pushActionLambdaFunction)

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
		"Engage":     *engageLambdaFunction.FunctionArn(),
		"PullAction": *pullActionLambdaFunction.FunctionArn(),
	}

	// Update the referenced resources (Prompts and Lambda functions), then Marshal the content into the same variable.
	UpdateResourcesARN(&contactFlowModuleContentMap, cfg.Region, cfg.AccountId, cfg.ConnectInstanceArn, promptArnsMap, lambdaFunctionsArnMap)

	// Update Output Variables
	UpdateExtractOutputVariables(&contactFlowModuleContentMap, cfg.OutputVariablesToAttributesMap)

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
	connectModule.Node().AddDependency(createBeepbopShortPrompt)
	connectModule.Node().AddDependency(createSilence1secondPrompt)
	connectModule.Node().AddDependency(createSilence400msPrompt)

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
		Value: pushActionLambdaFunction.FunctionArn(),
	})

	return stack
}

func generateObjectName(cfg *config.Config, name string) *string {
	val := fmt.Sprintf("%s%s", cfg.ObjectPrefix, name)
	return &val
}
