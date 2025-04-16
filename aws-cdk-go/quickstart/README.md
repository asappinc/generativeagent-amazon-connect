# ASAPP GenerativeAgent integration with Amazon Connect - AWS CDK quickstart sample

This guide provides an implementation of the ASAPP GenerativeAgent integration guide for Amazon Connect. 

> ## Prerequisites

   MacOS with Homebrew installed.

   Before you begin, make sure you have the following installed:

   1. ### **Golang (v1.23)**  

      Install Go v1.23:
      ```bash
      brew install go@1.23
      ```

      Check if Golang was installed correctly by running:
      ```bash
      go version
      ```

   2. ### **Node.js (v22)**  

      To install Node.js v22, run:
      ```bash
      brew install node@22
      ```

      Check if Node.js was installed correctly by running:
      ```bash
      node -v
      ```

   3. ### **AWS CLI**

      To install the AWS CLI, run:

      ```bash
      brew install awscli
      ```

      Check if AWS CLI was installed correctly by running:

      ```bash
      aws --version
      ```

   4. ### **AWS CDK (v2)**

      To install the AWS CDK, run:

      ```bash
      npm install -g aws-cdk@2
      ```

      Check if AWS CDK was installed correctly by running:

      ```bash
      cdk --version
      ```

   5. ### **Install esbuild**

      To build Lambda functions in JavaScript using the `awslambdanodejs` module, esbuild is required:

      ```bash
      npm install -g esbuild
      ```

      Check if esbuild was installed correctly by running:

      ```bash
      esbuild --version
      ```

<br />

> ## Setup the environment

   1. ### Configure your AWS credentials

      ```bash
      aws configure
      ```

   2. ### Complete the configuration file

      Before deploying the infrastructure, you need to provide a valid configuration file by editing  `config.sample.json` file located at the root level of the project. This file provides necessary environment details, and must follow the structure outlined below:

      ```json
      {
         "accountId": "",
         "region": "",
         "connectInstanceArn": "",
         "objectPrefix": "generativeagent-quickstart-",
         "asapp": {
            "apiHost": "",
            "apiId": "",
            "apiSecret" : "",
            "assumingRoleArn": ""
         },
         "attributesToInputVariablesMap":   {},
         "outputVariablesToAttributesMap": {},
         "ssmlConversions": []
      }
      ```

      | Property                         | Description                                                                                                                                                                                |
      | -------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
      | `accountId`                      | Your AWS account ID.                                                                                                                                                                       |
      | `region`                         | The AWS region where your Amazon Connect instance is hosted.                                                                                                                               |
      | `connectInstanceArn`             | The Amazon Resource Name (ARN) of your Amazon Connect instance that this setup is interacting with.                                                                                        |
      | `objectPrefix`                   | Prefix for AWS objects created by CDK stack, default value - `generativeagent-quickstart-`                                                                                                 |
      | `useExistingVpcId`               | Existing VPC Id to use instead of creating a new one. Default is "", which means new VPC will be created. If specified, it must exist and have at least 2 private subnets (no IGW, no NAT) |
      | `attributesToInputVariablesMap`  | Map of Amazon Connect attributes (User Defined) to GenerativeAgent input variables                                                                                                         |
      | `outputVariablesToAttributesMap` | Map of GenerativeAgent output variables to Amazon Connect attributes (User Defined)                                                                                                        |
      | `ssmlConversions`                | List of conversions for SSML replacements (see details below)                                                                                                                              |
      | `asapp.apiHost`                  | Provided by ASAPP. The API host endpoint, which the system interacts with.                                                                                                                 |
      | `asapp.apiId`                    | Provided by ASAPP. The API ID for authentication and access to the API.                                                                                                                    |
      | `asapp.apiSecret`                | Provided by   ASAPP. The API secret or authentication and access to the API.                                                                                                               |
      | `asapp.assumingRoleArn`          | Provided by ASAPP. The ARN of the IAM role that your system will assume to interact with ASAPP services.                                                                                   |


      #### SSML conversions
      Sometimes pronounciation of certain words needs to be customized which can be done using SSML (if the Amazon Polly voice used supports it). In these cases a list of ssmlConversions that specifies the `searchFor` and `replaceWith` values will make CDK provision the PullAction lambda with those parameters, so when `speak` action is returned by GenerativeAgent, the text returned by GenerativeAgent will be scanned for value of `searchFor` and replaced with the value of `replaceWith` for each element in the ssmlConversions parameter. If ssmlConversions is not an empty list, the overall text will also be enclosed into `<speak>`/`</speak>` tags and the flow module block that speaks the text will be set to interpret text as SSML.

      Sample ssmlConversions value:
      ```
      [
        {
            "searchFor": "ASAPP",
            "replaceWith": "<phoneme alphabet=\"ipa\" ph=\"eɪˈsæp\">ASAPP</phoneme>"
        }
      ]   
      ```
      Note escaping of the quotes, since quotes are used in JSON as terminators. Not all voices support all SSML tags, check https://docs.aws.amazon.com/polly/latest/dg/supportedtags.html for details. 
      SSML tags for English US are described at https://docs.aws.amazon.com/polly/latest/dg/ph-table-english-us.html


   3. ### Boostrap your CDK environment

      Bootstrapping is the process of preparing your AWS environment for usage with the AWS Cloud Development Kit (AWS CDK).

         ```bash
         cdk bootstrap aws://<account-id>/<region>
         ```

      > This step is only required the first time you deploy CDK in a new AWS environment.

<br />

> ## Deploy the CDK stack

   The `cdk deploy` command builds and deploys your AWS CloudFormation stack based on the provided CDK code. 
   By default, running cdk deploy will deploy using `config.sample.json` you created above:
   
   ```shell
   cdk deploy
   ```
   
   Multiple environments (contexts) are supported with default context set in `cdk.json` to `sample`. The configuration file name pattern is ```config.<envName>.json```. If you wish to use multiple configuration file, just create a different configuration file named ```config.<envName>.json``` and specify ```--context <envName>``` as parameter for `cdk deploy`.

   ```shell
   cdk deploy --context envName=<envName>
   ```
   
> <b>Important:</b> Once deployment is complete, CDK will output some values to the terminal. Copy those values and provide them to ASAPP in order to get the proper permissions granted for your infrastructure to connect to ASAPP services.
> Sometimes AWS API times out and CDK deployment fails. If that happens, the remaining artifacts can be cleaned up under CloudFormation service and CDK deploy can be run again.

<br />

> ## Destroy the CDK stack

   The `cdk destroy` command removes all the resources provisioned by the CDK code.

   ```shell
   cdk destroy --context envName=<envName>
   ```
   
   > <b>Important:</b> Make sure to destroy the stack when you're finished with the infrastructure to prevent unnecessary costs.
