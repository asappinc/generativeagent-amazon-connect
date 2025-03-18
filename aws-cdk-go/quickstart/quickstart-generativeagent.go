package main

import (
	"context"
	"encoding/json"
	"fmt"
	quickStart "generativeagent-quickstart/pkg"
	"log"
	"os"
	"strings"

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
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/connect"
	"github.com/aws/aws-sdk-go-v2/service/connect/types"
	"github.com/aws/jsii-runtime-go"

	"github.com/iancoleman/orderedmap"

	"github.com/heetch/confita"
	"github.com/heetch/confita/backend/file"

	"github.com/aws/constructs-go/constructs/v10"
)

type AmazonConnectDemoCdkStackProps struct {
	awscdk.StackProps
	EnvName *string
}

func NewQuickStartGenerativeAgentStack(scope constructs.Construct, id string, props *AmazonConnectDemoCdkStackProps, cfg *quickStart.Config) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	awslogs.NewLogGroup(stack, jsii.String("generativeagent-quickstart-stack-log-group"), &awslogs.LogGroupProps{
		LogGroupName:  jsii.String("generativeagent-quickstart-stack-log-group"),
		Retention:     awslogs.RetentionDays_THREE_DAYS,
		RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
	})

	awsCfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(cfg.Region))
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	// Check if there is any Kinesis Video Stream configuration
	svc := connect.NewFromConfig(awsCfg)
	instanceStorageConfigsInput := &connect.ListInstanceStorageConfigsInput{
		InstanceId:   jsii.String(cfg.ConnectInstanceArn),
		ResourceType: types.InstanceStorageResourceTypeMediaStreams,
	}
	result, err := svc.ListInstanceStorageConfigs(context.TODO(), instanceStorageConfigsInput)
	if err != nil {
		fmt.Println("Error:", err)
	}

	kinesisPrefixExists := false
	kinesisVideoStreamConfigPrefix := ""
	if len(result.StorageConfigs) > 0 {
		for _, storageConfig := range result.StorageConfigs {
			if storageConfig.StorageType == types.StorageTypeKinesisVideoStream {
				fmt.Printf("Kinesis Video Stream Config found: %v\n", *storageConfig.KinesisVideoStreamConfig.Prefix)
				kinesisVideoStreamConfigPrefix = *storageConfig.KinesisVideoStreamConfig.Prefix
				kinesisPrefixExists = true
			}
		}
	}

	if !kinesisPrefixExists { // If the Kinesis prefix doesn't exist, create it using a AWS Custom Resource call
		kinesisVideoStreamConfigPrefix = "generativeagent-quickstart"
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
	s3Bucket := awss3.NewBucket(stack, jsii.String("generativeagent-quickstart-bucket"), &awss3.BucketProps{
		Versioned:         jsii.Bool(false),
		RemovalPolicy:     awscdk.RemovalPolicy_DESTROY,
		AutoDeleteObjects: jsii.Bool(true),
	})

	// Grant the custom Role read access to the S3 Bucket
	customInstanceRole := awsiam.NewRole(stack, jsii.String("generativeagent-quickstart-custom-instance-role"), &awsiam.RoleProps{
		AssumedBy: awsiam.NewServicePrincipal(jsii.String("connect.amazonaws.com"), nil),
	})
	s3Bucket.GrantRead(customInstanceRole, "*")

	// Upload the audio files to the S3 Bucket
	s3BucketDeployment := awss3deployment.NewBucketDeployment(stack, jsii.String("generativeagent-quickstart-bucket-deployment"), &awss3deployment.BucketDeploymentProps{
		Sources: &[]awss3deployment.ISource{
			awss3deployment.Source_Asset(jsii.String("../../flow-modules/prompts"), nil),
		},
		DestinationBucket: s3Bucket,
	})

	// Create an S3 URL for the uploaded audio file
	beepbopUrl := s3Bucket.S3UrlForObject(jsii.String("asappBeepBop.wav"))
	silence1secondUrl := s3Bucket.S3UrlForObject(jsii.String("asappSilence1second.wav"))
	silence400msUrl := s3Bucket.S3UrlForObject(jsii.String("asappSilence400ms.wav"))

	// Create the Prompts
	createBeepbopShortPrompt := customresources.NewAwsCustomResource(stack, jsii.String("generativeagent-quickstart-create-prompt-asappBeepBop"),
		&customresources.AwsCustomResourceProps{
			OnCreate: &customresources.AwsSdkCall{
				Service: jsii.String("Connect"),
				Action:  jsii.String("CreatePrompt"),
				Parameters: map[string]interface{}{
					"InstanceId":  jsii.String(cfg.ConnectInstanceArn),
					"Name":        jsii.String("asappBeepBop"),
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

	createSilence1secondPrompt := customresources.NewAwsCustomResource(stack, jsii.String("generativeagent-quickstart-create-prompt-asappSilence1second"),
		&customresources.AwsCustomResourceProps{
			OnCreate: &customresources.AwsSdkCall{
				Service: jsii.String("Connect"),
				Action:  jsii.String("CreatePrompt"),
				Parameters: map[string]interface{}{
					"InstanceId":  jsii.String(cfg.ConnectInstanceArn),
					"Name":        jsii.String("asappSilence1second"),
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

	createSilence400msPrompt := customresources.NewAwsCustomResource(stack, jsii.String("generativeagent-quickstart-create-prompt-asappSilence400ms"),
		&customresources.AwsCustomResourceProps{
			OnCreate: &customresources.AwsSdkCall{
				Service: jsii.String("Connect"),
				Action:  jsii.String("CreatePrompt"),
				Parameters: map[string]interface{}{
					"InstanceId":  jsii.String(cfg.ConnectInstanceArn),
					"Name":        jsii.String("asappSilence400ms"),
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

	// -- Create the VPC --
	vpc := awsec2.NewVpc(stack, jsii.String("generativeagent-quickstart-vpc"), &awsec2.VpcProps{
		MaxAzs: aws.Float64(2), // Two availability zones.
		SubnetConfiguration: &[]*awsec2.SubnetConfiguration{
			{
				Name:       jsii.String("PrivateIsolatedSubnet"),
				SubnetType: awsec2.SubnetType_PRIVATE_ISOLATED,
			},
		},
	})
	vpcPrivateIsolatedSubnets := *vpc.IsolatedSubnets()

	// -- Create the Security Groups --
	redisSecurityGroup := awsec2.NewSecurityGroup(stack, jsii.String("generativeagent-quickstart-redis-security-group"), &awsec2.SecurityGroupProps{
		Vpc:               vpc,
		SecurityGroupName: jsii.String("generativeagent-quickstart-redis-security-group"),
		AllowAllOutbound:  jsii.Bool(false),
	})
	pullActionLambdaSecurityGroup := awsec2.NewSecurityGroup(stack, jsii.String("generativeagent-quickstart-lambda-pullaction-security-group"), &awsec2.SecurityGroupProps{
		Vpc:               vpc,
		SecurityGroupName: jsii.String("generativeagent-quickstart-lambda-pullaction-security-group"),
		AllowAllOutbound:  jsii.Bool(false),
	})
	pushActionLambdaSecurityGroup := awsec2.NewSecurityGroup(stack, jsii.String("generativeagent-quickstart-lambda-pushaction-security-group"), &awsec2.SecurityGroupProps{
		Vpc:               vpc,
		SecurityGroupName: jsii.String("generativeagent-quickstart-lambda-pushaction-security-group"),
		AllowAllOutbound:  jsii.Bool(false),
	})

	redisSecurityGroup.AddIngressRule(
		pullActionLambdaSecurityGroup,
		awsec2.Port_Tcp(aws.Float64(6379)),
		jsii.String("Allow inbound TCP traffic only from generativeagent-quickstart-lambda-pullaction-security-group into the Reddis port"),
		jsii.Bool(false),
	)

	redisSecurityGroup.AddIngressRule(
		pushActionLambdaSecurityGroup,
		awsec2.Port_Tcp(aws.Float64(6379)),
		jsii.String("Allow inbound TCP traffic only from generativeagent-quickstart-lambda-pushaction-security-group into the Reddis port"),
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

	// -- Create the Redis cluster --
	redisSubnetGroup := awselasticache.NewCfnSubnetGroup(stack, jsii.String("generativeagent-quickstart-redis-subnet-group"), &awselasticache.CfnSubnetGroupProps{
		Description: jsii.String("Subnet group for ASAPP Redis"),
		SubnetIds: &[]*string{
			vpcPrivateIsolatedSubnets[0].SubnetId(),
			vpcPrivateIsolatedSubnets[1].SubnetId(),
		},
	})

	redisCluster := awselasticache.NewCfnCacheCluster(stack, jsii.String("generativeagent-quickstart-redis-cluster"), &awselasticache.CfnCacheClusterProps{
		ClusterName:          jsii.String("generativeagent-quickstart-redis-cluster"),
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

	/// -- Create the Lambda functions and associate them to the Connect Instance --
	// Engage: this function only talks to Internet endpoints and is not attached to a VPC.
	engageLambdaFunction := awslambdanodejs.NewNodejsFunction(stack, jsii.String("generativeagent-quickstart-lambda-genagent-engage"), &awslambdanodejs.NodejsFunctionProps{
		FunctionName:     jsii.String("generativeagent-quickstart-lambda-genagent-engage"),
		Entry:            jsii.String("../../lambdas/engage/index.mjs"),
		Handler:          jsii.String("handler"),
		Runtime:          awslambda.Runtime_NODEJS_20_X(),
		DepsLockFilePath: jsii.String("../../lambdas/engage/package-lock.json"),
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
	pullActionLambdaFunction := awslambdanodejs.NewNodejsFunction(stack, jsii.String("generativeagent-quickstart-lambda-pullaction"), &awslambdanodejs.NodejsFunctionProps{
		FunctionName:     jsii.String("generativeagent-quickstart-lambda-pullaction"),
		Entry:            jsii.String("../../lambdas/pullaction/index.mjs"),
		Handler:          jsii.String("handler"),
		Runtime:          awslambda.Runtime_NODEJS_20_X(),
		DepsLockFilePath: jsii.String("../../lambdas/pullaction/package-lock.json"),
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
	pushActionLambdaFunction := awslambdanodejs.NewNodejsFunction(stack, jsii.String("generativeagent-quickstart-lambda-pushaction"), &awslambdanodejs.NodejsFunctionProps{
		FunctionName:     jsii.String("generativeagent-quickstart-lambda-pushaction"),
		Entry:            jsii.String("../../lambdas/pushaction/index.mjs"),
		Handler:          jsii.String("handler"),
		Runtime:          awslambda.Runtime_NODEJS_20_X(),
		DepsLockFilePath: jsii.String("../../lambdas/pushaction/package-lock.json"),
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
	contactFlowModuleContent, err := os.ReadFile("../../flow-modules/template/ASAPPGenerativeAgent.json")
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

	contactFlowModuleContent, err = json.MarshalIndent(contactFlowModuleContentMap, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal updated JSON: %v\n", err)
	}

	// Create the Contact Flow Module
	connectModule := awsconnect.NewCfnContactFlowModule(stack, jsii.String("generativeagent-quickstart-contact-flow-module"), &awsconnect.CfnContactFlowModuleProps{
		InstanceArn: jsii.String(cfg.ConnectInstanceArn),
		Name:        jsii.String("generativeagent-quickstart-contact-flow-module"),
		Content:     jsii.String(string(contactFlowModuleContent)),
	})

	// Wait for the Prompts to be ready before proceeding to create the Contact Flow Module
	connectModule.Node().AddDependency(createBeepbopShortPrompt)
	connectModule.Node().AddDependency(createSilence1secondPrompt)
	connectModule.Node().AddDependency(createSilence400msPrompt)

	// -- Create the Role: generativeagent-quickstart-access-role --
	asappGenagentAccessRole := awsiam.NewRole(stack, jsii.String("generativeagent-quickstart-access-role"), &awsiam.RoleProps{
		RoleName: jsii.String("generativeagent-quickstart-access-role"),
		AssumedBy: awsiam.NewCompositePrincipal(
			awsiam.NewArnPrincipal(jsii.String(cfg.Asapp.AssumingRoleArn)), // TrustASAPPRole
		)})

	var sbAsappKinesisAccessPolicy strings.Builder
	sbAsappKinesisAccessPolicy.WriteString("arn:aws:kinesisvideo:*:")
	sbAsappKinesisAccessPolicy.WriteString(cfg.AccountId)
	sbAsappKinesisAccessPolicy.WriteString(":stream/")
	sbAsappKinesisAccessPolicy.WriteString(kinesisVideoStreamConfigPrefix)
	sbAsappKinesisAccessPolicy.WriteString("*/*")
	asappKinesisAccessPolicy := awsiam.NewPolicy(stack, jsii.String("generativeagent-quickstart-kinesis-access"), &awsiam.PolicyProps{
		PolicyName: jsii.String("generativeagent-quickstart-kinesis-access"),
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
	asappGenagentAccessRole.AttachInlinePolicy(asappKinesisAccessPolicy)

	asappInvokePushActionLambdaPolicy := awsiam.NewPolicy(stack, jsii.String("generativeagent-quickstart-pushaction-lambda-access"), &awsiam.PolicyProps{
		PolicyName: jsii.String("generativeagent-quickstart-pushaction-lambda-access"),
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

func main() {
	app := awscdk.NewApp(nil)
	envName := app.Node().TryGetContext(jsii.String("envName")).(string)
	cfg, err := loadConfiguration(envName)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}
	NewQuickStartGenerativeAgentStack(app, "generativeagent-quickstart-stack", &AmazonConnectDemoCdkStackProps{
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

func loadConfiguration(envName string) (*quickStart.Config, error) {
	// Load configuration
	cfgFilename := fmt.Sprintf("config.%s.json", envName)
	loader := confita.NewLoader(file.NewBackend(cfgFilename))
	cfg := quickStart.Config{}
	err := loader.Load(context.Background(), &cfg)
	return &cfg, err
}
